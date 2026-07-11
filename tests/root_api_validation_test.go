package tests

import (
	"context"
	"testing"

	"github.com/axsh/entext"
)

func TestRootPublicTypesAndFunctionsCompile(t *testing.T) {
	t.Parallel()

	var _ entext.FileJob
	var _ entext.FileArtifact
	var _ entext.ExcelPDFOptions
	var _ entext.ExcelCSVOptions
	var _ entext.PDFImageOptions
	var _ entext.ImageToMarkdownConfig

	_, _ = entext.ConvertExcelToPDFWithOptions(context.Background(), entext.FileJob{
		InputPath: "dummy.xlsx",
		OutputDir: ".",
	}, entext.ExcelPDFOptions{
		Backend: "auto",
		Engine:  "legacy",
	})

	_, _ = entext.ConvertPDFToImageWithOptions(context.Background(), entext.FileJob{
		InputPath: "dummy.pdf",
		OutputDir: ".",
	}, "png", entext.PDFImageOptions{
		Backend: "auto",
		Engine:  "legacy",
		DPI:     200,
	})

	_, _ = entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath:  "dummy.png",
		OutputPath: "dummy.md",
	}, entext.ImageToMarkdownConfig{
		ServerURL:       "http://localhost:3100",
		Agent:           "codex",
		Model:           "gpt-5.3-codex",
		TernMode:        "external",
		TernConfigPath:  "settings/tern/tern-config.yaml",
		Verbose:         true,
		Quiet:           false,
		StrictGapJudge:  true,
		SaveQuestionLog: true,
		RoundSleepMS:    0,
		PhaseSleepMS:    0,
		MaxRounds:       3,
	})
}

func TestRootAPIValidationErrors(t *testing.T) {
	t.Parallel()

	_, err := entext.ConvertExcelToPDF(context.Background(), entext.FileJob{})
	if err == nil || !entext.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}

	_, err = entext.ConvertPDFToImage(context.Background(), entext.FileJob{
		InputPath: "a.pdf",
		OutputDir: "out",
	}, "gif")
	if err == nil || !entext.IsValidation(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}
