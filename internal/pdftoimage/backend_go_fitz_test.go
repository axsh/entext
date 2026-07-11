package pdftoimage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/common/sheetmap"
	"github.com/jung-kurt/gofpdf"
)

func TestGoFitzBackendName(t *testing.T) {
	t.Parallel()
	if got := NewGoFitzBackend(200, nil).Name(); got != "go-fitz" {
		t.Fatalf("unexpected backend name: %s", got)
	}
}

func TestGoFitzBackendConvertMissingPDF(t *testing.T) {
	t.Parallel()
	backend := NewGoFitzBackend(200, nil)
	_, err := backend.Convert(context.Background(), filepath.Join(t.TempDir(), "missing.pdf"), t.TempDir(), "png")
	if err == nil {
		t.Fatalf("expected error for missing pdf")
	}
}

func TestGoFitzBackendUsesSheetMapName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "in.pdf")
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "", 16)
	pdf.Cell(40, 10, "entext")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		t.Fatalf("failed to create sample pdf: %v", err)
	}
	backend := NewGoFitzBackend(200, &sheetmap.SheetMap{PageSheetNames: []string{"First Sheet"}})
	paths, err := backend.Convert(context.Background(), pdfPath, dir, "png")
	if err != nil {
		t.Fatalf("unexpected convert error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output image, got %d", len(paths))
	}
	if !strings.Contains(filepath.Base(paths[0]), "First_Sheet") {
		t.Fatalf("unexpected output name: %s", filepath.Base(paths[0]))
	}
}
