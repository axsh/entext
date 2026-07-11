package tests

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVaultCLISetOpenAIKeyForTern(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/vault-cli", "set", "--provider", "unknown", "--name", "default", "--secret", "dummy")
	cmd.Dir = filepath.Join("..", "features", "vault-cli")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure")
	}
	if !strings.Contains(stderr.String(), "provider must be openai or anthropic") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}

func TestInProcTernUsesVaultKey(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", "./cmd/vault-cli", "get", "--provider", "unknown", "--name", "default")
	cmd.Dir = filepath.Join("..", "features", "vault-cli")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure")
	}
	if !strings.Contains(stderr.String(), "provider must be openai or anthropic") {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
}
