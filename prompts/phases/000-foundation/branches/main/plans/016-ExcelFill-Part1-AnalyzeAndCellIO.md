# 016-ExcelFill-Part1-AnalyzeAndCellIO

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/017-ExcelFill-TemplateAnalyzeAndFill.md`
>
> **継続**: Part 2 は `prompts/phases/000-foundation/branches/main/plans/017-ExcelFill-Part2-FillDialogueAndVisual.md`

## Goal Description

Excel テンプレート埋め込み機能の **基盤（Part 1）** を実装する。(1) analyze/fill 共通の参照 Markdown・追加プロンプト解決 (`refprompt`)、(2) excelize によるセル読取/書込 (`excelcell`)、(3) 構造 Markdown の型とレンダリング、(4) `excel-template-analyze` CLI / 公開 API（画像パイプライン + セル対応の統合）。埋め込み対話・見た目検証ループは Part 2。

## User Review Required

1. **Excel ライブラリ**: セル番地付き読取/書込に **`github.com/xuri/excelize/v2`** を新規依存として採用する（`.xlsx` 主対象。`.xlsm` は読取可能ならサポート、マクロ実行はしない）。COM は PDF 化のみ既存経路を使う。
2. **複数シートの構造 Markdown**: 初版は **単一ファイルに全シートを連結**（`-o` 必須）。シートごとに `## Sheet: {name}` セクションを並べる。`--output-dir` は「出力ディレクトリ + 入力 basename の `.structure.md`」を書くショートカットとする。
3. **`--ref-dir`**: 指定ディレクトリ配下を **再帰**して `.md` を収集する（`filepath.WalkDir`）。`--ref` は既存どおりワークスペース相対パスに対する正規表現（root=`"."`）。
4. **意味解析（系統 1）**: 既存 `image-to-markdown` の多段 Phase は再利用せず、テンプレート専用の **1〜2 回の `SendImagePrompt`**（構造抽出プロンプト）とする。LLM 応答は Markdown 断片として受け取り、セル対応（系統 2）とマージする。単体/統合テストでは `SemanticAnalyzer` インタフェースをモック可能にする。
5. **任意要件（仕様 任意 1〜5）はすべて Part 1/2 とも先送り**: `--sheets` 限定、`--visual-strict`、`--session-log`、非対話一括、schema version 表示は follow-up（内部定数 `StructureMarkdownVersion = "1"` は埋め込むが CLI フラグは出さない）。
6. **Linux CI**: excelize 経路の解析（セル対応）とヒント解決は OS 非依存で E2E 可能。PDF/画像化を伴う意味解析 E2E は Windows（excel-com）または LibreOffice 利用可能環境向けとし、CI では **モック SemanticAnalyzer + 実セル読取** を主経路とする。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1 `excel-template-analyze` CLI | `cmd/excel-template-analyze/`（本 Part）。`excel-fill` は Part 2 |
| A2 公開 Go API | `entext.AnalyzeExcelTemplate`（本 Part）。`FillExcel` は Part 2 |
| A3 I/O・exit code 契約 | CLI + `apperr` / `exitcode`（既存踏襲） |
| A4 `--config`/`--verbose`/`--quiet`/`--output-mode` | 両 CLI（analyze は本 Part） |
| B5 入力 Excel 必須 | analyze CLI/API validation |
| B6 系統1画像 + 系統2ライブラリ | `internal/excelanalyze/` |
| B7 構造 Markdown 必須見出し | `internal/excelanalyze/structure/` |
| B8 `-o` / `--output-dir` | analyze CLI |
| B9 `--keep-work-dir` | analyze options |
| B10 `--ref`/`--ref-dir`/`--prompt`/`--prompt-file`（analyze） | `internal/common/refprompt/` + analyze CLI |
| C11–C20 fill 系 | **Part 2** |
| D21 原本非破壊 | Part 2 で書込コピー。本 Part の analyze は読取のみ |
| D22 クロスプラットフォームセル I/O | `excelcell` + excelize |
| D23 単体テスト（ヒント・セル） | 各 `*_test.go` |
| D24 統合テスト analyze→fill | analyze 分は本 Part、一連は Part 2 |
| D25 ヒント解決の共通化 | `internal/common/refprompt/` |
| 任意1–5 | **先送り**（User Review #5） |

## Proposed Changes

### `internal/common/refprompt`

#### [NEW] `internal/common/refprompt/resolve_test.go`(file://internal/common/refprompt/resolve_test.go)
*   **Description**: TDD RED — 参照 Markdown / 追加プロンプト解決。
*   **Technical Design**:
    *   ```go
        func TestResolveHintsEmpty(t *testing.T)
        func TestResolveHintsRefPatternsMatchAndDedupe(t *testing.T)
        func TestResolveHintsRefDirRecursiveCollectsMarkdown(t *testing.T)
        func TestResolveHintsPromptFilesConcatOrder(t *testing.T)
        func TestResolveHintsInlinePromptsConcatOrder(t *testing.T)
        func TestResolveHintsMissingPromptFileReturnsValidationError(t *testing.T)
        func TestResolveHintsInvalidRegexpReturnsError(t *testing.T)
        func TestResolveHintsSkipsNonMarkdownInRefDir(t *testing.T)
        ```
*   **Logic**:
    *   一時ディレクトリに `.md` / `.txt` を配置し、重複パスが 1 回だけ `Refs` に入ることを検証。
    *   `Prompts` の連結順: 全 `--prompt`（CLI 指定順）の後に全 `--prompt-file` 内容（指定順）。区切りは `"\n\n"`。
    *   欠落 `prompt-file` は `apperr` validation 系エラー。

#### [NEW] `internal/common/refprompt/resolve.go`(file://internal/common/refprompt/resolve.go)
*   **Description**: analyze/fill 共通ヒント解決。
*   **Technical Design**:
    *   ```go
        package refprompt

        type HintInput struct {
            RefPatterns []string
            RefDirs     []string
            Prompts     []string
            PromptFiles []string
            Root        string // default "."
        }

        type HintBundle struct {
            Refs    []refresolver.RefDocument // Path + Content
            Prompts string                    // joined additional prompt text
        }

        func Resolve(in HintInput) (HintBundle, error)

        // FormatForPrompt returns labeled context for LLM injection.
        func FormatForPrompt(b HintBundle) string
        ```
*   **Logic**:
    *   `Root == ""` → `"."`。
    *   Refs: `refresolver.ResolveRefs(patterns, root)` の結果 + 各 `RefDirs` を `WalkDir` して `.md` 全文読込。絶対パスで dedupe 後、パス昇順ソート。
    *   `FormatForPrompt`: 
        *   refs があれば `[Reference markdown context]` + 各 `### path` + content
        *   prompts 非空なら `[Additional prompt hints]` + prompts
        *   利用方針文（定数）を先頭に付与: 解析時は「意味理解のヒントでありセル番地の正解ではない」等（analyze/fill で文言切替可能な `ModeAnalyze` / `ModeFill` 引数を `FormatForPrompt` に持たせる）。

#### [NEW] `internal/common/refprompt/format.go`(file://internal/common/refprompt/format.go)
*   **Description**: ヒント利用方針の固定文とフォーマット。
*   **Logic**:
    *   `const HintPolicyAnalyze = "..."` / `HintPolicyFill = "..."` を定義し、仕様 B10 / C12 の方針をコードに固定。

### `internal/excelcell`

#### [NEW] `internal/excelcell/workbook_test.go`(file://internal/excelcell/workbook_test.go)
*   **Description**: TDD RED — excelize ラッパの読取/書込（フィクスチャはテスト内で最小 xlsx を excelize 自身で生成）。
*   **Technical Design**:
    *   ```go
        func TestOpenListsSheetsAndCells(t *testing.T)
        func TestMergedCellsReported(t *testing.T)
        func TestSetCellAndSaveAsDoesNotModifySource(t *testing.T)
        func TestCopyThenWritePreservesUnrelatedCells(t *testing.T)
        func TestOpenMissingFileReturnsError(t *testing.T)
        ```
*   **Logic**:
    *   ソースファイルの mtime/内容が `SaveAs` 後も不変であること。
    *   結合範囲は excelize の `GetMergeCells` 結果を文字列 slice で返す。

#### [NEW] `internal/excelcell/workbook.go`(file://internal/excelcell/workbook.go)
*   **Description**: excelize 依存を閉じ込めたセル I/O。
*   **Technical Design**:
    *   ```go
        package excelcell

        type Cell struct {
            Sheet string
            Ref   string // A1
            Value string // display string
        }

        type SheetSnapshot struct {
            Name       string
            Index      int // 1-based order in workbook
            Cells      []Cell
            MergeRanges []string
        }

        type Workbook struct { /* *excelize.File + path */ }

        func Open(path string) (*Workbook, error)
        func (w *Workbook) Close() error
        func (w *Workbook) Snapshots() ([]SheetSnapshot, error)
        func (w *Workbook) SetCellValue(sheet, cellRef, value string) error
        func (w *Workbook) SaveAs(path string) error

        func CopyFile(src, dst string) error // io copy; used by fill in Part 2
        ```
*   **Logic**:
    *   `Snapshots`: 使用領域（`GetRows`）を走査し、非空セルを `Cell` 化。空行も行番号把握のため必要なら最大行列を記録。
    *   依存追加: `go get github.com/xuri/excelize/v2`（`go.mod` 更新）。

### `internal/excelanalyze/structure`

#### [NEW] `internal/excelanalyze/structure/doc_test.go`(file://internal/excelanalyze/structure/doc_test.go)
*   **Description**: 構造ドキュメントの Markdown レンダリング契約テスト。
*   **Technical Design**:
    *   ```go
        func TestRenderContainsRequiredSections(t *testing.T)
        func TestRenderFillFieldCellTable(t *testing.T)
        func TestParseRoundTripOptional(t *testing.T) // 初版は Render のみでも可。Parse は fill が読むなら最小見出し抽出
        ```
*   **Logic**:
    *   必須見出し（固定英語キー + 日本語併記可）:
        *   `# Excel Template Structure`
        *   `## Metadata`
        *   `## Sheet: {name}`
        *   `### Overview`
        *   `### Semantic Structure`
        *   `### Cell Mapping`
        *   `### Edit Notes`
    *   Cell Mapping は表形式: `| field_id | label | sheet | cells | role |`

#### [NEW] `internal/excelanalyze/structure/doc.go`(file://internal/excelanalyze/structure/doc.go)
*   **Description**: 構造 Markdown のデータモデルとレンダラ。
*   **Technical Design**:
    *   ```go
        package structure

        const MarkdownVersion = "1"

        type Document struct {
            Version    string
            SourcePath string
            AnalyzedAt time.Time // UTC
            Backend    string    // pdf/image backend summary
            HintsUsed  bool
            Sheets     []Sheet
        }

        type Sheet struct {
            Name     string
            Index    int
            Overview string
            Semantic string // freeform markdown from vision
            Fields   []Field
            Notes    []string
        }

        type Field struct {
            ID     string
            Label  string
            Role   string
            Cells  []string // A1 refs or ranges
            Merge  string
        }

        func Render(doc Document) string
        func MergeSemantic(doc *Document, sheetName, semanticMarkdown string)
        func AttachCellSnapshots(doc *Document, snaps []excelcell.SheetSnapshot) error
        ```
*   **Logic**:
    *   `AttachCellSnapshots`: 各シートの非空セル・結合を `Fields` の候補または「Raw Cells」補助表として追記。Vision が付けた label とセルを突き合わせる簡易ヒューリスティクス（ラベル文字列が近傍セル値と一致 → その右/下を入力候補）を実装。不一致時は Raw 表を残し Field は Vision 側 ID を優先。
    *   `AnalyzedAt` は `time.Now().UTC()`。

### `internal/excelanalyze`

#### [NEW] `internal/excelanalyze/analyze_test.go`(file://internal/excelanalyze/analyze_test.go)
*   **Description**: オーケストレーション単体（モック Vision + 実 excelcell フィクスチャ）。
*   **Technical Design**:
    *   ```go
        type fakeSemantic struct {
            out map[string]string // sheet -> semantic md
            err error
        }
        func (f *fakeSemantic) AnalyzeSheets(ctx context.Context, images []SheetImage, hintText string) (map[string]string, error)

        func TestAnalyzeMergesSemanticAndCells(t *testing.T)
        func TestAnalyzeKeepWorkDirPreservesArtifacts(t *testing.T)
        func TestAnalyzeCleansTempWhenKeepWorkDirEmpty(t *testing.T)
        func TestAnalyzeValidationMissingInput(t *testing.T)
        func TestAnalyzeInjectsHintTextIntoSemantic(t *testing.T)
        ```

#### [NEW] `internal/excelanalyze/semantic.go`(file://internal/excelanalyze/semantic.go)
*   **Description**: 系統 1 — 画像 → 意味構造。
*   **Technical Design**:
    *   ```go
        type SheetImage struct {
            SheetName string
            ImagePath string
        }

        type SemanticAnalyzer interface {
            AnalyzeSheets(ctx context.Context, images []SheetImage, hintText string) (map[string]string, error)
        }

        type TernSemanticAnalyzer struct {
            Client tern.Client
            Agent  string
            Model  string
        }

        func (a *TernSemanticAnalyzer) AnalyzeSheets(...) (map[string]string, error)
        ```
*   **Logic**:
    *   シートごとにセッション作成（または 1 セッションで順次）し、`SendImagePrompt` に構造抽出プロンプト + `hintText` を付与。
    *   プロンプト定数 `TemplateStructurePrompt`: 記入欄/固定ラベル/セクションを Markdown で返すよう指示。非対話サフィックスを付与（`imagetomd` の NonInteractive 相当文を本パッケージ定数で持つ）。

#### [NEW] `internal/excelanalyze/pipeline.go`(file://internal/excelanalyze/pipeline.go)
*   **Description**: Excel → PDF → images の既存 API 呼び出し。
*   **Technical Design**:
    *   ```go
        type PipelineOptions struct {
            PDFBackend   string
            PDFEngine    string
            ImageBackend string
            ImageEngine  string
            DPI          int
            Sheets       string // reserved; Part1 では常に全シート（User Review #5）
        }

        func RenderTemplateImages(ctx context.Context, inputExcel, workDir string, opts PipelineOptions) (pdfPath, sheetMap string, images []SheetImage, err error)
        ```
*   **Logic**:
    *   `entext.ConvertExcelToPDFWithOptions` → `ConvertPDFToImageWithOptions`（パッケージ循環を避けるため、internal から `exceltopdf` / `pdftoimage` を直接呼ぶ。公開 entext は呼ばない）。
    *   sheet-map があれば画像とシート名を対応付け。無ければファイル名順 + `Sheet-N`。

#### [NEW] `internal/excelanalyze/service.go`(file://internal/excelanalyze/service.go)
*   **Description**: analyze 本体。
*   **Technical Design**:
    *   ```go
        type Options struct {
            InputPath    string
            OutputPath   string // final structure md
            KeepWorkDir  string // empty => temp + cleanup
            Hints        refprompt.HintInput
            Pipeline     PipelineOptions
            Semantic     SemanticAnalyzer // nil => TernSemanticAnalyzer via Runtime wiring at API layer
            Verbose      bool
        }

        type Result struct {
            StructurePath string
            WorkDir       string
        }

        func Analyze(ctx context.Context, opts Options) (Result, error)
        ```
*   **Logic**:
    1. Validate input/output。
    2. `refprompt.Resolve` → `FormatForPrompt(..., ModeAnalyze)`。
    3. workDir 準備。
    4. `RenderTemplateImages`。
    5. `Semantic.AnalyzeSheets`。
    6. `excelcell.Open` → `Snapshots` → `structure.Document` 構築 → `MergeSemantic` → `AttachCellSnapshots` → `Render` → ファイル書込。
    7. `KeepWorkDir==""` なら workDir 削除。

### `cmd/excel-template-analyze`

#### [NEW] `cmd/excel-template-analyze/main_test.go`(file://cmd/excel-template-analyze/main_test.go)
*   **Description**: フラグ契約の軽いテスト（validateOutput 等）。既存 CLI の `main_test.go` パターンに合わせる。

#### [NEW] `cmd/excel-template-analyze/main.go`(file://cmd/excel-template-analyze/main.go)
*   **Description**: CLI エントリ。
*   **Technical Design**:
    *   Flags:
        *   `-i/--input` (required unless 将来 stdin; 初版は単一 `-i` 必須)
        *   `-o/--output` または `--output-dir`（どちらか必須。両方なら `-o` 優先）
        *   `--ref` (string slice), `--ref-dir` (string slice)
        *   `--prompt` (string slice), `--prompt-file` (string slice)
        *   `--keep-work-dir`
        *   `--backend` / `--engine`（PDF）, 画像用は既存と同様 `--dpi` 任意
        *   Tern: `--server-url`, `--tern-mode`, `--tern-config`, `--agent`, `--model`
        *   共通: `--config`, `--verbose`, `--quiet`, `--output-mode`, `--print-json`
    *   Viper key prefix: `doc_excel_template_analyze`
*   **Logic**:
    *   `entext.AnalyzeExcelTemplate` を呼び、stdout に structure path を出力。

### `entext` 公開 API

#### [MODIFY] `entext.go`(file://entext.go)
*   **Description**: `AnalyzeExcelTemplate` 追加。
*   **Technical Design**:
    *   ```go
        type ExcelTemplateAnalyzeJob struct {
            InputPath   string
            OutputPath  string
            OutputDir   string // if OutputPath empty: filepath.Join(OutputDir, basename+".structure.md")
            KeepWorkDir string
            RefPatterns []string
            RefDirs     []string
            Prompts     []string
            PromptFiles []string
        }

        type ExcelTemplateAnalyzeConfig struct {
            ServerURL      string
            Agent          string
            Model          string
            TernMode       string
            TernConfigPath string
            PDFBackend     string
            PDFEngine      string
            ImageBackend   string
            ImageEngine    string
            DPI            int
            Verbose        bool
            Quiet          bool
        }

        type StructureArtifact struct {
            StructurePath string
        }

        func AnalyzeExcelTemplate(ctx context.Context, job ExcelTemplateAnalyzeJob, cfg ExcelTemplateAnalyzeConfig) (StructureArtifact, error)
        ```
*   **Logic**:
    *   Validation: InputPath 必須、OutputPath または OutputDir 必須。
    *   Tern `BuildRuntime` → `TernSemanticAnalyzer` を注入して `excelanalyze.Analyze`。
    *   Quiet/Verbose は logger 経由（既存 image-to-markdown と同様）。

#### [NEW] `entext_excel_analyze_test.go`(file://entext_excel_analyze_test.go)
*   **Description**: 公開 API の validation 単体（入力欠落で `IsValidation`）。

### Integration tests / fixtures

#### [MODIFY] `tests/e2e_backend_pipeline_test.go`(file://tests/e2e_backend_pipeline_test.go)
*   **Description**: `toolCommand` に `excel-template-analyze`（および Part 2 で `excel-fill`）を追加。
*   **Logic**:
    *   ```go
        case "excel-template-analyze":
            pkg = "./cmd/excel-template-analyze"
        case "excel-fill":
            pkg = "./cmd/excel-fill"
        ```

#### [NEW] `tests/testdata/excel_fill/minimal_template.xlsx`(file://tests/testdata/excel_fill/minimal_template.xlsx)
*   **Description**: ラベル + 空記入欄の最小ブック（生成スクリプトまたはテストで excelize 生成して committed 化）。ラベル `Name` が A1、記入欄 B1 など。

#### [NEW] `tests/testdata/excel_fill/hints/analyze_hint.md`(file://tests/testdata/excel_fill/hints/analyze_hint.md)
*   **Description**: 「B 列が記入欄」等のヒント。

#### [NEW] `tests/excel_template_analyze_e2e_test.go`(file://tests/excel_template_analyze_e2e_test.go)
*   **Description**: 統合テスト（LLM なし経路）。
*   **Technical Design**:
    *   ```go
        func TestExcelTemplateAnalyzeAPI_WithInjectedSemantic(t *testing.T)
        // 公開 API が Semantic 注入未対応なら internal を tests から呼べないため、
        // パッケージ excelanalyze の Analyze を tests 同モジュールから直接は不可。
        // → 方針: E2E は (1) excelcell 実ファイルの structure に Cell Mapping が含まれることを
        //   CLI 実 LLM なしでは担保しにくいので、API に Optional SemanticAnalyzer は
        //   テスト専用ではなく Config に載せない。
        //   代わりに internal 単体で十分カバーし、E2E は次を行う:
        func TestE2EExcelTemplateAnalyzeWritesStructureMarkdown(t *testing.T)
        // Windows+Excel または LibreOffice がある場合のみ画像経路まで。
        // 常時実行: TestRootAPIAnalyzeExcelTemplateValidation
        func TestRootAPIAnalyzeExcelTemplateValidation(t *testing.T)
        func TestE2ERefPromptHintsResolvedByAnalyzeCLI_Dry(t *testing.T)
        // CLI に --help と不正引数 exit 2
        ```
*   **Logic（確定方針）**:
    *   **常時**: validation E2E +（可能なら）excelize のみの「セル対応セクション生成」を `excelanalyze.Analyze` が `Semantic` に no-op/fake を取れることで internal テスト済み → 統合は `TestRootAPIAnalyzeExcelTemplateValidation` と、バイナリ起動の引数エラー。
    *   **実画像+LLM**: Nightly / 手動カテゴリには載せない。Part 2 のモック充実を優先。
    *   追加: `tests` から見えるよう、`Analyze` の fake 経路を **単体で完結**させ、統合は CLI exit code と「`--prompt-file` 欠落で 2」を検証。

> より強い E2E のため、`ExcelTemplateAnalyzeConfig` にテスト用フックは置かない。代わりに `internal/excelanalyze` のカバレッジを厚くする。

### Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **Description**: `excel-template-analyze` の Usage・ヒントフラグ・Go API `AnalyzeExcelTemplate` を追記。CLI 数の記述を更新（Part 2 完了まで「追加予定 excel-fill」と明記してよい）。

## Step-by-Step Implementation Guide

1. **依存追加**: `github.com/xuri/excelize/v2` を `go.mod` に追加。
2. **refprompt TDD**: `resolve_test.go` RED → `resolve.go` / `format.go` GREEN。
3. **excelcell TDD**: `workbook_test.go` RED → `workbook.go` GREEN。
4. **structure TDD**: `doc_test.go` RED → `doc.go` GREEN（`AttachCellSnapshots` 含む）。
5. **excelanalyze**: `semantic.go`（interface + Tern 実装）→ `pipeline.go` → `service.go`。`analyze_test.go` で fake Semantic。
6. **公開 API**: `entext.go` + validation test。
7. **CLI**: `cmd/excel-template-analyze/main.go`。
8. **fixtures + 統合**: testdata、`toolCommand` 更新、validation/E2E。
9. **README** 更新。
10. **Verification Plan** を実行。

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   ```bash
   ./scripts/process/build.sh
   ```

2. **Integration Tests（本 Part 範囲）**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "ExcelTemplate|AnalyzeExcelTemplate|excel-template-analyze|excelcell|refprompt"
   ```
   *   **Log Verification**: structure 出力パスが stdout に出ること、validation 失敗が exit 2 相当で扱われること。

3. **既存 Excel リグレッション**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "excel|Excel|sheet"
   ```

4. **E2E Tests**:
    #### [NEW] `tests/excel_template_analyze_e2e_test.go`(file://tests/excel_template_analyze_e2e_test.go)
    *   **テストケース**: `TestRootAPIAnalyzeExcelTemplateValidation` — 空 Input で validation。
    *   **テストケース**: `TestE2EExcelTemplateAnalyzeCLI_InvalidArgsExit2` — `-i` なしで終了コード 2。
    *   **検証ポイント**: 新 CLI がビルドされ、既存 I/O 契約に乗る。

### テスト項目設計（§11）とセルフレビュー

**ボトムアップ順序**:
1. `refprompt`（末端のファイル解決）
2. `excelcell`（末端の xlsx I/O）
3. `structure`（純関数レンダリング）
4. `excelanalyze`（上記合成 + fake Semantic）
5. 公開 API / CLI / 統合

**観点チェックリスト**:
| # | 観点 | 本 Part での対応 |
|---|------|------------------|
| 1 | 正常系 | fake Semantic + 実 xlsx → structure md に Cell Mapping |
| 2 | 異常系 | 欠落ファイル、不正 regexp、入力欠落 |
| 3 | 外部連携 | excelize 実ファイル、PDF パイプラインは pipeline 単体/環境依存 |
| 4 | データ一貫性 | Snapshot → Render に番地が残る |
| 5 | 状態遷移 | KeepWorkDir on/off で成果物残存 |
| 6 | 設定反映 | ヒントが Semantic に渡る |
| 7 | 副作用 | 原本 xlsx 非変更（analyze は読取のみ） |

**セルフレビュー結果**:
1. **網羅性**: 解析の主成果（構造 MD）とヒント共通化はカバー。fill/見た目は Part 2 明示。
2. **証拠の十分性**: Render 文字列アサーションとセル値の往復で証拠を取る。
3. **迂回排除**: Semantic は interface 注入で Tern 実呼びを単体から排除しつつ、本番配線は API で固定。
4. **依存関係**: refprompt/excelcell 成功が analyze 成功の前提。

### 総合判定プロセス（§12）

Verification 完了後、スキップ・WARN・フォールバック偽成功の有無を確認し、実装ウォークスルーに総合判定を記載すること。

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**: `excel-template-analyze` のインストール成果物パス、フラグ表、`AnalyzeExcelTemplate` サンプル。

## 継続計画について

本 Part 完了後、必ず `017-ExcelFill-Part2-FillDialogueAndVisual.md` を実施する。Part 2 は `excelcell.CopyFile` / `SetCellValue`、`refprompt`、構造 Markdown を前提に `excel-fill` の対話・埋め込み・見た目検証ループを追加する。
