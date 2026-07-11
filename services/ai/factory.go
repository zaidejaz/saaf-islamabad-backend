package ai

import (
	"fmt"
	"strings"

	"github.com/zaidejaz/saaf-islamabad-backend/config"
)

// NewClassifier builds a Classifier from config. Provider and model are
// swappable via AI_PROVIDER / AI_MODEL without code changes.
//
// Supported providers:
//   - groq  (default) — free-tier vision via Groq inference
//   - openai_compatible — any OpenAI-compatible chat/completions vision API
//     (set AI_BASE_URL + AI_API_KEY / GROQ_API_KEY)
func NewClassifier(cfg *config.Config) (Classifier, error) {
	if cfg == nil {
		return nil, ErrNotConfigured
	}
	if !cfg.AIEnabled {
		return nil, ErrDisabled
	}
	if strings.TrimSpace(cfg.AIAPIKey) == "" {
		return nil, fmt.Errorf("%w: set GROQ_API_KEY (or AI_API_KEY)", ErrNotConfigured)
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.AIProvider))
	if provider == "" {
		provider = "groq"
	}

	model := strings.TrimSpace(cfg.AIModel)
	if model == "" {
		model = DefaultGroqVisionModel
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.AIBaseURL), "/")
	if baseURL == "" {
		baseURL = DefaultGroqBaseURL
	}

	timeoutSecs := cfg.AITimeoutSecs
	if timeoutSecs <= 0 {
		timeoutSecs = 8
	}

	switch provider {
	case "groq", "openai_compatible", "openai":
		return NewOpenAICompatibleClassifier(OpenAICompatibleConfig{
			Name:       provider,
			APIKey:     cfg.AIAPIKey,
			BaseURL:    baseURL,
			Model:      model,
			TimeoutSec: timeoutSecs,
		}), nil
	default:
		return nil, fmt.Errorf("%w: unknown AI_PROVIDER %q (supported: groq, openai_compatible)", ErrNotConfigured, provider)
	}
}
