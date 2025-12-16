package main

import (
	crand "crypto/rand"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"strings"
)

type OutputGenerator interface {
	Generate(w io.Writer, cmd string, sshCtx SSHContext) error
}

type DefaultOutputGenerator struct{}

func (g *DefaultOutputGenerator) Generate(w io.Writer, cmd string, sshCtx SSHContext) error {
	cmdlen := len(cmd)
	defaultSize := cmdlen + mathrand.Intn(3*cmdlen)
	if defaultSize < 0 {
		defaultSize = 0
	}
	if defaultSize > 0 {
		if _, err := io.CopyN(w, crand.Reader, int64(defaultSize)); err != nil {
			return err
		}
	}
	_, err := w.Write([]byte{'\n'})
	return err
}

var generator OutputGenerator

// NewOutputGenerator 根据配置选择具体的输出生成器实现。
func NewOutputGenerator(cfg *Config, logger *log.Logger) (OutputGenerator, error) {
	mode := ""
	apiKey := ""
	if cfg != nil {
		mode = strings.ToLower(strings.TrimSpace(cfg.Output.Mode))
		apiKey = strings.TrimSpace(cfg.Output.APIKey)
	}

	switch mode {
	case "openai":
		if apiKey == "" {
			return nil, fmt.Errorf("output.mode is 'openai' but output.api_key is empty")
		}
		gen, err := NewAIOutputGenerator(cfg)
		if err != nil {
			return nil, err
		}
		return gen, nil
	case "default":
		return &DefaultOutputGenerator{}, nil
	case "junk":
		if logger != nil {
			logger.Print("output mode 'junk' is deprecated, treating as 'default'")
		}
		return &DefaultOutputGenerator{}, nil
	case "auto", "":
		fallthrough
	default:
		if mode != "" && mode != "auto" && logger != nil {
			logger.Printf("unknown output mode %q, falling back to auto selection", mode)
		}
		if apiKey != "" {
			gen, err := NewAIOutputGenerator(cfg)
			if err != nil {
				if logger != nil {
					logger.Printf("failed to initialize OpenAI client, falling back to default output: %v", err)
				}
				return &DefaultOutputGenerator{}, nil
			}
			return gen, nil
		}
		return &DefaultOutputGenerator{}, nil
	}
}
