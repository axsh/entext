package analyzer

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/axsh/entext/internal/imagetomd/csvhint"
)

type CsvInjectMode int

const (
	CsvInjectNone CsvInjectMode = iota
	CsvInjectFullContent
	CsvInjectPathOnly
	CsvInjectSynthesisOnly
)

const csvScopePriorityBlock = `【最優先: 出力スコープ】
- 出力 Markdown に含めてよいデータ行は、画像可視スコープ内の行のみ。
- CSV にあっても画像に写っていない行（例: 可視 No. が 43,44 のみなのに No.1〜42）は追加禁止。
- CSV は可視行のセル文字列判読補完にのみ使用する。
`

func buildCsvHintContext(hints []csvhint.CsvHint, mode CsvInjectMode, scope PhaseVisibleScope) string {
	if len(hints) == 0 || mode == CsvInjectNone {
		return ""
	}
	var b strings.Builder
	for _, hint := range hints {
		absPath, err := filepath.Abs(hint.Path)
		if err != nil {
			absPath = hint.Path
		}
		absPath = filepath.ToSlash(absPath)
		b.WriteString("\n\n[Reference csv hint]\n")
		b.WriteString(csvScopePriorityBlock)
		b.WriteString(buildScopeMetadataBlock(scope, hint))
		b.WriteString("Source: ")
		b.WriteString(absPath)
		b.WriteString("\n\n")
		b.WriteString("【CSV ヒントの位置づけ】\n")
		b.WriteString("- この CSV は元 Excel から抽出したセル値テキストである。\n")
		b.WriteString("- CSV には図表・色・結合レイアウト・入れ子の視覚構造は含まれない。\n\n")
		b.WriteString("【画像と CSV の使い分け（必ず守ること）】\n")
		b.WriteString("1. 表の構造（列見出し、行数、空欄行、セクション帯、結合・入れ子の有無）は、添付画像の Vision 読取を正とする。\n")
		b.WriteString("2. セル内の文字列データについて:\n")
		b.WriteString("   - 画像から判読可能で量が少ない場合: Vision 転記を優先する。\n")
		b.WriteString("   - 画像可視スコープ内に大量のセル文字列がある場合のみ CSV 参照可。可視スコープ外の行は転記禁止。\n")
		b.WriteString("     Vision で全セルを逐一再読する必要はない。\n")
		b.WriteString("3. CSV と画像の文字列が異なる場合: 画像で判読できる範囲を優先する。判読不能・曖昧なセルのみ CSV を参照する。\n")
		b.WriteString("4. CSV および画像の内容について、校正・意味整合性・URL/正規表現の妥当性の検証は行わない。\n\n")
		if mode == CsvInjectFullContent {
			b.WriteString("--- CSV content ---\n")
			b.WriteString(selectCsvExcerpt(hint, scope))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func buildScopeMetadataBlock(scope PhaseVisibleScope, hint csvhint.CsvHint) string {
	if scope.IsEmpty() && hint.SheetIndex == 0 && hint.SheetName == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("【画像可視スコープ（CSV 参照境界）】\n")
	if len(scope.VisibleRowIDs) > 0 {
		b.WriteString("- 可視データ行 No.: ")
		b.WriteString(strings.Join(scope.VisibleRowIDs, ", "))
		b.WriteString("\n")
	}
	if scope.ColumnCount > 0 || len(scope.ColumnNames) > 0 {
		b.WriteString("- 列数: ")
		if len(scope.ColumnNames) > 0 {
			b.WriteString(strings.Join(scope.ColumnNames, ", "))
		} else {
			b.WriteString("(see Phase 1)")
		}
		b.WriteString("\n")
	}
	if hint.SheetName != "" || hint.SheetIndex > 0 {
		b.WriteString("- CSV シート: ")
		if hint.SheetName != "" {
			b.WriteString(hint.SheetName)
		}
		if hint.SheetIndex > 0 {
			if hint.SheetName != "" {
				b.WriteString(" ")
			}
			b.WriteString("(sheet_index=")
			b.WriteString(strconv.Itoa(hint.SheetIndex))
			b.WriteString(")")
		}
		b.WriteString("\n")
	}
	if scope.VisibleBlankRows > 0 {
		b.WriteString("- 可視空欄行: ")
		b.WriteString(strconv.Itoa(scope.VisibleBlankRows))
		b.WriteString("\n")
	}
	if len(scope.NestedPlaceholders) > 0 {
		b.WriteString("- 入れ子: ")
		b.WriteString(strings.Join(scope.NestedPlaceholders, ", "))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
}

func selectCsvExcerpt(hint csvhint.CsvHint, scope PhaseVisibleScope) string {
	return csvhint.FilterCsvByScope(hint.Content, scope.VisibleRowIDs, csvhint.MaxCsvInjectLines)
}

func phase2CsvExecuteAppend(hints []csvhint.CsvHint, scope PhaseVisibleScope) string {
	if len(hints) == 0 {
		return ""
	}
	_ = scope
	return "\n\nPhase 2 追加指示:\n" +
		csvScopePriorityBlock +
		"- 原表の列構成・空欄行・入れ子の有無は画像で確認すること。\n" +
		"- データ行のセル文字列は、画像可視スコープ内の行についてのみ CSV から転記してよい。\n" +
		"- 可視スコープ外の行（CSV に存在しても画像に無い行）は出力に含めないこと。\n" +
		"- 最終回答は Markdown テーブル形式とし、CSV をそのまま貼り付けるのではなく、\n" +
		"  画像の表構造に合わせて配置すること。\n"
}

func csvFinalSynthesisAppend(hints []csvhint.CsvHint, scope PhaseVisibleScope) string {
	if len(hints) == 0 {
		return ""
	}
	_ = scope
	return "\n\n" + csvScopePriorityBlock +
		"- Phase 2 で CSV 参照により取得したセル値は、画像の原表構造に従って Markdown テーブルへ配置すること。\n" +
		"- CSV にしか存在しない列・行を追加してはならない（画像の可視スコープを超えない）。\n" +
		"- Phase 2 answer に可視スコープ外の行があっても、最終 MD には画像可視行のみを出力すること。\n"
}
