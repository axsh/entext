package csvhint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/common/apperr"
)

func TestResolveCsvHintsExplicitPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	first := filepath.Join(dir, "a.csv")
	second := filepath.Join(dir, "b.csv")
	if err := os.WriteFile(first, []byte("a,b\n1,2"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(second, []byte("x,y\n3,4"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	got, err := ResolveCsvHints([]string{first, second}, filepath.Join(dir, "img.png"), false)
	if err != nil {
		t.Fatalf("ResolveCsvHints failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 hints, got %d", len(got))
	}
	if !strings.Contains(got[0].Content, "1,2") || !strings.Contains(got[1].Content, "3,4") {
		t.Fatalf("unexpected hint contents: %#v", got)
	}
}

func TestResolveCsvHintsAutoSameDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	image := filepath.Join(dir, "01_foo.png")
	csv := filepath.Join(dir, "01_foo.csv")
	if err := os.WriteFile(csv, []byte("header\nvalue"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	got, err := ResolveCsvHints(nil, image, false)
	if err != nil {
		t.Fatalf("ResolveCsvHints failed: %v", err)
	}
	if len(got) != 1 || got[0].Path != csv {
		t.Fatalf("unexpected hints: %#v", got)
	}
}

func TestResolveCsvHintsAutoParentCsvDir(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	imageDir := filepath.Join(root, "images")
	csvDir := filepath.Join(root, "csv")
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	if err := os.MkdirAll(csvDir, 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}
	image := filepath.Join(imageDir, "01_foo.png")
	csv := filepath.Join(csvDir, "01_foo.csv")
	if err := os.WriteFile(csv, []byte("auto,resolved"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	got, err := ResolveCsvHints(nil, image, false)
	if err != nil {
		t.Fatalf("ResolveCsvHints failed: %v", err)
	}
	if len(got) != 1 || got[0].Path != csv {
		t.Fatalf("unexpected hints: %#v", got)
	}
}

func TestResolveCsvHintsMissingReturnsEmpty(t *testing.T) {
	t.Parallel()
	got, err := ResolveCsvHints(nil, filepath.Join(t.TempDir(), "missing.png"), false)
	if err != nil {
		t.Fatalf("ResolveCsvHints failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty hints, got %#v", got)
	}
}

func TestResolveCsvHintsExplicitMissingIsError(t *testing.T) {
	t.Parallel()
	_, err := ResolveCsvHints([]string{filepath.Join(t.TempDir(), "missing.csv")}, "img.png", false)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestResolveCsvHintsExplicitDisablesAuto(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	image := filepath.Join(dir, "01_foo.png")
	auto := filepath.Join(dir, "01_foo.csv")
	explicit := filepath.Join(dir, "manual.csv")
	if err := os.WriteFile(auto, []byte("auto"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := os.WriteFile(explicit, []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	got, err := ResolveCsvHints([]string{explicit}, image, false)
	if err != nil {
		t.Fatalf("ResolveCsvHints failed: %v", err)
	}
	if len(got) != 1 || got[0].Path != explicit {
		t.Fatalf("expected explicit only, got %#v", got)
	}
}
