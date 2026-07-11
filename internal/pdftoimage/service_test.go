package pdftoimage

import (
	"context"
	"errors"
	"testing"

	backenderr "github.com/axsh/entext/internal/common/backend"
)

type fakePDFBackend struct {
	name string
	err  error
	out  []string
}

func (f fakePDFBackend) Name() string {
	return f.name
}

func (f fakePDFBackend) Convert(_ context.Context, _ string, _ string, _ string) ([]string, error) {
	return f.out, f.err
}

func TestResolveBackendChainAuto(t *testing.T) {
	t.Parallel()
	chain, err := resolveBackendChain(BackendAuto)
	if err != nil {
		t.Fatalf("resolveBackendChain returned error: %v", err)
	}
	if len(chain) != 2 {
		t.Fatalf("expected 2 backends, got %d", len(chain))
	}
	if chain[0].Name() != BackendPDFToPPM {
		t.Fatalf("expected first backend pdftoppm, got %s", chain[0].Name())
	}
	if chain[1].Name() != BackendMagick {
		t.Fatalf("expected second backend magick, got %s", chain[1].Name())
	}
}

func TestResolveBackendChainSpecificMode(t *testing.T) {
	t.Parallel()
	chain, err := resolveBackendChain(BackendMagick)
	if err != nil {
		t.Fatalf("resolveBackendChain returned error: %v", err)
	}
	if len(chain) != 1 || chain[0].Name() != BackendMagick {
		t.Fatalf("unexpected chain: %#v", chain)
	}
}

func TestResolveBackendChainInvalidMode(t *testing.T) {
	t.Parallel()
	if _, err := resolveBackendChain("invalid"); err == nil {
		t.Fatalf("expected error for invalid backend mode")
	}
}

func TestRunChainFallbackAndSuccess(t *testing.T) {
	t.Parallel()
	out, err := runChain(context.Background(), []Backend{
		fakePDFBackend{name: "first", err: errors.New("failed")},
		fakePDFBackend{name: "second", out: []string{"a.png"}},
	}, "a.pdf", "out", "png")
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if len(out) != 1 || out[0] != "a.png" {
		t.Fatalf("unexpected output: %#v", out)
	}
}

func TestRunChainAggregateError(t *testing.T) {
	t.Parallel()
	_, err := runChain(context.Background(), []Backend{
		fakePDFBackend{name: "first", err: errors.New("failed1")},
		fakePDFBackend{name: "second", err: errors.New("failed2")},
	}, "a.pdf", "out", "png")
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
