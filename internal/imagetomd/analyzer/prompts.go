package analyzer

import (
	"fmt"
	"strings"
)

const ClassifyPrompt = `この画像の内容を分析し、以下のいずれかのカテゴリーに分類してください。
1. simple_text: 短い文章、シンプルなリストのみで構成されている
2. complex_table: 複雑なテーブル、大規模な一覧表、項目が密集したデータ
3. diagram: フローチャート、泳ぎ車線図、構造図などの図解が主体
4. mixed: 文章と図解（表と図など）が混在している

回答は、カテゴリー名（simple_text, complex_table, diagram, mixed）のみ、またはカテゴリー名を含む短い一文で答えてください。`

const SimpleTextPrompt = `この画像の内容を全てMarkdownに変換してください。テーブルはMarkdownテーブル形式で全列・全行を省略せずに出力してください。要約や整理は禁止です。画像内のテキストを忠実にデジタル化してください。`

const ExecutionQuestionSuffix = `

**CRITICAL: DO NOT PLAN OR EXPLAIN. OUTPUT THE REQUESTED DATA (TABLES/LISTS) IMMEDIATELY. NO PREAMBLE. NO 'I WILL LOOK'. NO 'UNDERSTOOD'. JUST DATA.**`

type Phase struct {
	Num       int
	Name      string
	Goal      string
	MaxRounds int
}

var DefaultPhases = []Phase{
	{
		Num:       1,
		Name:      "全体概要の把握",
		Goal:      "図の種類と全体像を特定する。テーブルが含まれる場合は、列数・行数・列名を正確に読み取り、テーブル全体の規模を把握する。図解要素がある場合はその種類と概要を記録する。",
		MaxRounds: 5,
	},
	{
		Num:       2,
		Name:      "データの網羅的な読み取り",
		Goal:      "テーブルの場合は、全行・全列のデータを1つも漏らさず読み取り、表形式で記録する。図解の場合は、含まれるテキスト・ラベル・接続関係をすべて抽出する。読み取りの正確性を最優先とし、要約や解釈は行わない。",
		MaxRounds: 5,
	},
	{
		Num:       3,
		Name:      "構造関係の解析",
		Goal:      "要素間の関係性（順序、依存、階層）を解析する。テーブルの場合は、セクション帯による行のグルーピング、親子関係、入れ子構造を定義する。図の場合はフロー・接続関係を定義する。",
		MaxRounds: 5,
	},
	{
		Num:       4,
		Name:      "暗黙知の言語化",
		Goal:      "視覚的な配置や書式（色、注記、結合）が暗黙的に示す情報を抽出する。テーブルの場合は、セルの背景色、文字色（赤字・青字）、注記記号（★等）の意味を記録する。不足しているデータ行がないか最終確認する。",
		MaxRounds: 5,
	},
}

func AttachedImageLine(absPath string) string {
	return fmt.Sprintf("\n\n[Attached image: %s]", absPath)
}

func AssessGapPrompt(phase Phase, knownInfo string) string {
	return fmt.Sprintf(`現在は画像解析の Phase %d: [%s] を実施しています。
この Phase の目的は「%s」です。

これまでに対話で判明している情報は以下の通りです：
---
%s
---

上記の情報を元に、Phase の目的を達成するのに十分な情報が集まったか判定してください。
もし不足している情報がある場合は、何が不足しているかを具体的に述べてください。
十分な情報が集まった場合は、回答の中に必ず "SUFFICIENT" という単語を含めてください。
回答は簡潔に行い、前置き（「はい、承知しました」等）は不要です。`,
		phase.Num,
		phase.Name,
		phase.Goal,
		knownInfo,
	)
}

func GenerateQuestionPrompt(phase Phase, gapAssessment string) string {
	return fmt.Sprintf(`画像解析の Phase %d: [%s] において、以下の不足情報が指摘されました：
---
%s
---

この不足を解消し、Phase の目的（%s）を達成するために、画像の内容についてエージェントに問いかける具体的な質問を1つ生成してください。

**質問生成のルール:**
1. 質問は、回答者が「前置きや計画を一切述べず、即座にデータ（テーブルやリスト）を回答する」ことを極めて強く命じる形式にしてください。
2. **テーブル解析の特記事項**: テーブル内に入れ子構造や詳細なリストが含まれるセルを発見した場合は、その存在を明記し、後に詳細を記述するためのプレースホルダー（例: [SUB_TABLE_P01]）を置いてください。
3. 回答の開始文字を指定してください。例：「『| 要素ID |』から書き始めてください」や「『- 要素1:』から書き始めてください」。
4. 「承知しました」「確認します」などの会話的な応答を一切禁止することを明記してください。
5. 外部ツール（OCR, tesseract, shell等）の使用を禁止し、自身の視覚能力（Vision）のみで即答するよう指示に含めてください。
6. 質問文のみを出力してください。`,
		phase.Num,
		phase.Name,
		gapAssessment,
		phase.Goal,
	)
}

func GenerateMarkdownPrompt(phaseLogs []PhaseLog) string {
	var b strings.Builder
	b.WriteString("これまでの各 Phase での解析結果を以下にまとめます：\n\n")
	for _, p := range phaseLogs {
		b.WriteString(fmt.Sprintf("### Phase %d: %s\n", p.PhaseNum, p.PhaseName))
		for _, r := range p.Rounds {
			b.WriteString(fmt.Sprintf("Q: %s\n", r.Question))
			b.WriteString(fmt.Sprintf("A: %s\n\n", r.Answer))
		}
	}
	b.WriteString(`以上の分析結果をもとに、この画像の内容を構造化された Markdown ドキュメントとして生成してください。

**最重要制約: データの忠実な再現 (要約禁止)**
- 分析フェーズで読み取った全てのデータを、1つも省略・要約せずに出力してください。
- テーブルが含まれる場合は、元の列構成（全列）と全行を忠実に Markdown テーブルとして再現してください。
- 「整理」「要約」「まとめ」を行い行や列を減らすことは厳禁です。
- Phase 2 で読み取った行データ（R01, R02, ...等）が最終出力に全て含まれているか自己チェックしてください。

以下の要件を厳守してください：
1. **テーブルの完全再現**: テーブルが含まれる場合は、全列・全行を含む Markdown テーブルを出力の中心に配置してください。セクション帯による区切りは、テーブル行の間に見出し行として挿入してください。
2. **入れ子構造の展開**: セル内に詳細なリストやサブテーブルがある場合は、親テーブルの該当セルにアンカーリンク（例: [詳細](#SUB_TABLE_P01)）を配置し、別セクションとして展開してください。
3. **図解の説明**: フローチャートや構造図が含まれる場合は、Mermaid 記法 (graph TD 等) で視覚的に再現し、かつその図が表す内容を説明する文章を必ず添えてください。Mermaid図だけでは検索や機械処理ができないため、説明文は必須です。
4. **書式情報の反映**: 赤字（注意・重要）、青字（参照・手順リンク）、★注記などの書式情報は、テーブル内の該当セルまたは凡例セクションで明示してください。
5. **見出しとセクション**: 適切な見出し（#、##）を使用して情報を整理してください。
6. 言語は日本語で出力する。`)
	return b.String()
}

func GenerateMarkdownRetryPrompt(answerCorpus string) string {
	return `最終Markdownの統合結果が不十分でした。以下の抽出データのみを情報源として、画像内容を最終Markdownに再構成してください。

[抽出データ]
` + answerCorpus + `

厳守事項:
- 最終成果物は「画像内容のMarkdown化」であり、Phaseレポートではありません。
- "Phase", "Q:", "A:", "round", "分析ログ" などの表現は出力に含めないでください。
- テーブルデータは列・行を省略せず、可能な限り完全再現してください。
- 要約中心ではなく、元データの忠実な再構成を優先してください。
- 回答はMarkdown本文のみを返してください。`
}
