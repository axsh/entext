package tests

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext"
	"gopkg.in/yaml.v2"
)

type ternConfigFile struct {
	LLMGateway struct {
		Port              int    `yaml:"port"`
		ModelProfilesPath string `yaml:"model_profiles_path"`
	} `yaml:"llm_gateway"`
}

func TestTernConfigRelativeModelProfilesPath(t *testing.T) {
	t.Parallel()
	path := filepath.Join("..", "settings", "tern", "tern-config.yaml")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config failed: %v", err)
	}
	var cfg ternConfigFile
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("yaml parse failed: %v", err)
	}
	if cfg.LLMGateway.ModelProfilesPath != "./model_profiles.yaml" {
		t.Fatalf("model_profiles_path must be relative, got %s", cfg.LLMGateway.ModelProfilesPath)
	}
}

func TestTernConfigSearchOrder(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath:  "dummy.png",
		OutputPath: "dummy.md",
	}, entext.ImageToMarkdownConfig{
		Agent:    "codex",
		Model:    "gpt-5.3-codex",
		TernMode: "inproc",
	})
	if err == nil {
		t.Fatalf("expected inproc config discovery error")
	}
	if !strings.Contains(err.Error(), "tern config file not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
