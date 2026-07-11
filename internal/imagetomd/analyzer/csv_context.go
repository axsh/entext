package analyzer

import (
	"path/filepath"
	"strings"

	"github.com/axsh/entext/internal/imagetomd/csvhint"
)

func buildCsvHintContext(hints []csvhint.CsvHint) string {
	if len(hints) == 0 {
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
		b.WriteString("   - 画像に大量の行・列データが含まれる場合: この CSV の該当セル値を参照して転記してよい。\n")
		b.WriteString("     Vision で全セルを逐一再読する必要はない。\n")
		b.WriteString("3. CSV と画像の文字列が異なる場合: 画像で判読できる範囲を優先する。判読不能・曖昧なセルのみ CSV を参照する。\n")
		b.WriteString("4. CSV および画像の内容について、校正・意味整合性・URL/正規表現の妥当性の検証は行わない。\n\n")
		b.WriteString("--- CSV content ---\n")
		b.WriteString(hint.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func phase2CsvExecuteAppend(hints []csvhint.CsvHint) string {
	if len(hints) == 0 {
		return ""
	}
	return "\n\nPhase 2 追加指示:\n" +
		"- 原表の列構成・空欄行・入れ子の有無は画像で確認すること。\n" +
		"- データ行のセル文字列は、行数が多い場合は上記 CSV から転記してよい。\n" +
		"- 最終回答は Markdown テーブル形式とし、CSV をそのまま貼り付けるのではなく、\n" +
		"  画像の表構造に合わせて配置すること。\n"
}

func csvFinalSynthesisAppend(hints []csvhint.CsvHint) string {
	if len(hints) == 0 {
		return ""
	}
	return "\n\n- Phase 2 で CSV 参照により取得したセル値は、画像の原表構造に従って Markdown テーブルへ配置すること。\n" +
		"- CSV にしか存在しない列・行を追加してはならない（画像の表構造を超えない）。\n"
}
