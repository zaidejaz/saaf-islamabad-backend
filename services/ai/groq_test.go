package ai

import (
	"testing"

	"github.com/google/uuid"
)

func TestNormalizeResultValid(t *testing.T) {
	id := uuid.New()
	cats := []CategoryOption{{ID: id, Name: "Waste / Garbage"}}
	score := 1.5
	in := &ClassifyResult{
		IsValidIssue:      true,
		CategoryName:      "waste / garbage",
		Title:             "Garbage pile near G-9",
		Description:       "Large waste pile on the roadside.",
		SeverityLevel:     "MODERATE",
		PriorityLevel:     "High",
		AIConfidenceScore: &score,
	}
	out := normalizeResult(in, cats)
	if !out.IsValidIssue {
		t.Fatal("expected valid")
	}
	if out.CategoryID == nil || *out.CategoryID != id {
		t.Fatalf("category id mismatch: %v", out.CategoryID)
	}
	if out.CategoryName != "Waste / Garbage" {
		t.Fatalf("name = %q", out.CategoryName)
	}
	if out.SeverityLevel != "moderate" || out.PriorityLevel != "high" {
		t.Fatalf("levels = %s/%s", out.SeverityLevel, out.PriorityLevel)
	}
	if out.AIConfidenceScore == nil || *out.AIConfidenceScore != 1 {
		t.Fatalf("score = %v", out.AIConfidenceScore)
	}
}

func TestNormalizeResultInvalidCategory(t *testing.T) {
	cats := []CategoryOption{{ID: uuid.New(), Name: "Waste / Garbage"}}
	out := normalizeResult(&ClassifyResult{
		IsValidIssue: true,
		CategoryName: "Something Else",
		Title:        "x",
	}, cats)
	if out.IsValidIssue || out.Error != "not_a_civic_issue" {
		t.Fatalf("expected reject, got %+v", out)
	}
}

func TestLocalUploadPath(t *testing.T) {
	p, ok := localUploadPath("/uploads/issue.jpg")
	if !ok || p != "uploads/issue.jpg" {
		t.Fatalf("got %q ok=%v", p, ok)
	}
	if _, ok := localUploadPath("/uploads/../etc/passwd"); ok {
		t.Fatal("traversal should fail")
	}
	if _, ok := localUploadPath("http://localhost:8080/uploads/a.png"); !ok {
		t.Fatal("localhost upload url should resolve")
	}
}

func TestIsQwen36(t *testing.T) {
	if !isQwen36("qwen/qwen3.6-27b") {
		t.Fatal("expected qwen3.6 match")
	}
	if isQwen36("qwen/qwen3-32b") {
		t.Fatal("qwen3-32b should not match qwen3.6 helper")
	}
	if isQwen36("meta-llama/llama-4-scout-17b-16e-instruct") {
		t.Fatal("llama should not match")
	}
}

func TestFirstNonEmpty(t *testing.T) {
	if got := firstNonEmpty("", "  ", `{"ok":true}`); got != `{"ok":true}` {
		t.Fatalf("got %q", got)
	}
}
