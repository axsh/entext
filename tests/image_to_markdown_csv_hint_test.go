package tests

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/axsh/entext"
)

func TestImageToMarkdownCsvHintMissingPathValidation(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath:    filepath.Join("samples", "dummy.png"),
		OutputPath:   filepath.Join(t.TempDir(), "out.md"),
		CsvHintPaths: []string{filepath.Join(t.TempDir(), "missing.csv")},
	}, entext.ImageToMarkdownConfig{
		ServerURL: "http://localhost:3100",
		TernMode:    "external",
	})
	if err == nil || !entext.IsValidation(err) {
		t.Fatalf("expected validation error for missing csv hint, got %v", err)
	}
}

func TestConversionScopeRegressionWithCsvHintContract(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "01_変更履歴.md", nil, []string{
		"SymbolEvidence",
		"文字差異注記",
		"内容整合性",
		"意味対応・解釈",
	})
}
