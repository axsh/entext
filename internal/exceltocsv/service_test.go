package exceltocsv

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	backenderr "github.com/axsh/entext/internal/common/backend"
)

type fakeCsvBackend struct {
	name string
	out  []string
	err  error
}

func (f fakeCsvBackend) Name() string {
	return f.name
}

func (f fakeCsvBackend) ConvertSheets(_ context.Context, _, _ string, _ []int) ([]string, error) {
	return f.out, f.err
}

func TestResolveBackendChainAutoOnWindows(t *testing.T) {
	t.Parallel()
	chain, err := resolveBackendChain(BackendAuto, "windows")
	if err != nil {
		t.Fatalf("resolveBackendChain returned error: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(chain))
	}
	if chain[0].Name() != BackendExcelCOM {
		t.Fatalf("expected first backend excel-com, got %s", chain[0].Name())
	}
	if chain[1].Name() != BackendLibreOffice {
		t.Fatalf("expected second backend libreoffice, got %s", chain[1].Name())
	}
}

func TestResolveBackendChainSpecificMode(t *testing.T) {
	t.Parallel()
	chain, err := resolveBackendChain(BackendLibreOffice, "windows")
	if err != nil {
		t.Fatalf("resolveBackendChain returned error: %v", err)
	}
	if len(chain) != 1 || chain[0].Name() != BackendLibreOffice {
		t.Fatalf("unexpected chain: %#v", chain)
	}
}

func TestResolveBackendChainInvalidMode(t *testing.T) {
	t.Parallel()
	if _, err := resolveBackendChain("invalid", "windows"); err == nil {
		t.Fatalf("expected error for invalid backend mode")
	}
}

func TestRunChainFallbackAndSuccess(t *testing.T) {
	t.Parallel()
	out, err := runChain(context.Background(), []Backend{
		fakeCsvBackend{name: "first", err: errors.New("failed")},
		fakeCsvBackend{name: "second", out: []string{"book.sheet-1.csv"}},
	}, "a.xlsx", "out", nil)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(out) != 1 || out[0] != "book.sheet-1.csv" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestRunChainAggregateError(t *testing.T) {
	t.Parallel()
	_, err := runChain(context.Background(), []Backend{
		fakeCsvBackend{name: "first", err: errors.New("failed1")},
		fakeCsvBackend{name: "second", err: errors.New("failed2")},
	}, "a.xlsx", "out", nil)
	if err == nil {
		t.Fatalf("expected aggregate error")
	}
	var agg *backenderr.AggregateError
	if !errors.As(err, &agg) {
		t.Fatalf("expected backend aggregate error, got: %T", err)
	}
	if len(agg.Attempts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(agg.Attempts))
	}
}

func TestOutputNamingSheetIndex(t *testing.T) {
	t.Parallel()
	got := csvOutputPath("/tmp/out", "book.xlsx", 2)
	want := filepath.Join("/tmp/out", "book.sheet-2.csv")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestValidateLibreOfficeSheetsRejectsMultiSheet(t *testing.T) {
	t.Parallel()
	if err := validateLibreOfficeSheets([]int{1, 2}); err == nil {
		t.Fatalf("expected error for multi-sheet libreoffice export")
	}
}

func TestValidateLibreOfficeSheetsRejectsNonFirstSheet(t *testing.T) {
	t.Parallel()
	if err := validateLibreOfficeSheets([]int{2}); err == nil {
		t.Fatalf("expected error for non-first sheet libreoffice export")
	}
}
