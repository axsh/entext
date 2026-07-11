package exceltocsv

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureUTF8BOMAddsPrefix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.csv")
	if err := os.WriteFile(path, []byte("a,b\n1,2"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ensureUTF8BOM(path); err != nil {
		t.Fatalf("ensureUTF8BOM failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(data) < 3 || data[0] != 0xEF || data[1] != 0xBB || data[2] != 0xBF {
		t.Fatalf("expected UTF-8 BOM prefix, got %v", data[:min(3, len(data))])
	}
}

func TestEnsureUTF8BOMIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.csv")
	withBOM := append([]byte{0xEF, 0xBB, 0xBF}, []byte("x")...)
	if err := os.WriteFile(path, withBOM, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := ensureUTF8BOM(path); err != nil {
		t.Fatalf("ensureUTF8BOM failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if len(data) != len(withBOM) {
		t.Fatalf("expected unchanged length %d got %d", len(withBOM), len(data))
	}
}
