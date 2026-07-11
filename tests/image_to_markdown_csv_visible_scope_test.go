package tests

import (
	"testing"
)

func TestCsvVisibleScope_ChangeHistoryGoldenForbidsEarlyRows(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "01_変更履歴.md",
		[]string{"| 43 |", "| 44 |", "秋葉達也", "藤本華子"},
		[]string{"| 1 |  | 20A向け", "| 42 |"},
	)
}
