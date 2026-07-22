package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

func TestMainMissingTemplateExitCode2(t *testing.T) {
	t.Parallel()
	cmd := exec.Command("go", "run", ".", "--structure", "s.md", "-o", "o.xlsx")
	cmd.Dir = "."
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected failure")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("%T %s", err, stderr.String())
	}
	if exitErr.ExitCode() != 2 && !strings.Contains(stderr.String(), "exit status 2") {
		t.Fatalf("exit=%d stderr=%s", exitErr.ExitCode(), stderr.String())
	}
}
