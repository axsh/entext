package tests

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

func TestRootAPIFillExcelValidation(t *testing.T) {
	_, err := entext.FillExcel(context.Background(), entext.ExcelFillJob{}, entext.ExcelFillConfig{})
	if err == nil || !entext.IsValidation(err) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestE2EExcelFillCLI_InvalidArgsExit2(t *testing.T) {
	cmd := toolCommand(t, "excel-fill", "-o", filepath.Join(t.TempDir(), "o.xlsx"))
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
