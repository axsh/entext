package analyzer

import "testing"

func TestIsSufficientCompat(t *testing.T) {
	t.Parallel()
	if !IsSufficient("NOT SUFFICIENT", GapJudgeCompat) {
		t.Fatalf("compat mode should match legacy behavior")
	}
}

func TestIsSufficientStrict(t *testing.T) {
	t.Parallel()
	if IsSufficient("NOT SUFFICIENT", GapJudgeStrict) {
		t.Fatalf("strict mode must reject NOT SUFFICIENT")
	}
	if !IsSufficient("SUFFICIENT", GapJudgeStrict) {
		t.Fatalf("strict mode should accept SUFFICIENT line")
	}
	if !IsSufficient("Decision:SUFFICIENT", GapJudgeStrict) {
		t.Fatalf("strict mode should accept Decision:SUFFICIENT")
	}
}
