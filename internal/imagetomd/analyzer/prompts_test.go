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
