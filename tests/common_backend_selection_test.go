package tests

import (
	"context"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

func TestExcelToPDFBackendSpecifiedNoFallback(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertExcelToPDFWithBackend(context.Background(), entext.FileJob{
		InputPath: "notfound.xlsx",
		OutputDir: ".",
	}, "libreoffice")
	if err == nil {
		t.Fatalf("expected conversion error")
	}
	if !strings.Contains(err.Error(), "all backends failed") {
		t.Fatalf("expected aggregate error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "libreoffice(") {
		t.Fatalf("expected libreoffice attempt in message, got: %v", err)
	}
}

func TestPDFToImageBackendSpecifiedNoFallback(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertPDFToImageWithBackend(context.Background(), entext.FileJob{
		InputPath: "notfound.pdf",
		OutputDir: ".",
	}, "png", "pdftoppm")
	if err == nil {
		t.Fatalf("expected conversion error")
	}
	if !strings.Contains(err.Error(), "all backends failed") {
		t.Fatalf("expected aggregate error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "pdftoppm(") {
		t.Fatalf("expected pdftoppm attempt in message, got: %v", err)
	}
}

func TestExcelToPDFEngineGoNativeValidation(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertExcelToPDFWithOptions(context.Background(), entext.FileJob{
		InputPath: "samples/R06_09.xlsx",
		OutputDir: ".",
	}, entext.ExcelPDFOptions{
		Backend: "auto",
		Engine:  "invalid",
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !entext.IsValidation(err) {
		t.Fatalf("expected validation error type, got: %v", err)
	}
}

func TestPDFToImageEngineGoNativeValidation(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertPDFToImageWithOptions(context.Background(), entext.FileJob{
		InputPath: "notfound.pdf",
		OutputDir: ".",
	}, "png", entext.PDFImageOptions{
		Backend: "auto",
		Engine:  "invalid",
		DPI:     200,
	})
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !entext.IsValidation(err) {
		t.Fatalf("expected validation error type, got: %v", err)
	}
}

func TestPDFToImageGoNativeUsesGoFitzOnly(t *testing.T) {
	t.Parallel()
	_, err := entext.ConvertPDFToImageWithOptions(context.Background(), entext.FileJob{
		InputPath: "notfound.pdf",
		OutputDir: ".",
	}, "png", entext.PDFImageOptions{
		Backend: "auto",
		Engine:  "go-native",
		DPI:     200,
	})
	if err == nil {
		t.Fatalf("expected conversion error")
	}
	errText := err.Error()
	if !strings.Contains(errText, "go-fitz(") {
		t.Fatalf("expected go-fitz attempt, got: %v", err)
	}
	if strings.Contains(errText, "pdftoppm(") || strings.Contains(errText, "magick(") {
		t.Fatalf("go-native should not use legacy backends, got: %v", err)
	}
}
