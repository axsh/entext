package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestMainInvalidTernModeExitCode2(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(
		"go", "run", ".",
		"--tern-mode", "bad",
		"-i", "dummy.png",
		"--output-dir", t.TempDir(),
	)
	cmd.Dir = "."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 && !strings.Contains(stderr.String(), "exit status 2") {
		t.Fatalf("expected exit code 2, got %d stderr=%s", exitErr.ExitCode(), stderr.String())
	}
}

func TestMainInProcMissingConfigExitCode1(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(
		"go", "run", ".",
		"--tern-mode", "inproc",
		"--tern-config", "not-found-config.yaml",
		"-i", "dummy.png",
		"--output-dir", t.TempDir(),
	)
	cmd.Dir = "."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%s", exitErr.ExitCode(), stderr.String())
	}
}

func TestMainVerboseKeepsStdoutCleanOnFailure(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(
		"go", "run", ".",
		"--tern-mode", "external",
		"--server", "http://127.0.0.1:1",
		"--verbose",
		"-i", "dummy.png",
		"--output-dir", t.TempDir(),
	)
	cmd.Dir = "."
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure")
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("stdout must remain machine-readable, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "starting image analysis") {
		t.Fatalf("expected verbose debug log in stderr, got %s", stderr.String())
	}
}
