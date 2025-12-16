package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
)

const (
	defaultSaveStatePath = "/var/lib/fakessh/commands.json"

	// Long commands and outputs are discarded to save space
	maxCommandLength = 256
	maxOutputLength  = 1024
)

type SaveState struct {
	Commands map[string]string `json:"commands"`
}

//go:embed system_prompt.txt
var embeddedSystemPrompt string

// AIOutputGenerator 封装了调用 OpenAI 生成输出所需的全部状态。
type AIOutputGenerator struct {
	client       *openai.Client
	model        string
	statePath    string
	saveState    SaveState
	saveStateMu  sync.RWMutex
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
		saveState:    SaveState{},
		systemPrompt: embeddedSystemPrompt,
		baseURL:      baseURL,
	}

	// 尝试从磁盘加载历史命令缓存，失败时仅记录日志并继续使用空缓存。
	if err := g.loadCommands(); err != nil {
		log.Printf("Error loading saved command output from %s: %v", g.statePath, err)
	}
	if g.saveState.Commands == nil {
		g.saveState.Commands = make(map[string]string)
	}

	return g, nil
}

func (g *AIOutputGenerator) loadCommands() error {
	f, err := os.Open(g.statePath)
	if err != nil {
		return err
	}
	defer f.Close()

	g.saveStateMu.Lock()
	defer g.saveStateMu.Unlock()
	return json.NewDecoder(f).Decode(&g.saveState)
}

func (g *AIOutputGenerator) saveCommands() error {
	f, err := os.Create(g.statePath)
	if err != nil {
		return err
	}
	defer f.Close()

	g.saveStateMu.RLock()
	cmdCopy := make(map[string]string, len(g.saveState.Commands))
	for k, v := range g.saveState.Commands {
		if len(k) < maxCommandLength && len(v) < maxOutputLength {
			cmdCopy[k] = v
		}
	}
	g.saveStateMu.RUnlock()

	saveStateCopy := SaveState{Commands: cmdCopy}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(&saveStateCopy)
}

func (g *AIOutputGenerator) getSavedCommand(cmd string) (string, bool) {
	g.saveStateMu.RLock()
	defer g.saveStateMu.RUnlock()
	output, ok := g.saveState.Commands[cmd]
	return output, ok
}

func (g *AIOutputGenerator) setSavedCommand(cmd, output string) error {
	g.saveStateMu.Lock()
	if g.saveState.Commands == nil {
		g.saveState.Commands = make(map[string]string)
	}
	g.saveState.Commands[cmd] = output
	g.saveStateMu.Unlock()
	return g.saveCommands()
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
