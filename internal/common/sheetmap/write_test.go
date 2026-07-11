package sheetmap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "a.pdf")
	if err := os.WriteFile(pdfPath, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("failed to create dummy pdf: %v", err)
	}
	m := SheetMap{
		PageSheetNames: []string{"S1"},
		SheetEntries: []SheetEntry{
			{SheetIndex: 1, SheetName: "S1", ExportStatus: "success", PageCount: 1},
		},
	}
	sidecar, err := Write(pdfPath, "a.xlsx", m)
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}
	got, err := Read(sidecar)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if got.Version != Version {
		t.Fatalf("unexpected version: %d", got.Version)
	}
	if len(got.PageSheetNames) != 1 || got.PageSheetNames[0] != "S1" {
		t.Fatalf("unexpected page_sheet_names: %#v", got.PageSheetNames)
	}
}
