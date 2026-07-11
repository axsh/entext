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

func TestLooksLikePhaseReport(t *testing.T) {
	t.Parallel()

	if !looksLikePhaseReport("### Phase 1: Overview\nQ: aaa\nA: bbb") {
		t.Fatalf("phase report should be detected")
	}
	if looksLikePhaseReport("# Final Markdown\n| a | b |") {
		t.Fatalf("normal markdown should not be detected as phase report")
	}
}
