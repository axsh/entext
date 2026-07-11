package sheetmap

import "testing"

func TestSanitizeFilename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in  string
		out string
	}{
		{in: "A B", out: "A_B"},
		{in: "A///B", out: "AB"},
		{in: "__A__B__", out: "A_B"},
	}
	for _, tt := range tests {
		if got := SanitizeFilename(tt.in); got != tt.out {
			t.Fatalf("sanitize(%q): got %q, want %q", tt.in, got, tt.out)
		}
	}
}
