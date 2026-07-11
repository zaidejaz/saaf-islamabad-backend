package ai

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrNotConfigured = errors.New("ai classifier is not configured")
	ErrDisabled      = errors.New("ai classifier is disabled")
	ErrNoImages      = errors.New("at least one image_url is required")
	ErrProvider      = errors.New("ai provider request failed")
	ErrParse         = errors.New("failed to parse ai response")
)

// CategoryOption is a known DB category the model must choose from.
type CategoryOption struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// ClassifyRequest is the input for civic-issue image classification.
type ClassifyRequest struct {
	ImageURLs  []string         `json:"image_urls"`
	Latitude   *float64         `json:"latitude,omitempty"`
	Longitude  *float64         `json:"longitude,omitempty"`
	Address    string           `json:"address,omitempty"`
	Categories []CategoryOption `json:"-"`
}

// ClassifyResult matches the AI Image Classification Integration Spec v1.0.
type ClassifyResult struct {
	IsValidIssue      bool       `json:"is_valid_issue"`
	Error             string     `json:"error,omitempty"`
	Message           string     `json:"message,omitempty"`
	CategoryID        *uuid.UUID `json:"category_id,omitempty"`
	CategoryName      string     `json:"category_name,omitempty"`
	Title             string     `json:"title,omitempty"`
	Description       string     `json:"description,omitempty"`
	SeverityLevel     string     `json:"severity_level,omitempty"`
	PriorityLevel     string     `json:"priority_level,omitempty"`
	AIConfidenceScore *float64   `json:"ai_confidence_score,omitempty"`
}

// Classifier is the swappable AI image classification contract.
// Implementations: Groq (default), and any OpenAI-compatible vision endpoint.
type Classifier interface {
	Name() string
	Model() string
	Classify(ctx context.Context, req ClassifyRequest) (*ClassifyResult, error)
}

func InvalidIssueResult() *ClassifyResult {
	return &ClassifyResult{
		IsValidIssue: false,
		Error:        "not_a_civic_issue",
		Message:      "Please upload a clear photo of a public civic issue.",
	}
}
