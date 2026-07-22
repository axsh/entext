package tests

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

func TestRootAPIAnalyzeExcelTemplateValidation(t *testing.T) {
	_, err := entext.AnalyzeExcelTemplate(context.Background(), entext.ExcelTemplateAnalyzeJob{}, entext.ExcelTemplateAnalyzeConfig{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !entext.IsValidation(err) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func expectExitInvalidArgs(t *testing.T, err error, stderr string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected failure")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T stderr=%s", err, stderr)
	}
	if exitErr.ExitCode() != 2 && !strings.Contains(stderr, "exit status 2") {
		t.Fatalf("expected exit 2, got %d stderr=%s", exitErr.ExitCode(), stderr)
	}
}

func TestE2EExcelTemplateAnalyzeCLI_InvalidArgsExit2(t *testing.T) {
	cmd := toolCommand(t, "excel-template-analyze", "-o", filepath.Join(t.TempDir(), "out.md"))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	expectExitInvalidArgs(t, err, stderr.String())
}

func TestE2EExcelTemplateAnalyzeCLI_MissingPromptFileExit2(t *testing.T) {
	xlsx := filepath.Join("..", "samples", "R06_09.xlsx")
	if _, err := os.Stat(xlsx); err != nil {
		t.Fatalf("fixture missing: %v", err)
	}
	cmd := toolCommand(t, "excel-template-analyze",
		"-i", xlsx,
		"-o", filepath.Join(t.TempDir(), "out.md"),
		"--prompt-file", filepath.Join(t.TempDir(), "missing-hint.txt"),
		"--tern-mode", "external",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	expectExitInvalidArgs(t, err, stderr.String())
}
