package tests

import (
	"testing"
)

func TestTableFaithful_ChangeHistoryGoldenContract(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "01_変更履歴.md",
		[]string{
			"# 変更履歴",
			"| No. | 変更箇所 | 変更内容(変更理由) | Ver | 作成・変更者 | 作成・変更日 | 承認者 | 承認日 | 備考 |",
			"| 43 |",
			"秋葉達也",
			"2025/7/7",
			"28X-REQ-A0220",
			"^/search/event.html(.*)",
			"| 44 |",
			"藤本華子",
			"2025/12/9",
			"28P-ITb1-0243",
			"入れ子構造の展開",
			"SUB_TABLE_P01",
		},
		[]string{
			"要素一覧（Phase",
			"書式・注記・セル結合",
			"意味対応・解釈",
			"図解概要",
			"Q:",
			"A:",
			"### Phase",
		},
	)
}

func TestTableFaithful_ForbiddenExplanatoryHeadings(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "01_変更履歴.md", nil, []string{
		"要素一覧（Phase",
		"書式・注記・セル結合・突合",
		"意味対応・解釈",
		"図解要素",
	})
}
