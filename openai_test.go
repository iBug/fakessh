package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

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

func TestAIOutputGenerator_InitCommandsDBCreatesMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "commands.sqlite3")
	g := &AIOutputGenerator{statePath: path}

	err := g.initCommandsDB()
	assert.NoError(t, err)
	defer g.db.Close()

	db, err := sql.Open("sqlite3", path)
	assert.NoError(t, err)
	defer db.Close()

	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'commands'").Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "commands", tableName)

	var tableCount int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'").Scan(&tableCount)
	assert.NoError(t, err)
	assert.Equal(t, 1, tableCount)
}

func TestAIOutputGenerator_SavedCommandUsesSQLiteCache(t *testing.T) {
	g := newTestAIOutputGenerator(t)

	err := g.setSavedCommand(g.normalizeCommand("  ls    -la  "), "cached output")
	assert.NoError(t, err)

	output, ok := g.getSavedCommand("ls -la")
	assert.True(t, ok)
	assert.Equal(t, "cached output", output)
}

func TestAIOutputGenerator_SavedCommandIgnoresStaleRows(t *testing.T) {
	g := newTestAIOutputGenerator(t)

	_, err := g.db.Exec(
		"INSERT INTO commands (command, output, updated_at) VALUES (?, ?, ?)",
		"whoami",
		"old output",
		time.Now().Add(-(cacheMaxAge + time.Hour)).UTC().Format(time.RFC3339Nano),
	)
	assert.NoError(t, err)

	output, ok := g.getSavedCommand("whoami")
	assert.False(t, ok)
	assert.Empty(t, output)
}

func TestAIOutputGenerator_SetSavedCommandUpdatesExistingRow(t *testing.T) {
	g := newTestAIOutputGenerator(t)

	assert.NoError(t, g.setSavedCommand("pwd", "first"))
	assert.NoError(t, g.setSavedCommand("pwd", "second"))

	output, ok := g.getSavedCommand("pwd")
	assert.True(t, ok)
	assert.Equal(t, "second", output)

	var count int
	err := g.db.QueryRow("SELECT COUNT(*) FROM commands WHERE command = ?", "pwd").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)

	err = g.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type = 'table'").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}

func newTestAIOutputGenerator(t *testing.T) *AIOutputGenerator {
	t.Helper()

	g := &AIOutputGenerator{statePath: filepath.Join(t.TempDir(), "commands.sqlite3")}
	assert.NoError(t, g.initCommandsDB())
	t.Cleanup(func() {
		if g.db != nil {
			_ = g.db.Close()
		}
	})
	return g
}
