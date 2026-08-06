package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

type OpenAICompatibleConfig struct {
	Name       string
	APIKey     string
	BaseURL    string
	Model      string
	TimeoutSec int
}

// OpenAICompatibleClassifier talks to Groq (or any OpenAI-compatible)
// chat/completions vision endpoint. Swap models with AI_MODEL.
type OpenAICompatibleClassifier struct {
	name   string
	apiKey string
	base   string
	model  string
	client *http.Client
}

func NewOpenAICompatibleClassifier(cfg OpenAICompatibleConfig) *OpenAICompatibleClassifier {
	timeout := time.Duration(cfg.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 8 * time.Second
	}
	name := cfg.Name
	if name == "" {
		name = "openai_compatible"
	}
	return &OpenAICompatibleClassifier{
		name:   name,
		apiKey: cfg.APIKey,
		base:   strings.TrimRight(cfg.BaseURL, "/"),
		model:  cfg.Model,
		client: &http.Client{Timeout: timeout},
	}
}

func (c *OpenAICompatibleClassifier) Name() string  { return c.name }
func (c *OpenAICompatibleClassifier) Model() string { return c.model }

func (c *OpenAICompatibleClassifier) Classify(ctx context.Context, req ClassifyRequest) (*ClassifyResult, error) {
	if len(req.ImageURLs) == 0 {
		return nil, ErrNoImages
	}
	if len(req.Categories) == 0 {
		return nil, fmt.Errorf("%w: no categories available", ErrProvider)
	}

	imageParts, err := resolveImageParts(req.ImageURLs)
	if err != nil {
		return nil, err
	}

	content := []map[string]any{
		{"type": "text", "text": buildUserPrompt(req)},
	}
	for _, part := range imageParts {
		content = append(content, map[string]any{
			"type": "image_url",
			"image_url": map[string]string{
				"url": part,
			},
		})
	}

	payload := map[string]any{
		"model": c.model,
		"messages": []map[string]any{
			{
				"role":    "system",
				"content": buildSystemPrompt(req.Categories),
			},
			{
				"role":    "user",
				"content": content,
			},
		},
		"temperature":     0.2,
		"max_tokens":      1024,
		"response_format": map[string]string{"type": "json_object"},
	}
	// Qwen 3.6 defaults to thinking mode. With JSON mode that often yields
	// empty content and Groq's json_validate_failed (failed_generation="").
	// Disable reasoning so the model emits JSON in message.content.
	if isQwen36(c.model) {
		payload["reasoning_effort"] = "none"
		payload["reasoning_format"] = "hidden"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrProvider, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: read response: %v", ErrProvider, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%w: status %d: %s", ErrProvider, resp.StatusCode, truncate(string(respBody), 400))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParse, err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("%w: empty model content", ErrParse)
	}
	msg := chatResp.Choices[0].Message
	modelText := firstNonEmpty(msg.Content, msg.Reasoning, msg.ReasoningContent)
	if strings.TrimSpace(modelText) == "" {
		return nil, fmt.Errorf("%w: empty model content", ErrParse)
	}

	raw := extractJSONObject(modelText)
	var result ClassifyResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParse, err)
	}

	return normalizeResult(&result, req.Categories), nil
}

func isQwen36(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "qwen3.6") || strings.Contains(m, "qwen3-6")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// resolveImageParts turns backend /uploads paths into base64 data URLs so
// Groq can read them even when the host is not publicly reachable.
func resolveImageParts(imageURLs []string) ([]string, error) {
	parts := make([]string, 0, len(imageURLs))
	for i, raw := range imageURLs {
		if i >= 5 {
			break // Groq vision limit
		}
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}

		if strings.HasPrefix(u, "data:image/") {
			parts = append(parts, u)
			continue
		}

		if localPath, ok := localUploadPath(u); ok {
			dataURL, err := fileToDataURL(localPath)
			if err != nil {
				return nil, fmt.Errorf("%w: cannot read local image %s: %v", ErrProvider, u, err)
			}
			parts = append(parts, dataURL)
			continue
		}

		// Absolute http(s) URL — pass through for public hosts.
		if parsed, err := url.Parse(u); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
			// Localhost / private hosts cannot be fetched by Groq — inline them.
			host := strings.ToLower(parsed.Hostname())
			if host == "localhost" || host == "127.0.0.1" || host == "::1" || strings.HasPrefix(host, "192.168.") || strings.HasPrefix(host, "10.") {
				if localPath, ok := localUploadPath(parsed.Path); ok {
					dataURL, err := fileToDataURL(localPath)
					if err != nil {
						return nil, fmt.Errorf("%w: cannot read local image %s: %v", ErrProvider, u, err)
					}
					parts = append(parts, dataURL)
					continue
				}
			}
			parts = append(parts, u)
			continue
		}

		return nil, fmt.Errorf("%w: unsupported image_url %q", ErrNoImages, u)
	}
	if len(parts) == 0 {
		return nil, ErrNoImages
	}
	return parts, nil
}

func localUploadPath(u string) (string, bool) {
	path := u
	if parsed, err := url.Parse(u); err == nil && parsed.Path != "" && (parsed.Scheme == "" || parsed.Scheme == "http" || parsed.Scheme == "https") {
		path = parsed.Path
	}
	path = strings.TrimPrefix(path, "./")
	path = strings.TrimPrefix(path, "/")
	if !strings.HasPrefix(path, "uploads/") {
		return "", false
	}
	clean := filepath.Clean(path)
	// Must stay under uploads/ after cleaning (blocks .. traversal).
	if clean != path && !strings.HasPrefix(clean, "uploads"+string(filepath.Separator)) {
		return "", false
	}
	if !strings.HasPrefix(clean, "uploads"+string(filepath.Separator)) {
		return "", false
	}
	return clean, true
}

func fileToDataURL(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", fmt.Errorf("empty file")
	}
	// Groq base64 request limit ~4MB.
	if len(data) > 4*1024*1024 {
		return "", fmt.Errorf("image exceeds 4MB base64 limit")
	}
	mime := http.DetectContentType(data)
	if !strings.HasPrefix(mime, "image/") {
		ext := strings.ToLower(filepath.Ext(path))
		switch ext {
		case ".jpg", ".jpeg":
			mime = "image/jpeg"
		case ".png":
			mime = "image/png"
		case ".webp":
			mime = "image/webp"
		case ".gif":
			mime = "image/gif"
		default:
			mime = "image/jpeg"
		}
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func normalizeResult(r *ClassifyResult, categories []CategoryOption) *ClassifyResult {
	if r == nil || !r.IsValidIssue {
		inv := InvalidIssueResult()
		if r != nil {
			if r.Error != "" {
				inv.Error = r.Error
			}
			if r.Message != "" {
				inv.Message = r.Message
			}
		}
		return inv
	}

	catByID := map[uuid.UUID]CategoryOption{}
	catByName := map[string]CategoryOption{}
	for _, c := range categories {
		catByID[c.ID] = c
		catByName[strings.ToLower(strings.TrimSpace(c.Name))] = c
	}

	var matched *CategoryOption
	if r.CategoryID != nil {
		if c, ok := catByID[*r.CategoryID]; ok {
			matched = &c
		}
	}
	if matched == nil && r.CategoryName != "" {
		if c, ok := catByName[strings.ToLower(strings.TrimSpace(r.CategoryName))]; ok {
			matched = &c
		}
	}
	if matched == nil {
		return InvalidIssueResult()
	}

	title := strings.TrimSpace(r.Title)
	if title == "" {
		title = matched.Name
	}
	if utf8.RuneCountInString(title) > 150 {
		runes := []rune(title)
		title = string(runes[:150])
	}

	desc := strings.TrimSpace(r.Description)
	if desc == "" {
		desc = "Civic issue detected requiring municipal attention."
	}

	sev := strings.ToLower(strings.TrimSpace(r.SeverityLevel))
	switch sev {
	case "low", "moderate", "critical":
	default:
		sev = "moderate"
	}

	pri := strings.ToLower(strings.TrimSpace(r.PriorityLevel))
	switch pri {
	case "low", "medium", "high":
	default:
		pri = "medium"
	}

	var score float64
	if r.AIConfidenceScore != nil {
		score = *r.AIConfidenceScore
	} else {
		score = 0.7
	}
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}

	id := matched.ID
	return &ClassifyResult{
		IsValidIssue:      true,
		CategoryID:        &id,
		CategoryName:      matched.Name,
		Title:             title,
		Description:       desc,
		SeverityLevel:     sev,
		PriorityLevel:     pri,
		AIConfidenceScore: &score,
	}
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
