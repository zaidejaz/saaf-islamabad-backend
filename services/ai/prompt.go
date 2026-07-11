package ai

import (
	"fmt"
	"strings"
)

// Default free-tier Groq vision model (swap via AI_MODEL).
// Alternatives on Groq free tier that support images:
//   - meta-llama/llama-4-scout-17b-16e-instruct  (default)
//   - qwen/qwen3.6-27b
const DefaultGroqVisionModel = "meta-llama/llama-4-scout-17b-16e-instruct"
const DefaultGroqBaseURL = "https://api.groq.com/openai/v1"

func buildSystemPrompt(categories []CategoryOption) string {
	var b strings.Builder
	b.WriteString(`You are the Saaf Islamabad civic-issue image classifier.
You validate whether a photo shows a real public civic issue in Islamabad, classify it, and draft report fields.

Rules:
- Only classify into the provided categories (use exact category_id and category_name).
- Reject images that are NOT public civic issues: selfies, food, pets (unless clearly stray animals in public), documents, screenshots, indoor photos, memes, blank/dark/blurry images, unrelated objects.
- Do NOT assign departments, save reports, or send notifications.
- title: max 150 characters, concise, location-aware when address/coords are given.
- description: 1–3 sentences describing the visible issue.
- severity_level: one of low, moderate, critical.
- priority_level: one of low, medium, high.
- ai_confidence_score: number from 0.0 to 1.0.
- Respond with a single JSON object only (no markdown).

If invalid, return exactly:
{"is_valid_issue":false,"error":"not_a_civic_issue","message":"Please upload a clear photo of a public civic issue."}

If valid, return exactly:
{"is_valid_issue":true,"category_id":"<uuid>","category_name":"<exact name>","title":"...","description":"...","severity_level":"low|moderate|critical","priority_level":"low|medium|high","ai_confidence_score":0.0}

Allowed categories:
`)
	for _, c := range categories {
		fmt.Fprintf(&b, "- %s | %s\n", c.ID.String(), c.Name)
	}
	return b.String()
}

func buildUserPrompt(req ClassifyRequest) string {
	var b strings.Builder
	b.WriteString("Classify this civic issue photo and return JSON only.")
	if req.Address != "" {
		fmt.Fprintf(&b, "\nAddress: %s", req.Address)
	}
	if req.Latitude != nil && req.Longitude != nil {
		fmt.Fprintf(&b, "\nCoordinates: %.6f, %.6f", *req.Latitude, *req.Longitude)
	}
	return b.String()
}
