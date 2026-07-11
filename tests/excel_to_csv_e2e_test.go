package tests

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

func TestE2EExcelToCsvInvalidBackendExitCode2(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "csv-invalid-backend")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := toolCommand(
		t,
		"excel-to-csv",
		"--backend", "invalid",
		"-i", filepath.Join(repoRootAbs(t), "samples", "R06_09.xlsx"),
		"-o", tmpDir,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure for invalid backend")
	}
	if !strings.Contains(stderr.String(), "excel backend must be auto, libreoffice, or excel-com") {
		t.Fatalf("expected backend validation message, stderr=%s", stderr.String())
	}
}

func TestE2EExcelToCsvInvalidSheetsExitCode2(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "csv-invalid-sheets")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := toolCommand(
		t,
		"excel-to-csv",
		"--sheets", "a,b",
		"-i", filepath.Join(repoRootAbs(t), "samples", "R06_09.xlsx"),
		"-o", tmpDir,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure for invalid sheets")
	}
	if !strings.Contains(stderr.String(), "invalid --sheets value") {
		t.Fatalf("expected sheets validation message, stderr=%s", stderr.String())
	}
}

func TestE2EExcelToCsvComExportsKnownCellText(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "csv-com-export")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := toolCommand(
		t,
		"excel-to-csv",
		"--backend", "excel-com",
		"-i", filepath.Join(repoRootAbs(t), "samples", "R06_09.xlsx"),
		"-o", tmpDir,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("excel-to-csv excel-com failed: %v stderr=%s", err, stderr.String())
	}
	csvPath := firstPath(stdout.String())
	if csvPath == "" {
		t.Fatalf("excel-to-csv stdout did not contain output path: %q", stdout.String())
	}
	csvPath = resolveToolOutputPath(csvPath)
	data, err := os.ReadFile(csvPath)
	if err != nil {
		t.Fatalf("failed to read csv output: %v", err)
	}
	if len(data) < 3 || data[0] != 0xEF || data[1] != 0xBB || data[2] != 0xBF {
		t.Fatalf("expected UTF-8 BOM in csv output")
	}
	body := string(data[3:])
	if !strings.Contains(body, ",") {
		t.Fatalf("expected comma-separated csv content, got: %q", body[:min(80, len(body))])
	}
	if strings.TrimSpace(body) == "" {
		t.Fatalf("expected non-empty csv body")
	}
}

func TestRootAPIConvertExcelToCSVValidation(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertExcelToCSVWithOptions(context.Background(), entext.FileJob{}, entext.ExcelCSVOptions{})
	if err == nil || !entext.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
