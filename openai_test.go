package main

import (
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestNewAIOutputGenerator_DefaultModelWhenEmpty(t *testing.T) {
	cfg := &Config{Output: OutputConfig{APIKey: "test-key"}}

	g, err := NewAIOutputGenerator(cfg)
	if err != nil {
		t.Fatalf("NewAIOutputGenerator returned error: %v", err)
	}

	if g.model != openai.GPT4oMini {
		t.Fatalf("expected default model %q, got %q", openai.GPT4oMini, g.model)
	}
}

func TestNewAIOutputGenerator_UsesConfiguredBaseURL(t *testing.T) {
	const baseURL = "https://example.com/v1"

	cfg := &Config{Output: OutputConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
	}}

	g, err := NewAIOutputGenerator(cfg)
	if err != nil {
		t.Fatalf("NewAIOutputGenerator returned error: %v", err)
	}

	if g.baseURL != baseURL {
		t.Fatalf("expected baseURL %q, got %q", baseURL, g.baseURL)
	}
}
