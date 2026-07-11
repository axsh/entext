package io

import (
	"strings"
	"testing"
)

func TestResolveInputPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    ResolveInputArgs
		wantLen int
		wantErr bool
	}{
		{
			name: "single input from flag",
			args: ResolveInputArgs{
				InputPath: "./a.xlsx",
			},
			wantLen: 1,
		},
		{
			name: "stdin input list",
			args: ResolveInputArgs{
				UseStdin: true,
				Stdin:    strings.NewReader("a.xlsx\n\nb.xlsx\na.xlsx\n"),
			},
			wantLen: 2,
		},
		{
			name: "both input and stdin are invalid",
			args: ResolveInputArgs{
				InputPath: "a.xlsx",
				UseStdin:  true,
				Stdin:     strings.NewReader("b.xlsx\n"),
			},
			wantErr: true,
		},
		{
			name:    "input required",
			args:    ResolveInputArgs{},
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ResolveInputPaths(tc.args)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tc.wantLen {
				t.Fatalf("length mismatch: got=%d want=%d", len(got), tc.wantLen)
			}
		})
	}
}
