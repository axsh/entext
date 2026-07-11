package vaultcli

import "testing"

func TestBuildPath(t *testing.T) {
	t.Parallel()
	got := BuildPath("openai", "default")
	if got != "providers/openai/default" {
		t.Fatalf("unexpected path: %s", got)
	}
}

func TestSetKeyValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   SetInput
		wantErr string
	}{
		{
			name: "invalid provider",
			input: SetInput{
				Provider: "x",
				Name:     "default",
				Secret:   "abc",
			},
			wantErr: "provider must be openai or anthropic",
		},
		{
			name: "missing name",
			input: SetInput{
				Provider: "openai",
				Name:     "",
				Secret:   "abc",
			},
			wantErr: "name is required",
		},
		{
			name: "missing secret",
			input: SetInput{
				Provider: "openai",
				Name:     "default",
				Secret:   "",
			},
			wantErr: "secret is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SetKey(t.Context(), tt.input)
			if err == nil {
				t.Fatalf("expected error")
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("unexpected error: got %q want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
