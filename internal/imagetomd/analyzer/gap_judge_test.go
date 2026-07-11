package analyzer

import "testing"

func TestIsSufficientStrictBinary(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "insufficient line", in: "INSUFFICIENT", want: false},
		{name: "decision insufficient", in: "Decision: INSUFFICIENT", want: false},
		{name: "sufficient line", in: "SUFFICIENT", want: true},
		{name: "decision sufficient", in: "Decision:SUFFICIENT", want: true},
		{name: "not sufficient legacy", in: "NOT SUFFICIENT", want: false},
		{name: "ambiguous no token", in: "不足しています", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSufficient(tc.in, GapJudgeStrict); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestIsSufficientCompatInsufficientBeforeSufficientSubstring(t *testing.T) {
	t.Parallel()
	in := "INSUFFICIENT\n補足: 列見出し未取得"
	if IsSufficient(in, GapJudgeCompat) {
		t.Fatalf("INSUFFICIENT must win over embedded SUFFICIENT substring")
	}
}
