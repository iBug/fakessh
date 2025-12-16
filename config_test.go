package main

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

	err := os.WriteFile(path, content, 0o644)
	assert.NoError(t, err, "failed to write temp config")

	cfg := LoadConfig(path, log.New(io.Discard, "", log.LstdFlags))
	assert.Equal(t, "openai", cfg.Output.Mode)
	assert.Equal(t, "test-key", cfg.Output.APIKey)
	assert.Equal(t, "custom-model", cfg.Output.Model)
	assert.Equal(t, "https://example.com/v1", cfg.Output.BaseURL)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	content := []byte("output: [invalid")

	err := os.WriteFile(path, content, 0o644)
	assert.NoError(t, err, "failed to write temp config")

	cfg := LoadConfig(path, log.New(io.Discard, "", log.LstdFlags))
	assert.Empty(t, cfg.Output.Mode)
	assert.Empty(t, cfg.Output.APIKey)
	assert.Empty(t, cfg.Output.Model)
	assert.Empty(t, cfg.Output.BaseURL)
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
			if assert.NoError(t, err) {
				assert.NotNil(t, gen)
				assert.IsType(t, tt.want, gen)
			}
		})
	}
}

func TestDefaultOutputGenerator_WritesDataWithNewline(t *testing.T) {
	g := &DefaultOutputGenerator{}
	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	err := g.Generate(&buf, "ls -la", sshCtx)
	assert.NoError(t, err)

	b := buf.Bytes()
	assert.NotEmpty(t, b)
	assert.Equal(t, byte('\n'), b[len(b)-1])
}

func TestNewOutputGenerator_OpenAIModeWithoutAPIKey(t *testing.T) {
	logger := log.New(io.Discard, "", log.LstdFlags)
	cfg := &Config{Output: OutputConfig{Mode: "openai"}}

	gen, err := NewOutputGenerator(cfg, logger)
	assert.Error(t, err)
	assert.Nil(t, gen)
}
