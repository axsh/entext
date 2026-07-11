//go:build windows

package exceltopdf

import "testing"

func TestResolveTargetSheetIndices(t *testing.T) {
	t.Parallel()
	got, err := resolveTargetSheetIndices(3, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("unexpected all-sheet selection: %#v", got)
	}

	got, err = resolveTargetSheetIndices(5, []int{1, 3, 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("unexpected subset selection: %#v", got)
	}

	if _, err := resolveTargetSheetIndices(2, []int{3}); err == nil {
		t.Fatalf("expected out-of-range error")
	}
}

func TestExpandPageSheetNames(t *testing.T) {
	t.Parallel()
	base := []string{"A"}
	got := expandPageSheetNames(base, "B", 3)
	if len(got) != 4 {
		t.Fatalf("unexpected len: %d", len(got))
	}
	if got[1] != "B" || got[2] != "B" || got[3] != "B" {
		t.Fatalf("unexpected expansion: %#v", got)
	}
}
