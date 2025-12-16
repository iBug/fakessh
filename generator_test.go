package main

import (
	"bytes"
	"io"
	"log"
	"testing"
	"time"
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

	if err := generateOutput(&buf, "help", sshCtx); err != nil {
		t.Fatalf("generateOutput returned error: %v", err)
	}

	got := buf.String()
	want := "Glad you asked. This is " + myHomepage + ", go and read the code by yourself.\n"
	if got != want {
		t.Fatalf("unexpected help output:\n got: %q\nwant: %q", got, want)
	}
}

func TestGenerateOutput_EchoCommand(t *testing.T) {
	prev := generator
	generator = panicGenerator{}
	defer func() { generator = prev }()

	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	if err := generateOutput(&buf, "echo hello world", sshCtx); err != nil {
		t.Fatalf("generateOutput returned error: %v", err)
	}

	got := buf.String()
	if got != "hello world\n" {
		t.Fatalf("unexpected echo output: got %q, want %q", got, "hello world\n")
	}
}

func TestGenerateOutput_SCPTCommand(t *testing.T) {
	prev := generator
	generator = panicGenerator{}
	defer func() { generator = prev }()

	var buf bytes.Buffer
	sshCtx := SSHContext{Hostname: "host", User: "user", T: time.Now()}

	if err := generateOutput(&buf, "scp -t /tmp/foo", sshCtx); err != nil {
		t.Fatalf("generateOutput returned error: %v", err)
	}

	got := buf.String()
	if got != "scp: Protocol error." {
		t.Fatalf("unexpected scp -t output: got %q, want %q", got, "scp: Protocol error.")
	}
}

func TestNewOutputGenerator_IntegrationWithLoadConfig(t *testing.T) {
	// Smoke test: ensure LoadConfig + NewOutputGenerator works together with missing file.
	logger := log.New(io.Discard, "", log.LstdFlags)
	cfg := LoadConfig("nonexistent-config.yml", logger)
	gen, err := NewOutputGenerator(cfg, logger)
	if err != nil {
		t.Fatalf("unexpected error from NewOutputGenerator: %v", err)
	}
	if gen == nil {
		t.Fatalf("expected non-nil generator for missing config")
	}
}
