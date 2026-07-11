package tests

import "testing"

func TestConversionScope_ChangeHistoryGoldenForbidsProofreadingArtifacts(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "01_変更履歴.md", nil, []string{
		"SymbolEvidence",
		"文字差異注記",
		"内容整合性",
		"意味対応・解釈",
	})
}
