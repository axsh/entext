package tern

import (
	"context"
	"errors"
	"testing"
)

func TestResolveMode(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   Mode
		want    Mode
		wantErr bool
	}{
		{name: "empty defaults to auto", input: "", want: ModeAuto},
		{name: "auto", input: ModeAuto, want: ModeAuto},
		{name: "external", input: ModeExternal, want: ModeExternal},
		{name: "inproc", input: ModeInProc, want: ModeInProc},
		{name: "invalid", input: "invalid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected mode: got %s want %s", got, tt.want)
			}
		})
	}
}

func TestBuildRuntimeExternalConnectionFailure(t *testing.T) {
	t.Parallel()
	_, err := BuildRuntime(context.Background(), RuntimeRequest{
		Mode:           ModeExternal,
		ExternalServer: "http://127.0.0.1:1",
	})
	if err == nil {
		t.Fatalf("expected external connection error")
	}
	if !errors.Is(err, ErrConnectFailed) {
		t.Fatalf("expected ErrConnectFailed, got %v", err)
	}
}

func TestBuildRuntimeInProcMissingConfig(t *testing.T) {
	t.Parallel()
	_, err := BuildRuntime(context.Background(), RuntimeRequest{
		Mode:           ModeInProc,
		ConfigPath:     "not-found-tern-config.yaml",
		ExternalServer: "http://127.0.0.1:1",
	})
	if err == nil {
		t.Fatalf("expected inproc config error")
	}
	if !errors.Is(err, ErrConfigMissing) {
		t.Fatalf("expected ErrConfigMissing, got %v", err)
	}
}

func TestRuntimeShutdownIdempotent(t *testing.T) {
	t.Parallel()
	calls := 0
	rt := &Runtime{
		shutdownFn: func(context.Context) error {
			calls++
			return nil
		},
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := rt.Shutdown(context.Background()); err != nil {
		t.Fatalf("unexpected second error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("shutdown should be called once, got %d", calls)
	}
}
