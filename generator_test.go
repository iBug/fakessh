package main

import (
	"bytes"
	"io"
	"log"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type panicGenerator struct{}

func (panicGenerator) Generate(w io.Writer, cmd string, sshCtx SSHContext) error {
	panic("generator should not be called for special commands")
}

func TestGenerateOutput_HelpCommand(t *testing.T) {
	prev := generator
	generator = panicGenerator{}
	defer func() { generator = prev }()

	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	err := generateOutput(&buf, "help", sshCtx)
	assert.NoError(t, err)

	got := buf.String()
	want := "Glad you asked. This is " + myHomepage + ", go and read the code by yourself.\n"
	assert.Equal(t, want, got)
}

func TestGenerateOutput_EchoCommand(t *testing.T) {
	prev := generator
	generator = panicGenerator{}
	defer func() { generator = prev }()

	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	err := generateOutput(&buf, "echo hello world", sshCtx)
	assert.NoError(t, err)

	got := buf.String()
	assert.Equal(t, "hello world\n", got)
}

func TestGenerateOutput_SCPTCommand(t *testing.T) {
	prev := generator
	generator = panicGenerator{}
	defer func() { generator = prev }()

	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	err := generateOutput(&buf, "scp -t /tmp/foo", sshCtx)
	assert.NoError(t, err)

	got := buf.String()
	assert.Equal(t, "scp: Protocol error.", got)
}

func TestNewOutputGenerator_IntegrationWithLoadConfig(t *testing.T) {
	// Smoke test: ensure LoadConfig + NewOutputGenerator works together with missing file.
	logger := log.New(io.Discard, "", log.LstdFlags)
	cfg := LoadConfig("nonexistent-config.yml", logger)

	gen, err := NewOutputGenerator(cfg, logger)
	if assert.NoError(t, err) {
		assert.NotNil(t, gen)
	}
}
