package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/sashabaranov/go-openai"
)

const (
	defaultSaveStatePath = "/var/lib/fakessh/commands.json"

	rfc2822 = "Mon Jan 02 15:04:05 -0700 2006"
)

type SaveState struct {
	Commands map[string]string `json:"commands"`
}

var (
	client      *openai.Client
	statePath   string
	saveState   SaveState
	saveStateMu sync.RWMutex

	//go:embed system_prompt.txt
	systemPrompt string
)

func loadCommands() error {
	f, err := os.Open(defaultSaveStatePath)
	if err != nil {
		return err
	}
	defer f.Close()
	saveStateMu.Lock()
	defer saveStateMu.Unlock()
	return json.NewDecoder(f).Decode(&saveState)
}

func saveCommands() error {
	f, err := os.Create(defaultSaveStatePath)
	if err != nil {
		return err
	}
	defer f.Close()
	saveStateMu.RLock()
	defer saveStateMu.RUnlock()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(&saveState)
}

func getSavedCommand(cmd string) (string, bool) {
	saveStateMu.RLock()
	output, ok := saveState.Commands[cmd]
	saveStateMu.RUnlock()
	return output, ok
}

func setSavedCommand(cmd, output string) error {
	saveStateMu.Lock()
	saveState.Commands[cmd] = output
	saveStateMu.Unlock()
	return saveCommands()
}

func normalizeCommand(cmd string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(cmd)), " ")
}

func createCompletion(ctx context.Context, cmd string) (string, error) {
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: cmd,
		},
	}
	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    openai.GPT3Dot5Turbo,
		Messages: messages,
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}

func generateOutputOpenAI(cmd string) (string, error) {
	cmd = normalizeCommand(cmd)
	if output, ok := getSavedCommand(cmd); ok {
		return output, nil
	}
	output, err := createCompletion(context.Background(), cmd)
	if err == nil {
		err = setSavedCommand(cmd, output)
	}
	return output, err
}

func init() {
	flag.StringVar(&statePath, "state", defaultSaveStatePath, "save state file path for OpenAI")

	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return
	}
	log.Print("Found OpenAI API key, enabling AI generation")
	client = openai.NewClient(key)
	if err := loadCommands(); err != nil {
		log.Print("Error loading saved command output:", err)
	}
	if saveState.Commands == nil {
		saveState.Commands = make(map[string]string)
	}
}
