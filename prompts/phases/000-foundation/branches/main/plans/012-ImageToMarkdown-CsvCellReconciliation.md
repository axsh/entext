# 012-ImageToMarkdown-CsvCellReconciliation

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/013-ImageToMarkdown-CsvCellReconciliation.md`

## Goal Description

Vision 出力の正規表現・エスケープ記号（例: `\` → `¥` 誤読）を、Excel 由来 CSV セル値との**決定的後処理照合**で補正する。`012` の可視スコープ制約を維持したまま、原表テーブルおよび `SUB_TABLE_Pxx` 展開セクション内の文字列を CSV 優先で置換し、セッションログに diff サマリを記録する。

## User Review Required

1. **主テーブルセルの照合範囲**: 親行の `[詳細](#sub_table_pxx)` リンクや空欄セルは構造として維持し、**テキストを含むセル**（作成者名・日付・短い正規表現行など）のみ CSV と照合する。入れ子全文は SUB_TABLE 側で照合する。この分割でよいか。
2. **CSV 列マッピングの既定**: 変更履歴シートでは `No.` 列をキーとし、SUB_TABLE ヘッダの `変更内容(変更理由)` 等の列名、または `R{n}C{m}` から列インデックスを推定する。列名不一致時は **列インデックス（0-based、No. 列除く）** でフォールバックする。
3. **`<修正前>` vs `＜修正前＞`**: 仕様任意要件 2。初版は **混同記号リストに含めない**（`\`/`¥` の本件のみ必須）。括弧正規化は follow-up とする。
4. **複数 CSV ヒント**: 仕様任意要件 3。初版は **`hints[0]`（明示 `--csv-hint` または sheet-map 解決結果の先頭 1 件）のみ照合**し、複数指定時の優先順位統合は先送り。
5. **公開 API `ReconcileImageMarkdown`**: 統合テスト（LLM 非依存）のため、照合ロジックを `entext` パッケージから呼べる薄いラッパを公開する。Analyze パイプライン内部実装と同一関数を利用する。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1. CSV あり時、最終 MD 確定前に照合 | `analyzer.go` final synthesis 直後、`csvreconcile.Reconcile` |
| A2. 可視スコープ内データ行のみ | `csv_cells.go` `ExtractScopedCells` + `visibleScope.VisibleRowIDs` |
| A3. 原表 + SUB_TABLE 両方 | `reconcile.go` + `subtable.go` |
| A4. 不一致時 CSV 優先置換 | `reconcile.go` `applyReplacements` |
| A5. 決定的 Go 実装（LLM 不使用） | 全 `csvreconcile` パッケージ |
| A6. CSV なし時スキップ | `reconcile.go` early return `Status: skipped_no_csv_hint` |
| B7. No.×列マッピング | `csv_cells.go` `ScopedCellMap` |
| B8. 正規化なし完全一致比較 | `reconcile.go` `strings.EqualFold` 不使用、`==` のみ |
| B9. SUB_TABLE 行分割・順序整列 | `subtable.go` `reconcileSubTableSection` |
| B10. 混同記号リスト拡張可能 | `confusion.go` `DefaultConfusionPairs`（任意モード用） |
| B11. SessionLog diff サマリ | `session.go` `CsvReconcileLog`、`analyzer.go` 代入 |
| C12. 行追加・削除なし | 置換のみ（テーブル行数・SUB_TABLE 行数不変） |
| C13. プロンプト注入変更なし | `csv_context.go` / `prompts.go` 無変更 |
| C14. golden 正規表現行整合 | `reconcile_test.go` シナリオ 4 fixture |
| C15. AssessGap に照合結果非注入 | `prompts_test.go` 回帰維持 |
| D16. CLI/API 同一ロジック | `entext.go` + `analyzer.go` 同一 `csvreconcile.Reconcile` |
| D17. `--no-csv-reconcile` 任意 | `cmd/image-to-markdown/main.go`, `ImageToMarkdownJob`, `AnalyzeOptions` |
| 任意1 Levenshtein メトリクス | **先送り** |
| 任意2 括弧正規化 | **先送り**（User Review #3） |
| 任意3 複数 CSV 優先順位 | **先送り**（User Review #4） |
| 任意4 fail-open | `reconcile.go` マッピング失敗時 `Status: partial` + 警告、MD 原文維持 |

## Proposed Changes

### `internal/imagetomd/csvreconcile`

#### [NEW] `internal/imagetomd/csvreconcile/reconcile_test.go`(file://internal/imagetomd/csvreconcile/reconcile_test.go)
*   **Description**: 照合エンジン全体の RED テスト（TDD 先行）。ボトムアップ Step 3。
*   **Technical Design**:
    *   ```go
        func TestReconcile_SkipsWhenNoCsvHints(t *testing.T) {
            got := Reconcile("md", nil, analyzer.PhaseVisibleScope{VisibleRowIDs: []string{"44"}})
            // Status == "skipped_no_csv_hint", Markdown unchanged
        }
        func TestReconcile_SkipsWhenDisabled(t *testing.T) {
            got := Reconcile("md", hints, scope, ReconcileOptions{Enabled: false})
        }
        func TestReconcile_FixesYenCorruptionInSubTable(t *testing.T) {
            // fixture: tests/testdata/csv_reconcile/01_変更履歴_yen_corruption.md
            // + tests/testdata/csv_reconcile/変更履歴_no43_44.csv
            // assert: no "event¥.html", contains "event\\.html"
        }
        func TestReconcile_DoesNotAddOutOfScopeRows(t *testing.T) {
            // corrupted md has only 43,44; csv has row 1; output must not gain row 1 patterns
        }
        func TestReconcile_RecordsReplacementDiff(t *testing.T) {
            // len(result.Log.Replacements) > 0, Before contains ¥, After contains \
        }
        ```
*   **Logic**:
    *   シナリオ 1・3・4 を fixture ベースで検証。
    *   テスト項目セルフレビュー（§11.4）: 本テスト群成功 → `\`/`¥` 本件修正・スコープ維持・diff 記録が確認できる。

#### [NEW] `internal/imagetomd/csvreconcile/csv_cells_test.go`(file://internal/imagetomd/csvreconcile/csv_cells_test.go)
*   **Description**: CSV セル抽出の RED テスト。ボトムアップ Step 1。
*   **Technical Design**:
    *   ```go
        func TestExtractScopedCells_No43And44(t *testing.T) {
            cells, err := ExtractScopedCells(csvContent, []string{"43", "44"})
            // cells["43"]["変更内容(変更理由)"] contains "^/search/event.html"
            // cells["44"]["変更内容(変更理由)"] contains "event\\.html" and "[^\\/]+"
        }
        func TestExtractScopedCells_EmptyScopeReturnsEmpty(t *testing.T)
        func TestExtractScopedCells_InvalidCSVReturnsError(t *testing.T)
        func TestExtractScopedCells_SkipsOutOfScopeRows(t *testing.T)
        ```
*   **Logic**:
    *   `tests/testdata/csv_reconcile/変更履歴_no43_44.csv` を `//go:embed` または `testdata/` 同梱。
    *   `encoding/csv` Reader（`LazyQuotes: true`, `FieldsPerRecord: -1`）で `csvhint/filter.go` の `parseCSVRecords` と同等パース。

#### [NEW] `internal/imagetomd/csvreconcile/subtable_test.go`(file://internal/imagetomd/csvreconcile/subtable_test.go)
*   **Description**: SUB_TABLE セクション解析・行整列の RED テスト。ボトムアップ Step 2。
*   **Technical Design**:
    *   ```go
        func TestParseSubTableSections_FindsP01P02(t *testing.T)
        func TestSubTableSection_RowIDFromHeader(t *testing.T) {
            // "### SUB_TABLE_P02（R3C3: No.44 変更内容(変更理由)）" → rowID "44", col "変更内容(変更理由)"
        }
        func TestReconcileSubTableSection_ReplacesFencedRegexLines(t *testing.T)
        func TestReconcileSubTableSection_PreservesNonDataLines(t *testing.T) {
            // "書き換え前のURL(from条件)" 等のラベル行は維持
        }
        func TestAlignCSVLinesToMarkdownLines_OrderPreserved(t *testing.T)
        ```
*   **Logic**:
    *   CSV セル `"line1\nline2\n^/search/ev_evid..."` を `\n` 分割。
    *   MD セクション内のデータ行（バッククォート行 `` `...` `` または正規表現らしい bullet）のみ CSV 行候補とマッチング。
    *   マッチング: 同一インデックス優先 → 部分文字列類似（`normalizeForMatch` で `¥`→`\` のみ試行、**置換値は常に CSV 原文**）→ 未マッチ行はそのまま。

#### [NEW] `internal/imagetomd/csvreconcile/confusion.go`(file://internal/imagetomd/csvreconcile/confusion.go)
*   **Description**: 拡張可能な混同記号リスト（任意モード・将来用）。
*   **Technical Design**:
    *   ```go
        type ConfusionPair struct {
            Vision rune // often misread
            CSV    rune // ground truth in CSV
        }

        var DefaultConfusionPairs = []ConfusionPair{
            {'¥', '\\'}, // U+00A5 YEN SIGN → backslash
        }
        ```
*   **Logic**:
    *   必須パスでは **CSV 全文行置換**を優先し、本ファイルは補助（部分マッチ候補生成）のみ。

#### [NEW] `internal/imagetomd/csvreconcile/csv_cells.go`(file://internal/imagetomd/csvreconcile/csv_cells.go)
*   **Description**: 可視スコープ行の CSV セル値 map 構築。
*   **Technical Design**:
    *   ```go
        // ScopedCellMap: rowID -> columnName -> cellText (raw, unnormalized)
        type ScopedCellMap map[string]map[string]string

        type ColumnIndex struct {
            Name  string
            Index int // 0-based field index in CSV record
        }

        func ExtractScopedCells(csvContent string, visibleRowIDs []string) (ScopedCellMap, []ColumnIndex, error)

        func parseCSVWithNoColumn(content string) (records [][]string, headerIdx, noColIdx int, columns []ColumnIndex, err error)
        ```
*   **Logic**:
    1.  `csvhint.FilterCsvByScope` と同様に先頭 5 行以内で `No.` 列を検出（`encoding/csv` `ReadAll`）。
    2.  ヘッダ行から `No.` 以外の列名を `ColumnIndex` として収集（空列名は `col_{i}`）。
    3.  `visibleRowIDs` を set 化。データ行で `No.` 値が set に含まれる行のみ map に追加。
    4.  各セル値は CSV フィールドの **生文字列**（`\` エスケープ保持）。
    5.  `visibleRowIDs` が空の場合は空 map を返す（呼び出し側で reconcile スキップ）。

#### [NEW] `internal/imagetomd/csvreconcile/subtable.go`(file://internal/imagetomd/csvreconcile/subtable.go)
*   **Description**: SUB_TABLE セクションの解析と CSV セル行との整列置換。
*   **Technical Design**:
    *   ```go
        type SubTableSection struct {
            Placeholder string // "SUB_TABLE_P02"
            RowID       string // "44"
            ColumnName  string // "変更内容(変更理由)"
            StartLine   int
            EndLine     int
            Lines       []string
        }

        var (
            reSubTableHeader = regexp.MustCompile(`(?i)^###\s+(SUB_TABLE_P\d+|sub_table_p\d+)(?:\s*[（(](?:R\d+C\d+:\s*)?No\.(\d+)\s+([^）)]+)[）)])?`)
            reFencedLine    = regexp.MustCompile("^-\\s*`(.+)`\\s*$")
            reBulletData    = regexp.MustCompile(`^-\s+(\^/|/corpinfo/)`)
        )

        func ParseSubTableSections(markdown string) []SubTableSection

        func reconcileSubTableSection(sec SubTableSection, csvCellText string) (newLines []string, replacements []Replacement)

        func splitCSVCellLines(text string) []string

        func isLabelLine(line string) bool // "書き換え前", "〈修正前〉", section headers — not replaced
        ```
*   **Logic**:
    1.  MD を行分割。`## 入れ子構造の展開` 以降を走査し `### SUB_TABLE` でセクション分割。
    2.  ヘッダ regex から `RowID`（No.44 → `"44"`）と `ColumnName` を抽出。欠落時は placeholder 順序と `visibleScope.NestedPlaceholders` から推定。
    3.  `csvCellText = cells[rowID][columnName]`。列名不一致時は `変更内容` を含む列、または列 index 最大の長文セルを使用。
    4.  `csvLines := splitCSVCellLines(csvCellText)`。
    5.  セクション内各 `line`:
        - `isLabelLine(line)` → そのまま出力。
        - `` - `payload` `` 形式 → `payload` と `csvLines` を順序走査。`payload != csvLine` なら `` - `csvLine` `` に置換し `Replacement` 記録。
        - `- ^/...` 非フェンス行も同様。
    6.  **行追加・削除禁止**: 入力行数 == 出力行数。

#### [NEW] `internal/imagetomd/csvreconcile/reconcile.go`(file://internal/imagetomd/csvreconcile/reconcile.go)
*   **Description**: 照合オーケストレーション。公開エントリポイント。
*   **Technical Design**:
    *   ```go
        type Replacement struct {
            RowID    string `json:"row_id,omitempty"`
            Location string `json:"location"`
            Before   string `json:"before"`
            After    string `json:"after"`
        }

        type ReconcileLog struct {
            Status       string        `json:"status"` // applied | skipped_no_csv_hint | skipped_disabled | partial
            HintPath     string        `json:"hint_path,omitempty"`
            Replacements []Replacement `json:"replacements,omitempty"`
            Warning      string        `json:"warning,omitempty"`
        }

        type ReconcileOptions struct {
            Enabled bool // default true when hints present
        }

        type Result struct {
            Markdown string
            Log      ReconcileLog
        }

        func Reconcile(
            markdown string,
            hints []csvhint.CsvHint,
            scope analyzer.PhaseVisibleScope,
            opts ReconcileOptions,
        ) Result
        ```
*   **Logic**:
    1.  `len(hints)==0` → `{Markdown: markdown, Log: {Status: "skipped_no_csv_hint"}}`。
    2.  `!opts.Enabled` → `{Status: "skipped_disabled"}`。
    3.  `hint := hints[0]`。`ExtractScopedCells(selectCsvExcerpt(hint, scope), scope.VisibleRowIDs)` — `selectCsvExcerpt` は `analyzer` 側の `FilterCsvByScope` 呼び出しと同等のため、`csvhint.FilterCsvByScope(hint.Content, scope.VisibleRowIDs, csvhint.MaxCsvInjectLines)` を直接使用。
    4.  **原表テーブル**: `reconcileMainTable(markdown, cells, scope)` — `| 43 |` / `| 44 |` 行のテキスト列（リンク列除く）で CSV 値と不一致なら置換。行 ID 列・空セル・`[詳細](#...)` は構造維持。
    5.  **SUB_TABLE**: 各 `ParseSubTableSections` 結果に `reconcileSubTableSection` 適用。行範囲を MD 内でスプライス。
    6.  全 `Replacement` を `Log.Replacements` に集約。1 件以上あれば `Status: "applied"`、マッピング失敗があれば `partial` + `Warning`。
    7.  LLM 呼び出しなし。

#### [NEW] `tests/testdata/csv_reconcile/変更履歴_no43_44.csv`(file://tests/testdata/csv_reconcile/変更履歴_no43_44.csv)
*   **Description**: No.43/44 行のみの CSV 抜粋 fixture（`FilterCsvByScope` 相当の内容）。
*   **Logic**: 本番 `URL書き換えルール一覧(学生PC).sheet-1.csv` から No.43/44 行を抽出した最小 CSV。正規表現に `\` を含む。

#### [NEW] `tests/testdata/csv_reconcile/01_変更履歴_yen_corruption.md`(file://tests/testdata/csv_reconcile/01_変更履歴_yen_corruption.md)
*   **Description**: `event¥.html` / `[^¥/]+` を意図的に混入した劣化 MD（`reference_parity/01_変更履歴.md` ベース）。

### `internal/imagetomd/analyzer`

#### [MODIFY] `internal/imagetomd/analyzer/session.go`(file://internal/imagetomd/analyzer/session.go)
*   **Description**: セッションログに照合結果フィールドを追加。
*   **Technical Design**:
    *   ```go
        type SessionLog struct {
            ImagePath     string                    `json:"image_path"`
            // ... existing fields ...
            CsvReconcile  *csvreconcile.ReconcileLog `json:"csv_reconcile,omitempty"`
        }
        ```
*   **Logic**:
    *   JSON 後方互換: 新フィールド optional。既存コンシューマは無視可能。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: final synthesis 成功後に reconcile を呼び出す。
*   **Technical Design**:
    *   ```go
        type AnalyzeOptions struct {
            // ... existing ...
            NoCsvReconcile bool
        }

        // After line ~275 (needsFinalSynthesisRetry pass), before analyze_end:
        reconcileResult := csvreconcile.Reconcile(
            markdown,
            actx.csvHints,
            actx.visibleScope,
            csvreconcile.ReconcileOptions{Enabled: !a.opts.NoCsvReconcile},
        )
        markdown = reconcileResult.Markdown
        log.CsvReconcile = &reconcileResult.Log
        a.progressf("step=csv_reconcile status=%s replacements=%d", reconcileResult.Log.Status, len(reconcileResult.Log.Replacements))
        ```
*   **Logic**:
    *   retry 成功後・`ErrEmptyMarkdown` チェック後に実行（空 MD には適用しない）。
    *   `persistSession` は既存 `analyze_end` 前の最終 save で `CsvReconcile` を含む。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: パイプライン統合（recording client）で reconcile 呼び出しと SessionLog 記録を検証。
*   **Technical Design**:
    *   ```go
        func TestAnalyzeAppliesCsvReconcileWhenHintPresent(t *testing.T) {
            // recording client returns markdown with event¥.html in final synthesis response
            // csv hint fixture with event\.html
            // assert output markdown has event\.html, session CsvReconcile.Status == "applied"
        }
        func TestAnalyzeSkipsCsvReconcileWithoutHint(t *testing.T)
        func TestAnalyzeSkipsCsvReconcileWhenNoCsvReconcileOption(t *testing.T)
        func TestAssessGapPromptStillOmitsCsvReconcile(t *testing.T) // C15 regression
        ```

### Root `entext` package

#### [MODIFY] `entext.go`(file://entext.go)
*   **Description**: Job オプションと公開 reconcile ラッパ（統合テスト用・CLI/API 共通）。
*   **Technical Design**:
    *   ```go
        type ImageToMarkdownJob struct {
            // ... existing ...
            NoCsvReconcile bool
        }

        type CsvReconcileResult struct {
            Markdown string
            Log      csvreconcile.ReconcileLog
        }

        // ReconcileImageMarkdown applies CSV cell reconciliation without running Vision.
        func ReconcileImageMarkdown(
            markdown string,
            csvHintPaths []string,
            visibleRowIDs []string,
            opts ReconcileOptions,
        ) (CsvReconcileResult, error)

        type ReconcileOptions struct {
            NoCsvReconcile bool
        }
        ```
*   **Logic**:
    *   `ReconcileImageMarkdown`: `csvhint.ResolveCsvHints(csvHintPaths, "", true)` で明示パスのみ読込 → `PhaseVisibleScope{VisibleRowIDs: visibleRowIDs}` → `csvreconcile.Reconcile`。
    *   `ConvertImageToMarkdown`: `AnalyzeOptions{NoCsvReconcile: job.NoCsvReconcile}` を渡す。

#### [MODIFY] `cmd/image-to-markdown/main.go`(file://cmd/image-to-markdown/main.go)
*   **Description**: CLI フラグ `--no-csv-reconcile` 追加。
*   **Technical Design**:
    *   ```go
        flags.BoolVar(&noCsvReconcile, "no-csv-reconcile", false, "Disable CSV cell reconciliation after synthesis")
        // ImageToMarkdownJob.NoCsvReconcile: noCsvReconcile
        ```

### `tests/` integration

#### [NEW] `tests/image_to_markdown_csv_reconcile_test.go`(file://tests/image_to_markdown_csv_reconcile_test.go)
*   **Description**: LLM 非依存の照合 E2E（公開 API 経由）。
*   **Technical Design**:
    *   ```go
        func TestCsvReconcileFixesYenCorruption(t *testing.T) {
            corrupted := readTestdata("csv_reconcile/01_変更履歴_yen_corruption.md")
            csvPath := filepath.Join("testdata", "csv_reconcile", "変更履歴_no43_44.csv")
            result, err := entext.ReconcileImageMarkdown(corrupted, []string{csvPath}, []string{"43", "44"}, entext.ReconcileOptions{})
            // assert: no event¥.html, contains event\.html, Status applied
        }
        func TestCsvReconcileSkipsWithoutCsvHint(t *testing.T)
        func TestCsvReconcileReferenceGoldenHasNoYenCorruption(t *testing.T) {
            assertReferenceMarkdownContract(t, "01_変更履歴.md", []string{"event\\.html"}, []string{"event¥.html", "[^¥/]+"})
        }
        ```
*   **Logic**:
    *   シナリオ 1・2 の決定的部分を LLM なしで検証。
    *   `//go:build integration` タグ付与。

## Step-by-Step Implementation Guide

1.  **Fixtures 作成**:
    *   `tests/testdata/csv_reconcile/変更履歴_no43_44.csv` を本番 CSV から No.43/44 抜粋で作成。
    *   `tests/testdata/csv_reconcile/01_変更履歴_yen_corruption.md` を golden ベースで `¥` 混入版作成。

2.  **Step 1 — CSV セル抽出 (RED → GREEN)**:
    *   `internal/imagetomd/csvreconcile/csv_cells_test.go` を追加し失敗確認。
    *   `csv_cells.go` で `ExtractScopedCells` / `parseCSVWithNoColumn` 実装。
    *   `./scripts/process/build.sh --skip-frontend --skip-etc` で PASS。

3.  **Step 2 — SUB_TABLE 整列 (RED → GREEN)**:
    *   `subtable_test.go` 追加。
    *   `subtable.go` で `ParseSubTableSections` / `reconcileSubTableSection` 実装。
    *   build PASS。

4.  **Step 3 — Reconcile オーケストレーション (RED → GREEN)**:
    *   `reconcile_test.go` 追加（シナリオ 4 含む）。
    *   `confusion.go` / `reconcile.go` 実装。
    *   build PASS。

5.  **Step 4 — Analyzer 統合**:
    *   `session.go` に `CsvReconcile` フィールド追加。
    *   `analyzer.go` final synthesis 後に `csvreconcile.Reconcile` 呼び出し。
    *   `AnalyzeOptions.NoCsvReconcile` 追加。
    *   `analyzer_test.go` に recording client テスト追加。
    *   build PASS。

6.  **Step 5 — CLI / 公開 API**:
    *   `entext.go` に `NoCsvReconcile` / `ReconcileImageMarkdown` 追加。
    *   `cmd/image-to-markdown/main.go` に `--no-csv-reconcile` 追加。
    *   build PASS。

7.  **Step 6 — 統合テスト**:
    *   `tests/image_to_markdown_csv_reconcile_test.go` 追加。
    *   Verification Plan を実行。

8.  **Verification Plan の実行**（下記）。

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```
    *   **Log Verification**: `internal/imagetomd/csvreconcile` 全テスト PASS。`analyzer_test.go` の reconcile 関連 PASS。

2.  **Integration Tests**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "CsvReconcil|ImageToMarkdown|CsvHint|CsvVisible"
    ```
    *   **Log Verification**:
        *   `TestCsvReconcileFixesYenCorruption` PASS — `event¥.html` 非存在、`event\.html` 存在。
        *   `TestCsvReconcileSkipsWithoutCsvHint` PASS — `Status: skipped_no_csv_hint`。
        *   既存 `CsvVisible` / `CsvHint` / `ImageToMarkdown` 回帰 PASS。

3.  **E2E Tests**:
    *   **LLM 実呼び出し E2E は CI 必須としない**（仕様 E2E 方針）。理由: 照合は決定的 pure logic。`ReconcileImageMarkdown` + fixture がシナリオ 1 の合格条件をカバー。Analyzer wiring は `analyzer_test.go` で検証。
    *   #### [NEW] `tests/image_to_markdown_csv_reconcile_test.go`(file://tests/image_to_markdown_csv_reconcile_test.go)
        *   **テストケース**: `TestCsvReconcileFixesYenCorruption`, `TestCsvReconcileSkipsWithoutCsvHint`, `TestCsvReconcileReferenceGoldenHasNoYenCorruption`
        *   **検証ポイント**: CSV 照合で `\`/`¥` 修正、スキップ条件、golden 参照整合

### テスト項目設計（§11 準拠）

| 順序 | 観点 | テスト |
| :--- | :--- | :--- |
| Step 1 | 正常系（CSV セル抽出） | `TestExtractScopedCells_No43And44` |
| Step 1 | 境界値（空スコープ） | `TestExtractScopedCells_EmptyScopeReturnsEmpty` |
| Step 2 | 正常系（SUB_TABLE 置換） | `TestReconcileSubTableSection_ReplacesFencedRegexLines` |
| Step 2 | 副作用なし（ラベル維持） | `TestReconcileSubTableSection_PreservesNonDataLines` |
| Step 3 | 正常系（全体 `\`/`¥`） | `TestReconcile_FixesYenCorruptionInSubTable` |
| Step 3 | 状態遷移（diff 記録） | `TestReconcile_RecordsReplacementDiff` |
| Step 3 | 行追加禁止 | `TestReconcile_DoesNotAddOutOfScopeRows` |
| Step 4 | パイプライン統合 | `TestAnalyzeAppliesCsvReconcileWhenHintPresent` |
| Step 5 | スキップ | `TestAnalyzeSkipsCsvReconcileWithoutHint` |
| Integration | 公開 API | `TestCsvReconcileFixesYenCorruption` |

**§11.4 セルフレビュー結果**: 上記成功時、末端 C（csvreconcile）→ B（subtable/csv_cells）→ A（analyzer/entext CLI）のボトムアップ確認が完了し、シナリオ 1–4 の合格条件を LLM なしで言い切れる。AssessGap 非注入は既存 `prompts_test.go` 回帰で担保。

### 総合判定プロセス（§12）

実装完了後、Verification Plan 実行後に以下を記録する:

```markdown
### 総合判定結果

**判定**: （実装者がテスト実行後に記入）

#### チェック項目
| # | 項目 | 確認方法 |
|---|------|----------|
| 1 | スキップなし | build/integration ログに SKIP なし |
| 2 | 部分エラーなし | stderr に panic/recovered なし |
| 3 | primary path | reconcile が final synthesis 後に実行（analyzer_test） |
| 4 | 012 回帰 | CsvVisible テスト PASS |
| 5 | 010 維持 | AssessGap に csv_reconcile 非注入 |
```

## Documentation

#### [MODIFY] `cmd/image-to-markdown/README.md` または `internal/imagetomd/README.md`(file://internal/imagetomd/README.md)（存在する方）
*   **更新内容**:
    *   `--csv-hint` 指定時、最終 Markdown 確定前に CSV セル照合が実行される旨。
    *   `--no-csv-reconcile` で無効化可能。
    *   開発者検証用 CLI 例（シナリオ 1）:
        ```bash
        go run ./cmd/image-to-markdown \
          -i tmp/output/pc/images/01_変更履歴.png \
          -o tmp/output/pc/md/01_変更履歴.md \
          --csv-hint tmp/output/pc/csv/URL書き換えルール一覧(学生PC).sheet-1.csv \
          --tern-mode inproc --tern-config settings/tern/tern-config.yaml --verbose
        ```
        照合のみ検証:
        ```bash
        # ReconcileImageMarkdown は entext API 経由（統合テスト参照）
        ```

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/011-ImageToMarkdown-ExcelCsvHint.md`(file://prompts/phases/000-foundation/branches/main/ideas/011-ImageToMarkdown-ExcelCsvHint.md)
*   **更新内容**: 冒頭に 013 照合後処理への参照注記を 1 行追加（011 の「画像優先」は Vision フェーズ、013 は後処理で CSV 文字精度を補完）。
