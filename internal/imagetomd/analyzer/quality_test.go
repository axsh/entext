package analyzer

import "testing"

func TestIsSufficientCompatRejectsJapaneseNegation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{name: "plain sufficient", in: "SUFFICIENT", want: true},
		{name: "decision sufficient", in: "判定: SUFFICIENT\n補足あり", want: true},
		{name: "japanese negation", in: "SUFFICIENT ではありません", want: false},
		{name: "english negation", in: "NOT SUFFICIENT", want: false},
		{name: "insufficient keyword", in: "INSUFFICIENT", want: false},
		{name: "insufficient with note", in: "判定: INSUFFICIENT\n不足: 行", want: false},
		{name: "fallback ambiguous", in: "不足しています。列見出しが未取得", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSufficient(tc.in, GapJudgeCompat); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestLooksLikeExplanatoryReport(t *testing.T) {
	t.Parallel()
	samples := []struct {
		name string
		in   string
		want bool
	}{
		{name: "phase meta heading", in: "## 要素一覧（Phase 1）\n| 要素ID |", want: true},
		{name: "format interpretation table", in: "## 書式・注記・セル結合・突合", want: true},
		{name: "meaning column", in: "| 意味対応・解釈 |", want: true},
		{name: "faithful table", in: "# 変更履歴\n\n| No. | 変更箇所 |", want: false},
		{name: "nested expansion ok", in: "## 入れ子構造の展開\n### SUB_TABLE_P01", want: false},
	}
	for _, tc := range samples {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeExplanatoryReport(tc.in); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsFinalSynthesisRetryExplanatoryReport(t *testing.T) {
	t.Parallel()
	retry, reason := needsFinalSynthesisRetry("## 要素一覧（Phase 1）\n| 要素ID |")
	if !retry || reason != "explanatory_report" {
		t.Fatalf("got retry=%v reason=%q", retry, reason)
	}
}
