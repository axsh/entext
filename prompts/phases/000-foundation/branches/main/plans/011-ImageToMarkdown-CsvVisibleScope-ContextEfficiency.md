# 011-ImageToMarkdown-CsvVisibleScope-ContextEfficiency

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/012-ImageToMarkdown-CsvVisibleScope-ContextEfficiency.md`

## Goal Description

`011-ImageToMarkdown-ExcelCsvHint` で導入した CSV ヒントについて、**出力境界を画像可視スコープに限定**し、CSV はその範囲内のセル値補完にのみ使うよう修正する。併せて **CSV 付与タイミングの縮小**、**Phase 間サマリ引き継ぎ**、**AssessGap 用 known_info 短縮**、**SessionLog 圧縮**、**sheet-map 連携 CSV 自動解決** を実装し、部分スクリーンショット + フルシート CSV による情報過剰出力（例: `01_変更履歴.png` → No.1〜44 出力）を防ぐ。

## User Review Required

1. **CSV 行フィルタ失敗時のフォールバック**: Phase 1 から可視行 ID を抽出できなかった場合、CSV 本文注入は **行フィルタなし + 最優先出力禁止制約プロンプト**（仕様 A4-b）にフォールバックする。Go 側フィルタ成功時のみ scoped excerpt を注入する方針で確定してよいか。
2. **`MaxCsvInjectLines = 200`**: 注入 CSV 本文の行数上限。超過時は truncate + `... (truncated, see Source path)` 注記。閾値 200 でよいか。
3. **sheet-map 探索パス**: 画像 `{imagesDir}/01_変更履歴.png` に対し、`{imagesDir}/../pdf/*.sheet-map.json` → `{imagesDir}/../*.sheet-map.json` → `{imagesDir}/*.sheet-map.json` の順で探索する。運用ディレクトリ `tmp/output/pc/` に合わせたこの優先順でよいか。
4. **SessionLog 後方互換**: `known_info` フィールドは残し、内容を **サマリ文字列** に置き換える。追加で `known_info_summary`（同一内容）と `known_info_chars` を付与。既存 JSON 消費者が全文期待の場合は破壊的変更になりうるが、012 要件 19 に従いこの方針でよいか。
5. **012 任意要件 1（sidecar scope.json）**: 初版は **未実装（先送り）**。Phase 1 LLM 抽出 + regex フォールバックのみ。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1. CSV は可視スコープ超えの行追加禁止 | `csvhint/filter.go`, `csv_context.go` 最優先制約, `GenerateMarkdownPrompt` |
| A2. CSV 転記は可視スコープ内データ行のみ | `FilterCsvByScope`, `phase2CsvExecuteAppend` |
| A3. 可視スコープメタデータをプロンプト明示 | `PhaseVisibleScope`, `buildScopeMetadataBlock` |
| A4. スコープフィルタ (a) 推奨 | `csvhint/filter.go`, `FilterCsvByScope` |
| A5. CSV 最大行数 cap | `csvhint/filter.go` `MaxCsvInjectLines = 200` |
| A6. AssessGap に CSV 非付与（維持） | `analyzer.go` 注入分岐, 既存 `prompts_test.go` |
| B7–B10. プロンプト最優先制約・ギャップ/統合文言 | `csv_context.go`, `prompts.go` |
| C11–C13. CSV 付与タイミング縮小 | `analyzer.go` `csvContextForStep`, `analyzer_test.go` |
| D14–D16. Phase 間サマリ引き継ぎ | `scope.go`, `analyzer.go` phase loop |
| E17–E20. known_info 短縮・ログ圧縮 | `scope.go` `buildPromptKnownInfo`, `session.go`, `analyzer.go` |
| F21–F23. sheet-map CSV 自動解決 | `csvhint/resolver.go`, `resolver_sheetmap.go` |
| G24–G25. CLI/API 一貫性 | `entext.go` 経由の同一 `ResolveCsvHints` |
| 任意1 sidecar scope.json | **先送り** — README/計画に記載のみ |
| 任意2 regex フォールバック抽出 | `scope.go` `extractScopeFromPhase1Answers` |
| 任意3 excel-to-pdf --with-csv | **先送り**（011 任意要件維持） |

## Proposed Changes

### `internal/imagetomd/csvhint`

#### [NEW] `internal/imagetomd/csvhint/filter_test.go`(file://internal/imagetomd/csvhint/filter_test.go)
*   **Description**: CSV スコープフィルタ・行数 cap の RED テスト（TDD 先行）。
*   **Technical Design**:
    *   ```go
        func TestFilterCsvByScope_KeepsOnlyVisibleRowIDs(t *testing.T) {
            full := ",No.,変更箇所\n,1,row1\n,43,row43\n,44,row44\n"
            got := FilterCsvByScope(full, []string{"43", "44"}, MaxCsvInjectLines)
            // must contain 43,44; must not contain ",1," data row
        }
        func TestFilterCsvByScope_EmptyScopeReturnsFullWithCap(t *testing.T) { /* scope nil → full + cap */ }
        func TestTruncateCsvLines_AddsTruncationNote(t *testing.T) { /* 201 lines → 200 + note */ }
        func TestDetectNoColumnIndex(t *testing.T) { /* header row parsing */ }
        ```
*   **Logic**:
    *   変更履歴形式 CSV（先頭空列 + `No.` 列）を fixture として固定。
    *   ボトムアップ Step 1: フィルタ単体 → resolver → analyzer へ。

#### [NEW] `internal/imagetomd/csvhint/filter.go`(file://internal/imagetomd/csvhint/filter.go)
*   **Description**: 可視行 ID に基づく CSV 抜粋と行数 cap。
*   **Technical Design**:
    *   ```go
        const MaxCsvInjectLines = 200

        // FilterCsvByScope returns CSV excerpt containing header + rows whose No. column
        // matches any ID in visibleRowIDs. When visibleRowIDs is empty, returns input
        // (caller must rely on prompt-only scope constraints).
        func FilterCsvByScope(content string, visibleRowIDs []string, maxLines int) string

        func TruncateCsvLines(content string, maxLines int) (string, bool /* truncated */)

        // detectNoColumnIndex scans the first 5 lines for a CSV field equal to "No." (trimmed).
        func detectNoColumnIndex(lines []string) (headerLineIdx, noColIdx int, ok bool)
        ```
*   **Logic**:
    *   CSV を `\n` 分割。ヘッダ行（`,No.` を含む行）を検出し No. 列イン�dex を特定。
    *   データ行について `csv.Parse` 相当の単純 split（引用符エスケープは `encoding/csv` Reader 使用）で No. 値を取得。
    *   `visibleRowIDs` が空でない場合: No. が ID 集合に含まれる行 + ヘッダ行のみ出力。
    *   出力前に `TruncateCsvLines(..., maxLines)` を適用。truncate 時は末尾に `\n... (CSV excerpt truncated; full file at Source path)\n` を付与。
    *   複数シート混在は入力 CSV 1 ファイル前提（011 維持）。

#### [NEW] `internal/imagetomd/csvhint/resolver_sheetmap_test.go`(file://internal/imagetomd/csvhint/resolver_sheetmap_test.go)
*   **Description**: sheet-map から sheet index / CSV パスを解決するテスト。
*   **Technical Design**:
    *   ```go
        func TestFindSheetMapNearImage(t *testing.T) { /* ../pdf/book.sheet-map.json */ }
        func TestResolveSheetIndexFromImageBasename(t *testing.T) {
            // 01_変更履歴.png + sheet-map → sheet_index=1, name=変更履歴
        }
        func TestResolveCsvPathFromSheetMap(t *testing.T) {
            // source_xlsx URL書き換え...xlsm → workbook.sheet-1.csv in ../csv/
        }
        func TestResolveCsvHintsSheetMapAuto(t *testing.T) {
            // no explicit --csv-hint; sheet-map + csv present → hint loaded
        }
        func TestResolveCsvHintsExplicitOverridesSheetMap(t *testing.T)
        ```
*   **Logic**:
    *   `tmp/output/pc/` 相当のディレクトリ構造を `t.TempDir()` で再現。
    *   画像 basename 規則は `pdftoimage.BuildOutputName` と整合: `^(\d{2})_(.+)\.(png|jpe?g|webp)$`。

#### [MODIFY] `internal/imagetomd/csvhint/resolver.go`(file://internal/imagetomd/csvhint/resolver.go)
*   **Description**: sheet-map 連携自動解決を `resolveAutoPath` に追加。
*   **Technical Design**:
    *   ```go
        type CsvHint struct {
            Path       string
            Content    string
            SheetIndex int    // 0 if unknown
            SheetName  string // optional, from sheet-map
        }

        func ResolveCsvHints(explicitPaths []string, imagePath string, disableAuto bool) ([]CsvHint, error)
        // internal:
        func resolveAutoPath(imagePath string) (string, bool)
        func resolveSheetMapCsvPath(imagePath string) (path string, sheetIndex int, sheetName string, ok bool)
        func findSheetMapFile(imagePath string) (string, bool)
        func parseImagePageBasename(imagePath string) (pageIndex int, sheetLabel string, ok bool)
        ```
*   **Logic**:
    1. `explicitPaths` が非空 → 既存どおり読み込み（sheet-map 探索しない）。
    2. `disableAuto` → 空 slice 返却。
    3. 自動解決順:
       - (a) 既存: `{imageDir}/{basename}.csv`, `{imageDir}/csv/{basename}.csv`, `{imageDir}/../csv/{basename}.csv`
       - (b) **新規**: `resolveSheetMapCsvPath`:
         - `findSheetMapFile` で JSON 読込
         - `parseImagePageBasename("01_変更履歴.png")` → pageIndex=0, sheetLabel=`変更履歴`
         - `SheetEntries` から `sheet_index` / `sheet_name` 確定
         - `workbookBase = basename(source_xlsx)` → `{../csv/}{workbookBase}.sheet-{sheet_index}.csv` を候補探索
    4. 最初に `Stat` 成功したパスを採用。`CsvHint` に SheetIndex/SheetName を設定。

### `internal/imagetomd/analyzer`

#### [NEW] `internal/imagetomd/analyzer/scope_test.go`(file://internal/imagetomd/analyzer/scope_test.go)
*   **Description**: Phase 1 回答からの可視スコープ抽出・プロンプト known 短縮の RED テスト。
*   **Technical Design**:
    *   ```go
        func TestExtractVisibleScopeFromPhase1Answer_No43And44(t *testing.T)
        func TestExtractVisibleScopeFromPhase1Answer_VisibleRowCount(t *testing.T)
        func TestExtractVisibleScopeFromPhase1Answer_NestedPlaceholders(t *testing.T)
        func TestExtractVisibleScope_EmptyPhaseLogReturnsEmptyScope(t *testing.T)
        func TestBuildPromptKnownInfo_Round1ScopeOnly(t *testing.T)
        func TestBuildPromptKnownInfo_Round2IncludesLatestDeltaOnly(t *testing.T)
        func TestBuildPromptKnownInfo_NoQuadraticGrowth(t *testing.T) {
            // round 5 known length < round1.answer + round4.answer concatenated
        }
        func TestScopeSummaryTextContainsRowIDs(t *testing.T)
        ```
*   **Logic**:
    *   実セッション JSON から抜粋した Phase 1 round 1 answer 文字列を fixture 化（テストファイル内 const）。

#### [NEW] `internal/imagetomd/analyzer/scope.go`(file://internal/imagetomd/analyzer/scope.go)
*   **Description**: 画像可視スコープモデルと Phase 間引き継ぎ・known 短縮。
*   **Technical Design**:
    *   ```go
        type PhaseVisibleScope struct {
            VisibleRowIDs      []string
            ColumnCount        int
            ColumnNames        []string
            VisibleBlankRows   int
            NestedPlaceholders []string
        }

        func (s PhaseVisibleScope) SummaryText() string
        func (s PhaseVisibleScope) IsEmpty() bool

        // ExtractVisibleScopeFromPhase1 scans all Phase 1 round answers (latest wins for conflicts).
        func ExtractVisibleScopeFromPhase1(phaseLog PhaseLog) PhaseVisibleScope

        // buildPromptKnownInfo builds AssessGap input (short form).
        func buildPromptKnownInfo(scope PhaseVisibleScope, previousAnswer string, roundNum int) string

        const maxLatestAnswerInPrompt = 2000

        func extractRowIDsFromText(text string) []string
        func extractColumnNamesFromText(text string) []string
        func extractPlaceholderIDs(text string) []string
        ```
*   **Logic**:
    *   **Row ID 抽出 regex**（優先順）:
        - `` `No\.(\d+)` `` / `No\.(\d+)` パターン（「取得済みデータ行」「表示データ行は」近傍を優先スキャン）
        - Markdown 表 `| 43 |` 形式（1列目が数字のみ）
        - 重複除去、昇順ソート
    *   **列名抽出**: `全列名` 行または `|` 区切りヘッダ行から split
    *   **空欄行数**: `空欄(\d+)` / `可視11行（ヘッダ1 + データ2 + 空欄8）` パターン
    *   **入れ子**: `\[SUB_TABLE_P\d+\]` を収集
    *   `SummaryText()` 出力例:
        ```text
        [画像可視スコープ]
        - 可視データ行 No.: 43, 44
        - 列数: 9
        - 列名: No., 変更箇所, ...
        - 可視空欄行: 8
        - 入れ子: [SUB_TABLE_P01], [SUB_TABLE_P02]
        - 出力禁止: 上記 No. 以外のデータ行を CSV から追加しないこと
        ```
    *   `buildPromptKnownInfo`:
        - round 1: `scope.SummaryText()` のみ
        - round 2+: `scope.SummaryText()` + `\n\n[直近ラウンド回答]\n` + `truncate(previousAnswer, 2000)`
        - **全文累積禁止**: 過去ラウンド answer の連結を行わない

#### [MODIFY] `internal/imagetomd/analyzer/csv_context_test.go`(file://internal/imagetomd/analyzer/csv_context_test.go)
*   **Description**: CSV コンテキストモード別の契約テスト追加・既存更新。
*   **Logic**:
    *   `TestBuildCsvHintContext_ModeFullIncludesContent`
    *   `TestBuildCsvHintContext_ModePathOnlyOmitsContent`
    *   `TestBuildCsvHintContext_ModeNoneReturnsEmpty`
    *   `TestBuildCsvHintContext_IncludesVisibleScopeBlock`
    *   `TestPhase2CsvExecuteAppend_ForbidsOffImageRows`
    *   `TestCsvFinalSynthesisAppend_NoCsvContent`

#### [MODIFY] `internal/imagetomd/analyzer/csv_context.go`(file://internal/imagetomd/analyzer/csv_context.go)
*   **Description**: CSV 注入モードと可視スコープ最優先制約。
*   **Technical Design**:
    *   ```go
        type CsvInjectMode int
        const (
            CsvInjectNone CsvInjectMode = iota
            CsvInjectFullContent   // Phase 2 execute round 1: scoped excerpt
            CsvInjectPathOnly      // Phase 2 execute round 2+
            CsvInjectSynthesisOnly // final: policy text only (no --- CSV content ---)
        )

        func buildCsvHintContext(hints []csvhint.CsvHint, mode CsvInjectMode, scope PhaseVisibleScope) string

        func buildScopeMetadataBlock(scope PhaseVisibleScope, hint csvhint.CsvHint) string

        func phase2CsvExecuteAppend(hints []csvhint.CsvHint, scope PhaseVisibleScope) string

        func csvFinalSynthesisAppend(hints []csvhint.CsvHint, scope PhaseVisibleScope) string

        func selectCsvExcerpt(hint csvhint.CsvHint, scope PhaseVisibleScope) string
        ```
*   **Logic**:
    *   **最優先制約**（`buildCsvHintContext` / `phase2CsvExecuteAppend` / `csvFinalSynthesisAppend` 共通先頭）:
        ```text
        【最優先: 出力スコープ】
        - 出力 Markdown に含めてよいデータ行は、画像可視スコープ内の行のみ。
        - CSV にあっても画像に写っていない行（例: 可視 No. が 43,44 のみなのに No.1〜42）は追加禁止。
        - CSV は可視行のセル文字列判読補完にのみ使用する。
        ```
    *   `CsvInjectFullContent`: ポリシー + `buildScopeMetadataBlock` + `--- CSV content ---` + `selectCsvExcerpt(...)`
    *   `selectCsvExcerpt`: `FilterCsvByScope(hint.Content, scope.VisibleRowIDs, MaxCsvInjectLines)` — scope 空なら truncate のみ
    *   `CsvInjectPathOnly`: ポリシー + `Source: {path}` + scope メタのみ（`--- CSV content ---` なし）
    *   `CsvInjectSynthesisOnly`: `csvFinalSynthesisAppend` 相当の箇条書きのみ
    *   **011 文言修正**:
        - 旧: 「画像に大量の行・列データが含まれる場合: CSV 転記可」
        - 新: 「**画像可視スコープ内**に大量のセル文字列がある場合のみ CSV 参照可。可視スコープ外の行は転記禁止。」

#### [MODIFY] `internal/imagetomd/analyzer/prompts.go`(file://internal/imagetomd/analyzer/prompts.go)
*   **Description**: ギャップ判定・最終統合プロンプトを可視スコープ整合。
*   **Logic**:
    *   `phase2ConversionGapGuide` 修正:
        ```go
        const phase2ConversionGapGuide = `
        Phase 2 追加ガイド:
        - 複数ラウンドの回答間の字形差だけを理由に INSUFFICIENT にしない。
        - **画像可視スコープ内**の列見出し・データ行・空欄行・入れ子が取得済みなら SUFFICIENT とする。
        - CSV フルシートの行数が揃っただけでは SUFFICIENT にしない（可視スコープ外の行は不要）。
        - SymbolEvidence、内容整合性、URL・正規表現の妥当性検証を不足理由にしない。`
        ```
    *   `GenerateMarkdownPrompt` 末尾に追加:
        ```text
        - Phase 2 で CSV 参照により取得した行のうち、画像可視スコープ外の行は最終出力から除外すること。
        - 可視スコープは Phase 1 で確定した行（known_info サマリ参照）を正とする。
        - Phase 2 answer に No.1 等の過剰行があっても、最終 MD には可視行のみを出力すること。
        ```
    *   `GenerateMarkdownPrompt` の「Phase 2 で読み取った行データ（R01...）を**全て**含める」文言を「**画像可視スコープ内**の行データを全て含める」に修正。

#### [MODIFY] `internal/imagetomd/analyzer/prompts_test.go`(file://internal/imagetomd/analyzer/prompts_test.go)
*   **Description**: 更新後プロンプト文字列のアサーション。
*   **Logic**:
    *   `TestPhase2ConversionGapGuide_LimitsSufficientToVisibleScope`
    *   `TestGenerateMarkdownPrompt_ExcludesOffImageRows`

#### [MODIFY] `internal/imagetomd/analyzer/session.go`(file://internal/imagetomd/analyzer/session.go)
*   **Description**: SessionLog 圧縮フィールド追加。
*   **Technical Design**:
    *   ```go
        type RoundLog struct {
            KnownInfo         string `json:"known_info,omitempty"`          // summary for audit (not full cumulative)
            KnownInfoSummary  string `json:"known_info_summary,omitempty"` // same as KnownInfo when set
            KnownInfoChars    int    `json:"known_info_chars,omitempty"`
            GapAssessment     string `json:"gap_assessment"`
            Sufficient        bool   `json:"sufficient"`
            Question          string `json:"question"`
            Answer            string `json:"answer"` // full answer retained
        }
        ```
*   **Logic**:
    *   `known_info` に Phase 1..N 全文連結を保存しない。
    *   各 round: `KnownInfo = buildPromptKnownInfo(...)` の送信内容と同一サマリを保存。
    *   `Answer` は従来どおり全文（監査用）。

#### [MODIFY] `internal/imagetomd/analyzer/session_test.go`(file://internal/imagetomd/analyzer/session_test.go)
*   **Description**: 圧縮フィールド JSON シリアライズテスト。
*   **Logic**:
    *   `TestRoundLog_KnownInfoSummaryNotFullCumulative`
    *   `TestSessionLogSave_PreservesKnownInfoChars`

#### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: CSV 注入タイミング・known 短縮の recording client テスト。011 テスト期待値更新。
*   **Logic**:
    *   **REPLACE** `TestAnalyzeInjectsCsvHintOnClassifyAndExecuteNotAssess`:
        - classify: `[Reference csv hint]` **なし**
        - Phase 1 execute: CSV **なし**
        - Phase 2 round 1 execute: `[Reference csv hint]` + `--- CSV content ---` **あり**
        - Phase 2 round 2 execute（MaxRounds>=2 設定）: `--- CSV content ---` **なし**, `Source:` **あり**
        - AssessGap: CSV **なし**
    *   **NEW** `TestAnalyzePhase2KnownInitializesWithVisibleScope`
    *   **NEW** `TestAnalyzeAssessKnownDoesNotGrowQuadratically`
    *   **NEW** `TestAnalyzeClassifyUsesRefOnlyWithoutCsv`

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: 注入タイミング分岐、Phase サマリ引き継ぎ、known 短縮の中核。
*   **Technical Design**:
    *   ```go
        type analyzeContext struct {
            refs           []refresolver.RefDocument
            csvHints       []csvhint.CsvHint
            visibleScope   PhaseVisibleScope
            refContext     string
        }

        func (a *Analyzer) csvContextForStep(step csvStep, ctx analyzeContext, phaseNum, roundNum int) string

        type csvStep int
        const (
            stepClassify csvStep = iota
            stepExecute
            stepFinalSynthesis
        )
        ```
*   **Logic**:
    1. **初期化**:
        ```go
        refContext := buildRefContext(refs)
        // REMOVE: csvContext := buildCsvHintContext(csvHints) at top level
        // REMOVE: visionContext := refContext + csvContext for all steps
        ctx := analyzeContext{refs: refs, csvHints: csvHints, refContext: refContext}
        ```
    2. **classify**:
        ```go
        classResp, err := a.client.SendText(ctx, sessionID,
            ClassifyPrompt + ctx.refContext + AttachedImageLine(absPath))
        ```
    3. **Phase loop 開始時 known 初期化**:
        ```go
        known := ""
        if phase.Num >= 2 && !ctx.visibleScope.IsEmpty() {
            known = ctx.visibleScope.SummaryText()
        }
        var lastAnswer string
        ```
    4. **Phase 1 終了後**:
        ```go
        if phase.Num == 1 {
            ctx.visibleScope = ExtractVisibleScopeFromPhase1(phaseLog)
        }
        ```
    5. **AssessGap**:
        ```go
        promptKnown := buildPromptKnownInfo(ctx.visibleScope, lastAnswer, roundNum)
        assessPrompt := AssessGapPrompt(phase, promptKnown)
        roundLog.KnownInfo = promptKnown
        roundLog.KnownInfoSummary = promptKnown
        roundLog.KnownInfoChars = len(promptKnown)
        ```
    6. **execute visionContext**:
        ```go
        var extra string
        if phase.Num == 2 && len(ctx.csvHints) > 0 {
            mode := CsvInjectPathOnly
            if roundNum == 1 {
                mode = CsvInjectFullContent
            }
            extra = buildCsvHintContext(ctx.csvHints, mode, ctx.visibleScope)
                + phase2CsvExecuteAppend(ctx.csvHints, ctx.visibleScope)
                + Phase2ExecuteHint()
        }
        answerPrompt := question + extra + ExecutionQuestionSuffix + ctx.refContext + AttachedImageLine(absPath)
        ```
    7. **known 更新**（累積廃止）:
        ```go
        lastAnswer = answer
        // DO NOT: known = known + answer
        ```
    8. **final synthesis**:
        ```go
        finalPrompt := GenerateMarkdownPrompt(log.Phases) +
            csvFinalSynthesisAppend(csvHints, ctx.visibleScope)
        // retry も同様。buildCsvHintContext Full は付与しない
        ```

### `tests/` (integration contract)

#### [NEW] `tests/image_to_markdown_csv_visible_scope_test.go`(file://tests/image_to_markdown_csv_visible_scope_test.go)
*   **Description**: 012 シナリオ 1 の決定的契約テスト（LLM 非呼び出し）。
*   **Technical Design**:
    *   ```go
        func TestCsvVisibleScope_ChangeHistoryGoldenForbidsEarlyRows(t *testing.T) {
            assertReferenceMarkdownContract(t, "01_変更履歴.md",
                []string{"| 43 |", "| 44 |", "秋葉達也", "藤本華子"},
                []string{"| 1 |  | 20A向け", "| 42 |"},
            )
        }
        func TestCsvVisibleScope_FilterUnitMatchesGoldenScenario(t *testing.T) {
            // inline CSV fixture + scope {43,44} → excerpt rows count == 2
        }
        ```
*   **Logic**:
    *   golden MD は `tests/testdata/reference_parity/01_変更履歴.md` を使用（No.43/44 のみ含む参照）。
    *   禁止トークン `| 1 |  | 20A向け` は No.1 データ行の存在を検出。

#### [MODIFY] `tests/image_to_markdown_csv_hint_test.go`(file://tests/image_to_markdown_csv_hint_test.go)
*   **Description**: 可視スコープ契約テストへの委譲または forbidden 行追加。

### Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**:
    *   CSV hint 節に「CSV は画像可視行のセル値補完のみ。可視外の行は出力しない」を追記。
    *   sheet-map 連携自動解決（`../pdf/*.sheet-map.json` + `../csv/{workbook}.sheet-N.csv`）の 1 段落追加。
    *   classify / Phase 1 では CSV 本文を送らない旨（デバッグ者向け）。

## Step-by-Step Implementation Guide

1.  **[x] CSV フィルタ (RED → GREEN)**:
    *   Add `internal/imagetomd/csvhint/filter_test.go` with failing cases.
    *   Implement `filter.go` (`FilterCsvByScope`, `TruncateCsvLines`, `MaxCsvInjectLines`).
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc`.

2.  **[x] 可視スコープモデル (RED → GREEN)**:
    *   Add `internal/imagetomd/analyzer/scope_test.go` with Phase 1 fixture text.
    *   Implement `scope.go` (`PhaseVisibleScope`, `ExtractVisibleScopeFromPhase1`, `buildPromptKnownInfo`).
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc`.

3.  **[x] CSV コンテキストモード (RED → GREEN)**:
    *   Update `csv_context_test.go` for `CsvInjectMode`.
    *   Refactor `csv_context.go` (modes, scope block, revised policy strings).
    *   Update `prompts.go` / `prompts_test.go`.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc`.

4.  **[x] sheet-map 自動解決 (RED → GREEN)**:
    *   Add `resolver_sheetmap_test.go`.
    *   Extend `resolver.go` with sheet-map path resolution.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc`.

5.  **[x] Analyzer 配線 (RED → GREEN)**:
    *   Update `session.go` / `session_test.go` (compressed fields).
    *   Refactor `analyzer.go` per `analyzeContext` / `csvContextForStep` design.
    *   Update `analyzer_test.go` (replace classify CSV expectation, add timing tests).
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc`.

6.  **[x] 契約テスト追加**:
    *   Add `tests/image_to_markdown_csv_visible_scope_test.go`.
    *   Update `tests/image_to_markdown_csv_hint_test.go` if needed.
    *   Run full `./scripts/process/build.sh`.

7.  **[x] ドキュメント**:
    *   Update `README.md` CSV hint section.

8.  **Verification Plan を実行**（下記）。

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```

2.  **Integration Tests (common / imagetomd regression)**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "ImageToMarkdown|CsvHint|CsvVisible|ConversionScope|TableFaithful|ReferenceParity"
    ```
    *   **Log Verification**: 失敗時はどの契約テスト（required/forbidden token）が崩れたかを確認。特に `TestCsvVisibleScope_ChangeHistoryGoldenForbidsEarlyRows` で No.1 行混入を検出。

3.  **E2E Tests (新規/追加)**:

    #### [NEW] `tests/image_to_markdown_csv_visible_scope_test.go`(file://tests/image_to_markdown_csv_visible_scope_test.go)
    *   **テストケース**:
        *   `TestCsvVisibleScope_ChangeHistoryGoldenForbidsEarlyRows` — golden MD に No.43/44 必須、No.1/42 データ行禁止
        *   `TestCsvVisibleScope_FilterUnitMatchesGoldenScenario` — Go フィルタ単体で 2 行抽出
    *   **検証ポイント**: LLM 呼び出しなしで 012 シナリオ 1 の核心（可視外行禁止）を担保

    #### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
    *   **テストケース**: `TestAnalyzeInjectsCsvHintOnPhase2Round1Only`（旧テスト置換）— プロンプト注入タイミング
    *   **検証ポイント**: classify/Phase1 に `--- CSV content ---` が無いこと

    **LLM 実呼び出し E2E について**: 012 仕様どおり CI 必須としない。理由: arctic-tern + codex 依存で時間・不安定。プロンプト記録ユニットテスト + golden 契約テストで要件 C/B/A の決定論部分を担保。開発者による `go run ./cmd/image-to-markdown ...` 実変換は手順書（012 シナリオ 1）として README に記載するが、Verification Plan のゲートには含めない。

### テスト項目設計 (Testing Rules §11)

**ボトムアップ順序**:
1. `csvhint/filter_test.go` — CSV 行フィルタ（末端）
2. `analyzer/scope_test.go` — スコープ抽出
3. `csv_context_test.go` — プロンプトモード
4. `csvhint/resolver_sheetmap_test.go` — パス解決
5. `analyzer_test.go` — Analyze 配線
6. `tests/image_to_markdown_csv_visible_scope_test.go` — 契約

**§11.3 観点チェックリスト**:

| # | 観点 | 対応テスト |
|---|------|-----------|
| 1 | 正常系 | スコープ `{43,44}` で CSV 2 行抽出、golden required tokens |
| 2 | 異常・境界 | scope 空、CSV 200 行超 truncate、sheet-map 不在 |
| 3 | 外部連携 | sheet-map JSON 読込、CSV ファイル Stat |
| 4 | データ一貫性 | フィルタ後 excerpt が元 CSV の部分集合 |
| 5 | 状態遷移 | Phase 1 完了 → visibleScope 設定 → Phase 2 known 初期化 |
| 6 | 設定反映 | `--csv-hint` 優先、`--no-csv-hint-auto` |
| 7 | 副作用 | SessionLog known が O(N²) にならない |

**§11.4 セルフレビュー結果**:
- 網羅性: 決定論ロジックはユニット + 契約テストで「可視外行が出力 MD に無い」まで追跡可能。LLM 実出力の完全保証は開発者実変換が補完（仕様明示）。
- 証拠: recording client がプロンプト内容を直接検証。
- 迂回排除: Go 側 `FilterCsvByScope` により LLM のみに依存しない。
- 依存関係: filter → scope → csv_context → analyzer の順で RED→GREEN。

### 総合判定プロセス (Testing Rules §12)

実装完了後、Verification Plan 実行後に以下を記録する:

```markdown
### 総合判定結果

**判定**: PASS

#### テスト結果サマリ
- build.sh: PASS
- integration_test.sh --specify "ImageToMarkdown|CsvHint|CsvVisible|ConversionScope|TableFaithful|ReferenceParity": PASS

#### チェック項目
| 1 | スキップされたテスト | なし |
| 2 | 部分エラー in log | なし |
| 6 | 新規テストカバレッジ | csv_visible_scope, filter, scope, analyzer timing |
```

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**: CSV visible scope ポリシー、sheet-map 自動解決、partial screenshot 注意事項。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/011-ImageToMarkdown-ExcelCsvHint.md`(file://prompts/phases/000-foundation/branches/main/ideas/011-ImageToMarkdown-ExcelCsvHint.md)
*   **更新内容**: 先頭に「運用上のスコープ制限は `012-...` を参照。012 実装後は classify への CSV 全文付与は廃止」と 1 段落追記（011 履歴は残す）。
