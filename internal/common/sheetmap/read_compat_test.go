package sheetmap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCompatSetsDefaultVersion(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "a.sheet-map.json")
	raw := `{"source_xlsx":"a.xlsx","pdf_path":"a.pdf","page_sheet_names":["S1"],"sheet_entries":[{"sheet_index":1,"sheet_name":"S1","export_status":"success","page_count":1}]}`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
	got, err := ReadCompat(path)
	if err != nil {
		t.Fatalf("ReadCompat failed: %v", err)
	}
	if got.Version != Version {
		t.Fatalf("unexpected version: %d", got.Version)
	}
}
