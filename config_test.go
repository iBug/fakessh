package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadConfig_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte(`output:
  mode: openai
  api-key: test-key
  model: custom-model
  baseurl: https://example.com/v1
`)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg := LoadConfig(path, log.New(io.Discard, "", log.LstdFlags))
	if cfg.Output.Mode != "openai" {
		t.Errorf("expected mode 'openai', got %q", cfg.Output.Mode)
	}
	if cfg.Output.APIKey != "test-key" {
		t.Errorf("expected api-key 'test-key', got %q", cfg.Output.APIKey)
	}
	if cfg.Output.Model != "custom-model" {
		t.Errorf("expected model 'custom-model', got %q", cfg.Output.Model)
	}
	if cfg.Output.BaseURL != "https://example.com/v1" {
		t.Errorf("expected base_url 'https://example.com/v1', got %q", cfg.Output.BaseURL)
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte("output: [invalid")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg := LoadConfig(path, log.New(io.Discard, "", log.LstdFlags))
	if cfg.Output.Mode != "" {
		t.Fatalf("expected empty mode for invalid config, got %q", cfg.Output.Mode)
	}
	if cfg.Output.APIKey != "" {
		t.Fatalf("expected empty api_key for invalid config, got %q", cfg.Output.APIKey)
	}
	if cfg.Output.Model != "" {
		t.Fatalf("expected empty model for invalid config, got %q", cfg.Output.Model)
	}
	if cfg.Output.BaseURL != "" {
		t.Fatalf("expected empty base_url for invalid config, got %q", cfg.Output.BaseURL)
	}
}

func TestNewOutputGenerator_SelectionLogic(t *testing.T) {
	logger := log.New(io.Discard, "", log.LstdFlags)

	tests := []struct {
		name   string
		mode   string
		apiKey string
		want   interface{}
	}{
		{"openai_with_key", "openai", "test-key", &AIOutputGenerator{}},
		{"default_mode", "default", "", &DefaultOutputGenerator{}},
		{"default_mode_with_key", "default", "ignored-key", &DefaultOutputGenerator{}},
		{"junk_alias", "junk", "", &DefaultOutputGenerator{}},
		{"auto_with_key", "auto", "test-key", &AIOutputGenerator{}},
		{"auto_without_key", "auto", "", &DefaultOutputGenerator{}},
		{"empty_with_key", "", "test-key", &AIOutputGenerator{}},
		{"empty_without_key", "", "", &DefaultOutputGenerator{}},
		{"invalid_with_key", "something", "test-key", &AIOutputGenerator{}},
		{"invalid_without_key", "something", "", &DefaultOutputGenerator{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Output: OutputConfig{
				Mode:   tt.mode,
				APIKey: tt.apiKey,
			}}
			gen, err := NewOutputGenerator(cfg, logger)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gen == nil {
				t.Fatalf("expected non-nil generator")
			}
			if fmt.Sprintf("%T", gen) != fmt.Sprintf("%T", tt.want) {
				t.Fatalf("unexpected generator type: got %T, want %T", gen, tt.want)
			}
		})
	}
}

func TestDefaultOutputGenerator_WritesDataWithNewline(t *testing.T) {
	g := &DefaultOutputGenerator{}
	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	if err := g.Generate(&buf, "ls -la", sshCtx); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	b := buf.Bytes()
	if len(b) == 0 {
		t.Fatalf("expected some output, got empty buffer")
	}
	if b[len(b)-1] != '\n' {
		t.Fatalf("expected output to end with newline, got %q", b[len(b)-1])
	}
}

func TestNewOutputGenerator_OpenAIModeWithoutAPIKey(t *testing.T) {
	logger := log.New(io.Discard, "", log.LstdFlags)
	cfg := &Config{Output: OutputConfig{Mode: "openai"}}
	gen, err := NewOutputGenerator(cfg, logger)
	if err == nil {
		t.Fatalf("expected error for openai mode without api_key, got nil (generator=%T)", gen)
	}
}
