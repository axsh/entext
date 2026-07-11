package pdfnative

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/jung-kurt/gofpdf"
)

func TestMergePDFsRequiresInput(t *testing.T) {
	t.Parallel()
	err := MergePDFs(nil, filepath.Join(t.TempDir(), "out.pdf"))
	if err == nil {
		t.Fatalf("expected error for empty merge input")
	}
}

func TestRenderPagesRejectsFormat(t *testing.T) {
	t.Parallel()
	_, err := RenderPages("dummy.pdf", 200, "gif")
	if err == nil {
		t.Fatalf("expected unsupported format error")
	}
}

func TestCountPagesMissingFile(t *testing.T) {
	t.Parallel()
	_, err := CountPages(filepath.Join(t.TempDir(), "missing.pdf"))
	if err == nil {
		t.Fatalf("expected missing file error")
	}
}

func TestRenderPagesMissingFile(t *testing.T) {
	t.Parallel()
	_, err := RenderPages(filepath.Join(t.TempDir(), "missing.pdf"), 200, "png")
	if err == nil {
		t.Fatalf("expected error for missing pdf")
	}
}

func TestRenderPagesDPIAffectsSize(t *testing.T) {
	t.Parallel()
	pdfPath := filepath.Join(t.TempDir(), "sample.pdf")
	makeSamplePDF(t, pdfPath)
	p200, err := RenderPages(pdfPath, 200, "png")
	if err != nil {
		t.Fatalf("render 200dpi failed: %v", err)
	}
	p300, err := RenderPages(pdfPath, 300, "png")
	if err != nil {
		t.Fatalf("render 300dpi failed: %v", err)
	}
	if len(p200) != 1 || len(p300) != 1 {
		t.Fatalf("unexpected rendered page count: p200=%d p300=%d", len(p200), len(p300))
	}
	img200, err := png.DecodeConfig(bytes.NewReader(p200[0].Bytes))
	if err != nil {
		t.Fatalf("decode 200dpi image failed: %v", err)
	}
	img300, err := png.DecodeConfig(bytes.NewReader(p300[0].Bytes))
	if err != nil {
		t.Fatalf("decode 300dpi image failed: %v", err)
	}
	if img300.Width <= img200.Width {
		t.Fatalf("expected 300dpi width > 200dpi width: %d <= %d", img300.Width, img200.Width)
	}
}

func makeSamplePDF(t *testing.T, out string) {
	t.Helper()
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 16)
	pdf.Cell(40, 10, "entext")
	if err := pdf.OutputFileAndClose(out); err != nil {
		t.Fatalf("failed to create sample pdf: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("sample pdf missing: %v", err)
	}
}
