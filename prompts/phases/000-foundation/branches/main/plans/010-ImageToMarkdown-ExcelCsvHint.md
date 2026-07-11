# 010-ImageToMarkdown-ExcelCsvHint

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/011-ImageToMarkdown-ExcelCsvHint.md`

## Goal Description

Excel ワークブックから **セル値テキストのみ** を UTF-8 CSV として抽出する `excel-to-csv` CLI/API を新設し、`image-to-markdown` が **画像（Vision）と CSV ヒントを併用** できるようにする。プロンプト上は **表構造は画像を正**、**大量セル文字列は CSV 参照可** という使い分けポリシーを Agent に明示する。`010-ConversionScopedGapJudgment` のギャップ判定境界（AssessGap に CSV 非付与・内容校正禁止）は維持する。

## User Review Required

1. **`--csv-hint` は repeatable**（`--ref` と同様、複数 CSV 指定可）とする。自動解決は `--no-csv-hint-auto` で無効化。1 パス限定にする理由がなければこの方針で確定してよいか。
2. **結合セル平坦化規則**: Excel COM / LibreOffice の CSV エクスポートに合わせ、**結合範囲の左上セルのみ非空・他セルは空欄** とする（同一値複写は行わない）。テストで固定する方針でよいか。
3. **CSV 文字コード**: 日本語 Excel 互換のため **UTF-8 BOM 付き** で書き出す。LibreOffice 出力が BOM なしの場合は Go 側で BOM を付与する。
4. **LibreOffice legacy の多シート**: 初版は **1  invocation = 先頭シート相当の 1 CSV**（`--sheets` 指定時は指定インデックス 1 件のみ変換）。Windows では `excel-com` が全シート／`--sheets` 複数指定に対応。Linux CI では validation E2E + 単体テスト中心、成功系 E2E は `excel-com` 利用可能環境向けとする方針でよいか。
5. **任意要件 1〜4 は先送り**: `sheet-map.json` 連携、`excel-to-pdf --with-csv`、CSV 最大文字数 truncate、wrapper script は follow-up。初版は core path のみ実装する方針でよいか。
6. **最終統合プロンプト**: 現行 `--ref` は `GenerateMarkdownPrompt` に付与されていないが、仕様要件 5 に従い **CSV ヒントのみ** 最終統合／リトライプロンプトへ追記する（ref 挙動は変更しない）。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. Excel → CSV 変換新設（UTF-8） | `internal/exceltocsv/`, `cmd/excel-to-csv/`, `entext.go` |
| 2. 書式無視・セル値テキストのみ | `exceltocsv` backend 方針、平坦化規則テスト |
| 3. CLI + 公開 Go API | `cmd/excel-to-csv/main.go`, `ConvertExcelToCSVWithOptions` |
| 4. `--csv-hint` + basename 自動解決 | `csvhint/resolver.go`, CLI/API フラグ |
| 5. Vision 実行プロンプトに付与、AssessGap には非付与 | `analyzer.go` 注入タイミング、`analyzer_test.go` |
| 6. 画像+CSV 使い分けポリシー | `analyzer/csv_context.go`, `prompts_test.go` |
| 7. 010 非矛盾（構造=Vision、値=CSV可） | AssessGap 非付与、Phase2 execute 追記、ConversionScope 回帰 |
| 8. CLI/API 同一ヒント注入 | `entext.ConvertImageToMarkdown` → `Analyze` |
| 9. `--ref` 分離（`[Reference csv hint]`） | `buildCsvHintContext` ラベル契約テスト |
| 任意1. sheet-map 連携 | **先送り** — basename 自動解決のみ |
| 任意2. `--with-csv` | **先送り** |
| 任意3. CSV truncate | **先送り** — プロンプト溢れは follow-up |
| 任意4. wrapper script | **先送り** |

## Proposed Changes

### `internal/exceltocsv`

#### [NEW] `internal/exceltocsv/service_test.go`(file://internal/exceltocsv/service_test.go)
*   **Description**: backend chain / sheets フィルタ / 出力命名の RED テスト（TDD 先行）。
*   **Technical Design**:
    *   ```go
        type fakeCsvBackend struct {
            name string
            out  []string
            err  error
        }
        func (f *fakeCsvBackend) Name() string { return f.name }
        func (f *fakeCsvBackend) ConvertSheets(ctx context.Context, input, outputDir string, sheets []int) ([]string, error) {
            return f.out, f.err
        }

        func TestResolveBackendChainAutoOnWindows(t *testing.T) { /* excel-com → libreoffice */ }
        func TestResolveBackendChainSpecificMode(t *testing.T) { /* excel-com only, libreoffice only */ }
        func TestRunChainFallbackAndSuccess(t *testing.T) { /* 1st fail, 2nd success */ }
        func TestOutputNamingSheetIndex(t *testing.T) {
            // input book.xlsx sheet 2 → book.sheet-2.csv
        }
        ```
*   **Logic**:
    *   `exceltopdf/service_test.go` の chain テストパターンを CSV 向けに複製。
    *   実装前に RED を確認する。

#### [NEW] `internal/exceltocsv/sheets_parser_test.go`(file://internal/exceltocsv/sheets_parser_test.go)
*   **Description**: `--sheets` パースの単体テスト。
*   **Logic**:
    *   `exceltopdf.ParseSheetIndices` を **再利用**（新規パーサーは作らない）。薄いラッパー `ParseSheetIndices(s string) ([]int, error)` が `exceltopdf` を delegate することをテストで固定。

#### [NEW] `internal/exceltocsv/service.go`(file://internal/exceltocsv/service.go)
*   **Description**: Excel → 複数 CSV の変換オーケストレーション。
*   **Technical Design**:
    *   ```go
        package exceltocsv

        type Backend interface {
            Name() string
            ConvertSheets(ctx context.Context, input string, outputDir string, sheets []int) ([]string, error)
        }

        const (
            BackendAuto        = "auto"
            BackendLibreOffice = "libreoffice"
            BackendExcelCOM    = "excel-com"
            EngineLegacy       = "legacy"
        )

        type Service struct { goos string }

        type ConvertResult struct {
            CSVPaths []string
        }

        func New() *Service

        func (s *Service) ConvertWithOptions(
            ctx context.Context,
            input string,
            outputDir string,
            mode string,
            sheets []int,
        ) (ConvertResult, error)
        ```
*   **Logic**:
    *   `engine` 引数は **初版 legacy のみ**（go-native CSV は follow-up）。`mode == ""` は `auto`。
    *   `resolveBackendChain(mode, goos)`:
        *   Windows `auto`: `[excel-com, libreoffice]`
        *   非 Windows `auto`: `[libreoffice]`
    *   `runChain` が `backenderr.AggregateError` で全失敗を返す（`exceltopdf` と同型）。
    *   出力命名: `{workbookBasename}.sheet-{1-based-index}.csv`（例: `R06_09.sheet-1.csv`）。
    *   出力前に `outputDir` を `MkdirAll`。

#### [NEW] `internal/exceltocsv/backend_excel_com_windows.go`(file://internal/exceltocsv/backend_excel_com_windows.go)
*   **Description**: PowerShell + Excel COM によるシート単位 CSV エクスポート。
*   **Technical Design**:
    *   ```go
        type ExcelCOMBackend struct{}
        func (b *ExcelCOMBackend) ConvertSheets(ctx context.Context, input, outputDir string, sheets []int) ([]string, error)
        ```
*   **Logic**:
    *   PowerShell スクリプトで `$excel=New-Object -ComObject Excel.Application` を起動。
    *   `$wb.Worksheets` を 1-based で走査。`sheets == nil` なら全シート、非 nil なら指定インデックスのみ。
    *   各対象シート:
        ```powershell
        $outPath = Join-Path $outputDir ($basename + '.sheet-' + $idx + '.csv')
        $ws.SaveAs($outPath, 62)  # xlCSVUTF8
        ```
    *   結合セルは Excel 標準 CSV 出力に従い **左上のみ値**（平坦化規則）。
    *   数式セルは **表示値** が CSV に出る（COM SaveAs 既定）。
    *   生成後、Go 側で各ファイルを読み、先頭に UTF-8 BOM (`0xEF,0xBB,0xBF`) が無ければ付与して上書き。

#### [NEW] `internal/exceltocsv/backend_excel_com_stub.go`(file://internal/exceltocsv/backend_excel_com_stub.go)
*   **Description**: 非 Windows 用 COM stub（即エラー）。
*   **Logic**: `"excel-com backend requires windows with microsoft excel"` を返す。

#### [NEW] `internal/exceltocsv/backend_libreoffice.go`(file://internal/exceltocsv/backend_libreoffice.go)
*   **Description**: LibreOffice headless CSV 変換。
*   **Logic**:
    *   ```go
        exec.CommandContext(ctx, "libreoffice", "--headless", "--convert-to", "csv", "--outdir", outputDir, input)
        ```
    *   出力ファイル名は LibreOffice 既定（通常 `{basename}.csv`）→ Go 側で `{basename}.sheet-1.csv` に **rename**（初版は先頭シート 1 件）。
    *   `--sheets` で 2 以降が指定された場合、LibreOffice backend は `"libreoffice backend supports single-sheet export in v1; use excel-com on windows for multi-sheet"` エラーを返す（明確な validation エラー）。
    *   BOM 付与処理を COM と共通化（`ensureUTF8BOM(path string) error` ヘルパー）。

#### [NEW] `internal/exceltocsv/runtime.go`(file://internal/exceltocsv/runtime.go)
*   **Description**: `runtimeGOOS()` — `exceltopdf` と同型（テスト差し替え用）。

### `cmd/excel-to-csv`

#### [NEW] `cmd/excel-to-csv/main_test.go`(file://cmd/excel-to-csv/main_test.go)
*   **Description**: CLI validation の RED テスト。
*   **Logic**:
    *   `TestMainInvalidSheetsExitCode2`: `go run . --sheets a,b` → exit 2。
    *   `TestMainInvalidBackendExitCode2`: `--backend invalid` → exit 2。

#### [NEW] `cmd/excel-to-csv/main.go`(file://cmd/excel-to-csv/main.go)
*   **Description**: `excel-to-pdf` と同型の Cobra + Viper CLI。
*   **Technical Design**:
    *   ```go
        // Flags
        // --input / -i, --stdin, --output-dir / -o
        // --backend auto|libreoffice|excel-com
        // --sheets "1,3,5"
        // --config, --verbose, --quiet, --output-mode, --print-json
        ```
*   **Logic**:
    *   入力ループごとに `entext.ConvertExcelToCSVWithOptions(ctx, job, opts)` を呼ぶ。
    *   複数 CSV パスを `commonio.WriteResultPaths` で stdout へ（改行区切り／json モード）。
    *   Viper prefix: `doc_convert_excel_csv`。

### `entext.go`（ルート公開 API）

#### [MODIFY] `entext.go`(file://entext.go)
*   **Description**: Excel CSV 変換 API と `ImageToMarkdownJob` 拡張。
*   **Technical Design**:
    *   ```go
        type ExcelCSVOptions struct {
            Backend string // auto|libreoffice|excel-com
            Sheets  string // "1,3,5"
        }

        func ConvertExcelToCSV(ctx context.Context, job FileJob) (FileArtifact, error)
        func ConvertExcelToCSVWithOptions(ctx context.Context, job FileJob, opts ExcelCSVOptions) (FileArtifact, error)

        type ImageToMarkdownJob struct {
            InputPath      string
            OutputPath     string
            OutputDir      string
            RefPatterns    []string
            CsvHintPaths   []string // 新規。空かつ auto 有効なら自動解決
            NoCsvHintAuto  bool     // 新規。true なら自動解決しない
        }
        ```
*   **Logic**:
    *   `ConvertExcelToCSVWithOptions`:
        1. `input_path` / `output_dir` validation（`ValidationError`）。
        2. `isValidExcelBackend(opts.Backend)` 再利用。
        3. `exceltocsv.ParseSheetIndices(opts.Sheets)` → `svc.ConvertWithOptions(...)`。
        4. `FileArtifact{Paths: result.CSVPaths}` を返す。
    *   `ConvertImageToMarkdown` 内:
        1. 既存 `refresolver.ResolveRefs` の後に `csvhint.ResolveCsvHints(job.CsvHintPaths, job.InputPath, job.NoCsvHintAuto)` を呼ぶ。
        2. `an.Analyze(ctx, job.InputPath, ".", refs, hints)` に hints を渡す。

### `internal/imagetomd/csvhint`

#### [NEW] `internal/imagetomd/csvhint/resolver_test.go`(file://internal/imagetomd/csvhint/resolver_test.go)
*   **Description**: CSV ヒント解決の RED テスト（filesystem 使用、integration 不要）。
*   **Technical Design**:
    *   ```go
        type CsvHint struct {
            Path    string
            Content string
        }

        func TestResolveCsvHintsExplicitPaths(t *testing.T) { /* 2 files loaded */ }
        func TestResolveCsvHintsAutoSameDir(t *testing.T) {
            // imageDir/01_foo.png + imageDir/01_foo.csv
        }
        func TestResolveCsvHintsAutoParentCsvDir(t *testing.T) {
            // imageDir/01_foo.png + imageDir/../csv/01_foo.csv
        }
        func TestResolveCsvHintsMissingReturnsEmpty(t *testing.T) { /* no error */ }
        func TestResolveCsvHintsExplicitMissingIsError(t *testing.T) { /* --csv-hint 指定で不存在 → error */ }
        ```
*   **Logic**:
    *   明示パスは **存在必須**（validation error）。
    *   自動解決のみ missing を空スライス（エラーにしない）。

#### [NEW] `internal/imagetomd/csvhint/resolver.go`(file://internal/imagetomd/csvhint/resolver.go)
*   **Description**: 仕様 3.1 のヒント読込。
*   **Technical Design**:
    *   ```go
        type CsvHint struct {
            Path    string
            Content string
        }

        func ResolveCsvHints(explicitPaths []string, imagePath string, disableAuto bool) ([]CsvHint, error)
        ```
*   **Logic**:
    *   `explicitPaths` 各要素を `filepath.Clean` + `os.ReadFile` → `CsvHint{Path, Content}`。
    *   `disableAuto == false` かつ explicit が空のとき自動解決:
        1. `filepath.Join(filepath.Dir(imagePath), basename+".csv")`（`basename = strings.TrimSuffix(filepath.Base(imagePath), ext)`）
        2. `filepath.Join(filepath.Dir(imagePath), "csv", basename+".csv")`
        3. `filepath.Join(filepath.Dir(imagePath), "..", "csv", basename+".csv")`
    *   最初に `os.Stat` 成功した 1 件のみ採用（複数候補がある場合は上記優先順）。
    *   explicit + auto 両方ある場合: explicit を先に全部読み、auto は **追加しない**（明示指定時は auto 無効）。

### `internal/imagetomd/analyzer`

#### [NEW] `internal/imagetomd/analyzer/csv_context_test.go`(file://internal/imagetomd/analyzer/csv_context_test.go)
*   **Description**: CSV プロンプト契約の RED テスト。
*   **Technical Design**:
    *   ```go
        func TestBuildCsvHintContextContainsUsagePolicy(t *testing.T) {
            got := buildCsvHintContext([]csvhint.CsvHint{{Path: "a.csv", Content: "col1,col2\n1,2"}})
            for _, want := range []string{
                "[Reference csv hint]",
                "大量の行・列データ",
                "CSV の該当セル値を参照して転記してよい",
                "表の構造",
                "Vision",
                "--- CSV content ---",
                "col1,col2",
            } {
                if !strings.Contains(got, want) { t.Fatalf("missing %q", want) }
            }
        }
        func TestPhase2CsvExecuteAppendWhenHintsPresent(t *testing.T) { /* Phase 2 追加指示 */ }
        func TestCsvFinalSynthesisAppend(t *testing.T) { /* 最終統合追記 */ }
        func TestBuildCsvHintContextEmptyReturnsEmpty(t *testing.T) {}
        ```
*   **Logic**:
    *   仕様 3.2 の全文ブロックが **省略なく** 含まれることを固定する。

#### [NEW] `internal/imagetomd/analyzer/csv_context.go`(file://internal/imagetomd/analyzer/csv_context.go)
*   **Description**: CSV ヒントプロンプト合成（仕様 3.2 全文）。
*   **Technical Design**:
    *   ```go
        func buildCsvHintContext(hints []csvhint.CsvHint) string
        func phase2CsvExecuteAppend(hints []csvhint.CsvHint) string
        func csvFinalSynthesisAppend(hints []csvhint.CsvHint) string
        ```
*   **Logic**:
    *   `hints` が空なら各関数は `""`。
    *   `buildCsvHintContext` は hint ごとに以下を連結:
        ```text
        [Reference csv hint]
        Source: {absPath}

        【CSV ヒントの位置づけ】
        - この CSV は元 Excel から抽出したセル値テキストである。
        - CSV には図表・色・結合レイアウト・入れ子の視覚構造は含まれない。

        【画像と CSV の使い分け（必ず守ること）】
        1. 表の構造（列見出し、行数、空欄行、セクション帯、結合・入れ子の有無）は、添付画像の Vision 読取を正とする。
        2. セル内の文字列データについて:
           - 画像から判読可能で量が少ない場合: Vision 転記を優先する。
           - 画像に大量の行・列データが含まれる場合: この CSV の該当セル値を参照して転記してよい。
             Vision で全セルを逐一再読する必要はない。
        3. CSV と画像の文字列が異なる場合: 画像で判読できる範囲を優先する。判読不能・曖昧なセルのみ CSV を参照する。
        4. CSV および画像の内容について、校正・意味整合性・URL/正規表現の妥当性の検証は行わない。

        --- CSV content ---
        {Content}
        ```
    *   `phase2CsvExecuteAppend`（hints 非空時のみ）:
        ```text
        Phase 2 追加指示:
        - 原表の列構成・空欄行・入れ子の有無は画像で確認すること。
        - データ行のセル文字列は、行数が多い場合は上記 CSV から転記してよい。
        - 最終回答は Markdown テーブル形式とし、CSV をそのまま貼り付けるのではなく、
          画像の表構造に合わせて配置すること。
        ```
    *   `csvFinalSynthesisAppend`（hints 非空時のみ）:
        ```text
        - Phase 2 で CSV 参照により取得したセル値は、画像の原表構造に従って Markdown テーブルへ配置すること。
        - CSV にしか存在しない列・行を追加してはならない（画像の表構造を超えない）。
        ```

#### [MODIFY] `internal/imagetomd/analyzer/prompts_test.go`(file://internal/imagetomd/analyzer/prompts_test.go)
*   **Description**: AssessGap / GenerateQuestion が CSV 非含有であることの契約テスト追加。
*   **Logic**:
    *   `TestAssessGapPromptDoesNotMentionCsvHint` — AssessGap 出力に `[Reference csv hint]` / `CSV ヒント` が含まれない。
    *   `TestGenerateQuestionPromptDoesNotMentionCsvHint` — 同上。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: モッククライアントで CSV ヒント注入タイミングを検証。
*   **Technical Design**:
    *   ```go
        type recordingClient struct {
            prompts []string
            // ... existing mock ...
        }

        func TestAnalyzeInjectsCsvHintOnClassifyAndExecuteNotAssess(t *testing.T) {
            hints := []csvhint.CsvHint{{Path: "h.csv", Content: "a,b\n1,2"}}
            // SendText 呼び出しごとに prompt を記録
            // classify prompt: contains [Reference csv hint]
            // assess prompt: does NOT contain [Reference csv hint]
            // phase2 execute: contains Phase 2 追加指示 + [Reference csv hint]
            // generate_question: does NOT contain [Reference csv hint]
        }

        func TestAnalyzeFinalSynthesisIncludesCsvAppendWhenHintsPresent(t *testing.T) {
            // final GenerateMarkdown prompt contains csvFinalSynthesisAppend block
        }

        func TestAnalyzeWithoutHintsUnchangedPromptShape(t *testing.T) {
            // hints=nil → csv blocks absent; baseline substring checks
        }
        ```
*   **Logic**:
    *   010 の ConversionScope モックフロー（assess 二値）を維持したまま CSV 注入のみ追加検証。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: `Analyze` シグネチャ拡張と CSV コンテキスト注入。
*   **Technical Design**:
    *   ```go
        func (a *Analyzer) Analyze(
            ctx context.Context,
            imagePath string,
            workDir string,
            refs []refresolver.RefDocument,
            csvHints []csvhint.CsvHint,
        ) (string, *SessionLog, error)
        ```
*   **Logic**:
    *   冒頭で:
        ```go
        refContext := buildRefContext(refs)
        csvContext := buildCsvHintContext(csvHints)
        visionContext := refContext + csvContext
        ```
    *   **付与する**（`visionContext` 使用）:
        *   分類: `ClassifyPrompt + visionContext + AttachedImageLine(absPath)`
        *   execute（全 Phase）:
            ```go
            answerPrompt := question + ExecutionQuestionSuffix + visionContext + AttachedImageLine(absPath)
            if phase.Num == 2 {
                answerPrompt = question + "\n\n" + Phase2ExecuteHint() + phase2CsvExecuteAppend(csvHints) + ExecutionQuestionSuffix + visionContext + AttachedImageLine(absPath)
            }
            ```
        *   最終統合:
            ```go
            finalPrompt := GenerateMarkdownPrompt(log.Phases) + csvFinalSynthesisAppend(csvHints)
            ```
        *   リトライ:
            ```go
            retryPrompt := GenerateMarkdownRetryPrompt(corpus) + csvFinalSynthesisAppend(csvHints)
            ```
    *   **付与しない**（変更なし）:
        *   `AssessGapPrompt`
        *   `GenerateQuestionPrompt`
        *   `SimpleTextPrompt`

### `cmd/image-to-markdown`

#### [MODIFY] `cmd/image-to-markdown/main.go`(file://cmd/image-to-markdown/main.go)
*   **Description**: CSV ヒント CLI フラグ追加。
*   **Technical Design**:
    *   ```go
        // --csv-hint path   (StringArray / repeatable)
        // --no-csv-hint-auto (bool, default false)
        ```
*   **Logic**:
    *   `job.CsvHintPaths = csvHintPaths`
    *   `job.NoCsvHintAuto = noCsvHintAuto`

#### [MODIFY] `cmd/image-to-markdown/main_test.go`(file://cmd/image-to-markdown/main_test.go)
*   **Description**: 新フラグが `ImageToMarkdownJob` に渡ることの smoke（validation 系既存テストに影響しないこと）。

### `tests/`（統合・E2E）

#### [NEW] `tests/excel_to_csv_e2e_test.go`(file://tests/excel_to_csv_e2e_test.go)
*   **Description**: `excel-to-csv` CLI/API の E2E。
*   **Technical Design**:
    *   ```go
        func TestE2EExcelToCsvInvalidBackendExitCode2(t *testing.T)
        func TestE2EExcelToCsvInvalidSheetsExitCode2(t *testing.T)
        func TestE2EExcelToCsvComExportsKnownCellText(t *testing.T) {
            // Windows + Excel 環境: --backend excel-com -i samples/R06_09.xlsx
            // 出力 CSV に既知文字列が含まれる（samples の既知セルを事前調査して required 文字列を固定）
        }
        func TestRootAPIConvertExcelToCSVValidation(t *testing.T) {
            // entext.ConvertExcelToCSVWithOptions empty input → ValidationError
        }
        ```
*   **Logic**:
    *   `toolCommand(t, "excel-to-csv", args...)` パターン（`e2e_backend_pipeline_test.go` 再利用）。
    *   `TestE2EExcelToCsvComExportsKnownCellText` は Excel COM 成功時のみ既知セル文字列を assert（失敗時は aggregate error で t.Fatalf — skip 禁止）。

#### [NEW] `tests/image_to_markdown_csv_hint_test.go`(file://tests/image_to_markdown_csv_hint_test.go)
*   **Description**: CSV ヒント resolver の統合テスト + 010 回帰。
*   **Technical Design**:
    *   ```go
        func TestCsvHintResolverIntegrationExplicitAndAuto(t *testing.T)
        func TestImageToMarkdownJobAcceptsCsvHintPaths(t *testing.T) {
            // entext.ConvertImageToMarkdown with missing input still validates;
            // job struct compile + CsvHintPaths field wiring via root API test
        }
        func TestConversionScopeRegressionWithCsvHintContract(t *testing.T) {
            // assertReferenceMarkdownContract for 01_変更履歴.md unchanged
        }
        ```
*   **Logic**:
    *   LLM 実行なし。resolver / API wiring / ゴールデン契約のみ。

## Step-by-Step Implementation Guide

1.  **exceltocsv RED tests**:
    *   Add `internal/exceltocsv/service_test.go`, `sheets_parser_test.go`.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc` and confirm new tests FAIL.

2.  **exceltocsv implementation**:
    *   Add `service.go`, `runtime.go`, `backend_libreoffice.go`, `backend_excel_com_windows.go`, `backend_excel_com_stub.go`.
    *   Implement backend chain, output naming, UTF-8 BOM helper.
    *   GREEN unit tests.

3.  **excel-to-csv CLI RED**:
    *   Add `cmd/excel-to-csv/main_test.go` (invalid flags).
    *   Confirm FAIL (command not found or exit code mismatch).

4.  **Public API + CLI**:
    *   Add `ConvertExcelToCSVWithOptions` to `entext.go`.
    *   Add `cmd/excel-to-csv/main.go`.
    *   GREEN CLI tests + `./scripts/process/build.sh --skip-frontend --skip-etc`.

5.  **csvhint resolver RED**:
    *   Add `internal/imagetomd/csvhint/resolver_test.go`.
    *   Confirm FAIL.

6.  **csvhint resolver GREEN**:
    *   Add `resolver.go` with `CsvHint` struct and `ResolveCsvHints`.

7.  **analyzer csv_context RED**:
    *   Add `csv_context_test.go`, extend `prompts_test.go`.
    *   Confirm FAIL.

8.  **analyzer csv_context + wiring GREEN**:
    *   Add `csv_context.go`.
    *   Modify `analyzer.go` signature and injection points.
    *   Update `entext.go` `ConvertImageToMarkdown` to resolve hints and pass to `Analyze`.
    *   Extend `analyzer_test.go` mock tests.
    *   GREEN analyzer unit tests.

9.  **image-to-markdown CLI**:
    *   Add `--csv-hint`, `--no-csv-hint-auto` to `cmd/image-to-markdown/main.go`.
    *   Wire `ImageToMarkdownJob` fields.

10. **Integration / E2E tests**:
    *   Add `tests/excel_to_csv_e2e_test.go`, `tests/image_to_markdown_csv_hint_test.go`.
    *   Extend `tests/root_api_validation_test.go` if needed for new public types.

11. **Documentation**:
    *   Update `README.md` with `excel-to-csv` usage and `image-to-markdown --csv-hint` examples.

12. **Run Verification Plan** (below).

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh --skip-frontend --skip-etc
    ```
    *   **Log Verification**: 新規 `_test.go`（`exceltocsv`, `csvhint`, `csv_context`, `cmd/excel-to-csv`）が PASS。既存 analyzer テストが regress していないこと。

2.  **Integration Tests (feature-focused)**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "ExcelToCsv|CsvHint|ImageToMarkdown|RootAPI"
    ```
    *   **Log Verification**:
        *   `TestE2EExcelToCsvInvalidBackendExitCode2` / `InvalidSheetsExitCode2` PASS
        *   `TestCsvHintResolverIntegrationExplicitAndAuto` PASS
        *   `TestRootAPIConvertExcelToCSVValidation` PASS
        *   `TestConversionScopeRegressionWithCsvHintContract` PASS

3.  **Integration Tests (010/008 regression)**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "TableFaithful|ReferenceParity|ConversionScope|NoPhaseReport"
    ```
    *   **Log Verification**: 既存ゴールデン契約テストが PASS（CSV 機能追加による出力契約変更なし）。

4.  **Full build (final)**:
    ```bash
    ./scripts/process/build.sh
    ```

### E2E Tests

#### [NEW] `tests/excel_to_csv_e2e_test.go`(file://tests/excel_to_csv_e2e_test.go)
*   **テストケース**: `TestE2EExcelToCsvInvalidBackendExitCode2`, `TestE2EExcelToCsvComExportsKnownCellText`
*   **検証ポイント**: CLI がビルド成果物として実行可能、validation exit code 2、Windows+Excel 環境では `samples/R06_09.xlsx` から UTF-8 CSV が生成され既知セル文字列を含む

#### [NEW] `tests/image_to_markdown_csv_hint_test.go`(file://tests/image_to_markdown_csv_hint_test.go)
*   **テストケース**: `TestCsvHintResolverIntegrationExplicitAndAuto`, `TestConversionScopeRegressionWithCsvHintContract`
*   **検証ポイント**: 自動解決パス列挙が実 filesystem で動作、010 ゴールデン Markdown 契約が維持

**E2E 不要と判断した範囲**: LLM を伴う「大量データシートで Phase 2 が CSV を参照した Markdown を返す」検証は、コストと非決定性のため **自動 E2E 対象外**。代替として analyzer モックでプロンプト契約を検証し、手動確認は仕様の「任意・手動」シナリオに委ねる。

### Test Item Design Self-Review (§11.3 / §11.4)

| # | 観点 | 対応テスト |
|---|------|-----------|
| 1 | 正常系 | COM CSV 出力、resolver explicit/auto、buildCsvHintContext 全文 |
| 2 | 異常系・境界 | invalid backend/sheets exit 2、explicit missing CSV error、LibreOffice multi-sheet error |
| 3 | 外部連携 | LibreOffice/COM E2E、filesystem resolver integration |
| 4 | データ一貫性 | BOM 付与、出力命名 `.sheet-N.csv`、CSV 本文がプロンプトに埋め込まれる |
| 5 | 状態遷移 | analyzer モック: classify/execute/final に付与、assess/question に非付与 |
| 6 | 設定反映 | `--no-csv-hint-auto`、`--backend excel-com`、`--sheets` |
| 7 | 副作用 | 自動解決 missing が error にならない、010 ゴールデン契約 non-regression |

**セルフレビュー結論**: 上表により「CSV 変換 → 解決 → プロンプト注入 → 010 非矛盾」のボトムアップ証拠链が成立する。LLM 実変換の完全自動証明はモック契約 + 既存ゴールデンで間接担保し、実画像変換は follow-up 手動／別 idea とする。

### Post-Test Comprehensive Verdict (§12)

実装完了後、Verification Plan の全コマンド成功を確認し、以下を記録する:

```markdown
### 総合判定結果

**判定**: （実装者が記入）

#### テスト結果サマリ
- 全テスト数:
- 成功:
- 失敗:
- 事実上スキップ:

#### チェック項目の結果
| # | チェック項目 | 結果 | 備考 |
|---|------------|------|------|
| 1 | スキップされたテスト | | t.Skip 不使用 |
| 2 | 部分的なエラー | | build/integration ログ |
| 3 | 迂回処理による偽成功 | | fake backend 単体 vs E2E COM |
| 4 | アダプタ誤適用 | | --backend 指定テスト |
| 5 | テスト順序依存 | | t.Parallel 影響 |
| 6 | カバレッジ妥当性 | | 新規 _test.go 一覧 |
| 7 | 外部システム状態 | | Excel/LibreOffice 有無 |

#### 判定理由
（実装者が記入）
```

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**:
    *   `excel-to-csv` CLI 例: `bin/entext/excel-to-csv -i ./book.xlsx -o ./tmp/csv/ --backend auto`
    *   `image-to-markdown` CSV ヒント: `--csv-hint ./tmp/csv/book.sheet-1.csv` および basename 自動解決の説明 1 段落
    *   画像+CSV 使い分け（構造=Vision、大量セル値=CSV 参照可）の 1 文
