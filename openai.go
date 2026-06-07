package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/sashabaranov/go-openai"
)

const (
	defaultSaveStatePath = "/var/lib/fakessh/state.db"
	cacheMaxAge          = 7 * 24 * time.Hour

	// Long commands and outputs are discarded to save space
	maxCommandLength = 256
	maxOutputLength  = 1024
)

//go:embed system_prompt.txt
var embeddedSystemPrompt string

// AIOutputGenerator 封装了调用 OpenAI 生成输出所需的全部状态。
type AIOutputGenerator struct {
	client       *openai.Client
	model        string
	statePath    string
	db           *sql.DB
	systemPrompt string
	baseURL      string
}

// NewAIOutputGenerator 根据配置构造一个基于 OpenAI 的输出生成器实例。
//
// 要求：cfg.Output.APIKey 必须非空，否则返回错误。
// 模型名称默认使用 openai.GPT4oMini，可通过配置 output.model 覆盖。
// 若配置了 output.base_url，则会覆盖默认的 OpenAI 接口地址。
func NewAIOutputGenerator(cfg *Config) (*AIOutputGenerator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	apiKey := strings.TrimSpace(cfg.Output.APIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("output.api_key is empty")
	}

	model := strings.TrimSpace(cfg.Output.Model)
	if model == "" {
		model = openai.GPT4oMini
	}

	baseURL := strings.TrimSpace(cfg.Output.BaseURL)

	config := openai.DefaultConfig(apiKey)
	if baseURL != "" {
		config.BaseURL = baseURL
	}

	client := openai.NewClientWithConfig(config)

	g := &AIOutputGenerator{
		client:       client,
		model:        model,
		statePath:    defaultSaveStatePath,
		systemPrompt: embeddedSystemPrompt,
		baseURL:      baseURL,
	}

	// 尝试初始化磁盘命令缓存，失败时仅记录日志并继续无缓存运行。
	if err := g.initDB(); err != nil {
		log.Printf("Error initializing saved command output database %s: %v", g.statePath, err)
	}

	return g, nil
}

func (g *AIOutputGenerator) initDB() error {
	if err := os.MkdirAll(filepath.Dir(g.statePath), 0o755); err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", g.statePath)
	if err != nil {
		return err
	}

	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS commands (
			id INTEGER PRIMARY KEY,
			command TEXT NOT NULL UNIQUE,
			output TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`); err != nil {
		_ = db.Close()
		return err
	}

	g.db = db
	return nil
}

func (g *AIOutputGenerator) getSavedCommand(cmd string) (string, bool) {
	if g.db == nil {
		return "", false
	}

	var output string
	cutoff := time.Now().Add(-cacheMaxAge).UTC().Format(time.RFC3339)
	err := g.db.QueryRow(
		"SELECT output FROM commands WHERE command = ? AND updated_at >= ?",
		cmd,
		cutoff,
	).Scan(&output)
	if err != nil {
		if err != sql.ErrNoRows {
			log.Printf("Error reading saved command output: %v", err)
		}
		return "", false
	}
	return output, true
}

func (g *AIOutputGenerator) setSavedCommand(cmd, output string) error {
	if g.db == nil {
		return nil
	}
	if len(cmd) >= maxCommandLength || len(output) >= maxOutputLength {
		return nil
	}

	_, err := g.db.Exec(`
		INSERT INTO commands (command, output, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT(command) DO UPDATE SET
			output = excluded.output,
			updated_at = excluded.updated_at
	`, cmd, output, time.Now().UTC().Format(time.RFC3339))
	return err
}

func (g *AIOutputGenerator) normalizeCommand(cmd string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(cmd)), " ")
}

func (g *AIOutputGenerator) createCompletion(ctx context.Context, cmd string, sshCtx SSHContext) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: g.systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: sshCtx.String(),
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: cmd,
		},
	}

	resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    g.model,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from OpenAI")
	}
	return resp.Choices[0].Message.Content, nil
}

// Generate 实现 OutputGenerator 接口，基于 OpenAI 生成命令输出并做本地缓存。
func (g *AIOutputGenerator) Generate(w io.Writer, cmd string, sshCtx SSHContext) error {
	normalized := g.normalizeCommand(cmd)
	if output, ok := g.getSavedCommand(normalized); ok {
		io.WriteString(w, output)
		if !strings.HasSuffix(output, "\n") {
			_, _ = w.Write([]byte("\n"))
		}
		return nil
	}

	output, err := g.createCompletion(context.Background(), normalized, sshCtx)
	if err == nil {
		// 持久化缓存属于 best-effort，如果失败只记录日志，不覆盖主流程错误。
		if errSave := g.setSavedCommand(normalized, output); errSave != nil {
			log.Printf("Error saving command output: %v", errSave)
		}
	}

	io.WriteString(w, output)
	if !strings.HasSuffix(output, "\n") {
		_, _ = w.Write([]byte("\n"))
	}

	return err
}
