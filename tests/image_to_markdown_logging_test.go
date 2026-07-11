package tests

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerboseLogsContainCoreFields(t *testing.T) {
	t.Parallel()

	cmd := imageToMarkdownCommand(
		t,
		"--tern-mode", "external",
		"--server", "http://127.0.0.1:1",
		"--verbose",
		"-i", "dummy.png",
		"--output-dir", t.TempDir(),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected command to fail without external tern server")
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("stdout should contain no logs, got %q", stdout.String())
	}
	logText := stderr.String()
	required := []string{
		"starting image analysis",
		"input=dummy.png",
		"model=gpt-5.3-codex",
	}
	for _, token := range required {
		if !strings.Contains(logText, token) {
			t.Fatalf("missing verbose log token %q in stderr=%s", token, logText)
		}
	}
}

func TestQuietSuppressesVerboseDebugLogs(t *testing.T) {
	t.Parallel()

	cmd := imageToMarkdownCommand(
		t,
		"--tern-mode", "external",
		"--server", "http://127.0.0.1:1",
		"--verbose",
		"--quiet",
		"-i", "dummy.png",
		"--output-dir", t.TempDir(),
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatalf("expected command to fail without external tern server")
	}
	if strings.Contains(stderr.String(), "starting image analysis") {
		t.Fatalf("quiet mode must suppress debug logs, stderr=%s", stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("stdout should contain no logs, got %q", stdout.String())
	}
}

func imageToMarkdownCommand(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	goArgs := []string{"run", "./cmd/image-to-markdown"}
	goArgs = append(goArgs, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = filepath.Join("..")
	return cmd
}
