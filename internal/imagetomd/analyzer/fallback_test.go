package analyzer

import (
	"strings"
	"testing"
)

func TestBuildAnswerCorpusKeepsAllNonEmptyAnswers(t *testing.T) {
	t.Parallel()

	logs := []PhaseLog{
		{
			PhaseNum:  1,
			PhaseName: "Overview",
			Rounds: []RoundLog{
				{Answer: "  "},
				{Answer: "画像を読み取り、指定フォーマットで出力します。"},
				{Answer: "| col |\n| --- |\n| v1 |"},
			},
		},
		{
			PhaseNum:  2,
			PhaseName: "Exhaustive Data",
			Rounds: []RoundLog{
				{Answer: "line-a\nline-b"},
			},
		},
	}

	got := buildAnswerCorpus(logs)
	if got == "" {
		t.Fatalf("answer corpus should not be empty")
	}
	if !strings.Contains(got, "## Phase Overview") {
		t.Fatalf("missing phase header: %s", got)
	}
	if !strings.Contains(got, "| col |") {
		t.Fatalf("missing non-empty answer content: %s", got)
	}
	if !strings.Contains(got, "指定フォーマットで出力します") {
		t.Fatalf("non-empty answers should be preserved: %s", got)
	}
	if !strings.Contains(got, "## Phase Exhaustive Data") {
		t.Fatalf("missing second phase header: %s", got)
	}
}

func TestBuildAnswerCorpusReturnsEmptyWhenNoUsefulAnswers(t *testing.T) {
	t.Parallel()

	logs := []PhaseLog{
		{
			PhaseNum:  1,
			PhaseName: "Overview",
			Rounds: []RoundLog{
				{Answer: "   "},
				{Answer: "   "},
			},
		},
	}

	if got := buildAnswerCorpus(logs); got != "" {
		t.Fatalf("answer corpus should be empty, got: %s", got)
	}
}

func TestNeedsFinalSynthesisRetryReasons(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		in     string
		retry  bool
		reason string
	}{
		{name: "empty", in: "   ", retry: true, reason: "empty"},
		{name: "phase report", in: "### Phase 1\nQ: q\nA: a", retry: true, reason: "phase_report"},
		{name: "explanatory report", in: "## 要素一覧（Phase 1）\n| 要素ID |", retry: true, reason: "explanatory_report"},
		{name: "faithful table", in: "# 変更履歴\n\n| No. | 変更箇所 |", retry: false, reason: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			retry, reason := needsFinalSynthesisRetry(tc.in)
			if retry != tc.retry || reason != tc.reason {
				t.Fatalf("got retry=%v reason=%q want retry=%v reason=%q", retry, reason, tc.retry, tc.reason)
			}
		})
	}
}
