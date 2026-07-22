package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestMainMissingInputExitCode2(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", ".", "-o", t.TempDir()+"/out.md")
	cmd.Dir = "."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected failure")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 && !strings.Contains(stderr.String(), "exit status 2") {
		t.Fatalf("expected exit 2, got %d stderr=%s", exitErr.ExitCode(), stderr.String())
	}
}
