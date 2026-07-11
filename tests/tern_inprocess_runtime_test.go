package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

func TestImageToMarkdownExternalModeFailsWithoutServer(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath:  "dummy.png",
		OutputPath: "dummy.md",
	}, entext.ImageToMarkdownConfig{
		ServerURL: "http://127.0.0.1:1",
		Agent:     "codex",
		Model:     "gpt-5.3-codex",
		TernMode:  "external",
	})
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if !strings.Contains(err.Error(), "tern external connect failed") {
		t.Fatalf("expected connect failed error, got %v", err)
	}
}

func TestImageToMarkdownInProcModeFailsWithoutConfig(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath:  "dummy.png",
		OutputPath: "dummy.md",
	}, entext.ImageToMarkdownConfig{
		ServerURL:      "http://localhost:3100",
		Agent:          "codex",
		Model:          "gpt-5.3-codex",
		TernMode:       "inproc",
		TernConfigPath: "not-found-tern-config.yaml",
	})
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if !strings.Contains(err.Error(), "tern config file not found") {
		t.Fatalf("expected config missing error, got %v", err)
	}
}

func TestImageToMarkdownInvalidModeValidation(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath:  "dummy.png",
		OutputPath: "dummy.md",
	}, entext.ImageToMarkdownConfig{
		ServerURL: "http://localhost:3100",
		Agent:     "codex",
		Model:     "gpt-5.3-codex",
		TernMode:  "invalid",
	})
	if err == nil {
		t.Fatalf("expected runtime error")
	}
	if !strings.Contains(err.Error(), "tern mode must be auto, external, or inproc") {
		t.Fatalf("unexpected error: %v", err)
	}
}
