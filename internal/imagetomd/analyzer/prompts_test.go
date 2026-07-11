package analyzer

import (
	"strings"
	"testing"
)

func TestGenerateMarkdownPromptContainsTableFaithfulConstraints(t *testing.T) {
	t.Parallel()
	prompt := GenerateMarkdownPrompt(nil)
	must := []string{
		"原表",
		"解析メタ表",
		"要素一覧（Phase",
		"書式・注記・セル結合",
		"意味対応・解釈",
		"入れ子構造の展開",
	}
	for _, token := range must {
		if !strings.Contains(prompt, token) {
			t.Fatalf("missing constraint token %q", token)
		}
	}
}

func TestGenerateMarkdownRetryPromptContainsTableFaithfulConstraints(t *testing.T) {
	t.Parallel()
	prompt := GenerateMarkdownRetryPrompt("sample corpus")
	must := []string{
		"原表",
		"要素一覧（Phase",
		"入れ子構造の展開",
	}
	for _, token := range must {
		if !strings.Contains(prompt, token) {
			t.Fatalf("missing retry constraint token %q", token)
		}
	}
}

func TestPhase2ExecuteHintContainsColumnHeader(t *testing.T) {
	t.Parallel()
	hint := Phase2ExecuteHint()
	if !strings.Contains(hint, "| No. | 変更箇所 |") {
		t.Fatalf("phase2 hint should mention table header: %s", hint)
	}
}

func TestAssessGapPromptContainsBinaryGapJudgment(t *testing.T) {
	t.Parallel()
	got := AssessGapPrompt(DefaultPhases[1], "known")
	for _, want := range []string{
		"SUFFICIENT",
		"INSUFFICIENT",
		"混在禁止",
		"不足しています",
		"NOT SUFFICIENT",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in AssessGapPrompt", want)
		}
	}
}

func TestAssessGapPromptContainsConversionScopeBoundary(t *testing.T) {
	t.Parallel()
	got := AssessGapPrompt(DefaultPhases[1], "")
	for _, want := range []string{
		"画像から Markdown への変換",
		"校正または評価してはなりません",
		"そのまま転記",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing conversion scope %q", want)
		}
	}
}

func TestAssessGapPromptPhase2ContainsConversionGuide(t *testing.T) {
	t.Parallel()
	got := AssessGapPrompt(DefaultPhases[1], "| No. | ... |")
	if !strings.Contains(got, "表記ゆれ") || !strings.Contains(got, "未取得") {
		t.Fatalf("phase2 conversion guide missing: %s", got)
	}
}

func TestPhase2GoalRequiresTranscriptionFidelityNotProofreading(t *testing.T) {
	t.Parallel()
	goal := DefaultPhases[1].Goal
	if !strings.Contains(goal, "校正") || !strings.Contains(goal, "意味的整合性") {
		t.Fatalf("phase2 goal missing conversion-scope wording: %s", goal)
	}
}

func TestGenerateQuestionPromptForbidsContentValidation(t *testing.T) {
	t.Parallel()
	got := GenerateQuestionPrompt(DefaultPhases[1], "INSUFFICIENT\n未取得: 行3")
	for _, want := range []string{
		"含めてはならない",
		"SymbolEvidence",
		"表記ゆれの統一",
		"未取得データ",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing prohibition rule %q", want)
		}
	}
}

func TestPhase2ConversionGapGuide_LimitsSufficientToVisibleScope(t *testing.T) {
	t.Parallel()
	got := AssessGapPrompt(DefaultPhases[1], "")
	for _, want := range []string{
		"画像可視スコープ内",
		"CSV フルシート",
		"可視スコープ外",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in phase2 gap guide", want)
		}
	}
}

func TestGenerateMarkdownPrompt_ExcludesOffImageRows(t *testing.T) {
	t.Parallel()
	got := GenerateMarkdownPrompt(nil)
	for _, want := range []string{
		"画像可視スコープ外",
		"可視スコープは Phase 1",
		"画像可視スコープ内",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in final synthesis prompt", want)
		}
	}
}

func TestAssessGapPromptDoesNotMentionCsvHint(t *testing.T) {
	t.Parallel()
	got := AssessGapPrompt(DefaultPhases[1], "INSUFFICIENT")
	for _, forbidden := range []string{"[Reference csv hint]", "CSV ヒント"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("assess prompt must not mention csv hint, found %q", forbidden)
		}
	}
}

func TestGenerateQuestionPromptDoesNotMentionCsvHint(t *testing.T) {
	t.Parallel()
	got := GenerateQuestionPrompt(DefaultPhases[1], "INSUFFICIENT")
	for _, forbidden := range []string{"[Reference csv hint]", "CSV ヒント"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("question prompt must not mention csv hint, found %q", forbidden)
		}
	}
}
