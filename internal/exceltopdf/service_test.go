package exceltopdf

import (
	"context"
	"errors"
	"testing"

	backenderr "github.com/axsh/entext/internal/common/backend"
)

type fakeBackend struct {
	name string
	err  error
	out  string
}

func (f fakeBackend) Name() string {
	return f.name
}

func (f fakeBackend) Convert(_ context.Context, _ string, _ string) (string, error) {
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
		fakeBackend{name: "first", err: errors.New("failed")},
		fakeBackend{name: "second", out: "ok.pdf"},
	}, "a.xlsx", "out")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if out != "ok.pdf" {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestRunChainAggregateError(t *testing.T) {
	t.Parallel()
	_, err := runChain(context.Background(), []Backend{
		fakeBackend{name: "first", err: errors.New("failed1")},
		fakeBackend{name: "second", err: errors.New("failed2")},
	}, "a.xlsx", "out")
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
