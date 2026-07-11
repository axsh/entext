package main

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainInvalidSheetsExitCode2(t *testing.T) {
	t.Parallel()
	cmd := exec.Command(
		"go", "run", ".",
		"--engine", "go-native",
		"--sheets", "a,b",
		"-i", filepath.Join("..", "..", "..", "..", "samples", "R06_09.xlsx"),
		"-o", t.TempDir(),
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
