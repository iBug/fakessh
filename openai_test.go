package main

import (
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
)

func TestNewAIOutputGenerator_DefaultModelWhenEmpty(t *testing.T) {
	cfg := &Config{Output: OutputConfig{APIKey: "test-key"}}

	g, err := NewAIOutputGenerator(cfg)
	if assert.NoError(t, err) {
		assert.Equal(t, openai.GPT4oMini, g.model)
	}
}

func TestNewAIOutputGenerator_UsesConfiguredBaseURL(t *testing.T) {
	const baseURL = "https://example.com/v1"

	cfg := &Config{Output: OutputConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
	}}

	g, err := NewAIOutputGenerator(cfg)
	if assert.NoError(t, err) {
		assert.Equal(t, baseURL, g.baseURL)
	}
}
