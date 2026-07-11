# 003-GoNative-ExcelPdfImage-with-GoOLE-GoFitz

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/003-GoNative-ExcelPdfImage-with-GoOLE-GoFitz.md`

## Goal Description
`excel-to-pdf` / `pdf-to-image` を Go ネイティブ主手法（`go-ole` + `go-fitz`）へ移行し、Python 実装と同等の sidecar 連携（sheet-map）とファイル命名規約を提供する。CLI 契約（stdout/stderr、exit code）を維持しつつ、公開 Go API からも同機能を呼び出せる状態にする。

## User Review Required
1. `--engine` の既定値を `go-native` とし、`legacy` は明示指定時のみ有効化する方針でよいか。
2. `go-fitz` 導入に伴う CGO 依存（ビルド環境要件）を README へ明記する方針でよいか。
3. 非Windowsでは `go-native` の Excel COM が利用不可なため、`legacy` へフォールバックせずエラーにする方針でよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. Excel->PDF 主手法を go-ole (Windows COM) | Proposed Changes > `internal/exceltopdf/backend_go_native_windows.go`, `internal/exceltopdf/service.go` |
| 2. PDF結合/PDF->Image 主手法を go-fitz | Proposed Changes > `internal/pdfnative/merge_render.go`, `internal/pdftoimage/backend_go_fitz.go` |
| 3. 外部CLIは主手法にしない | Proposed Changes > `internal/exceltopdf/service.go`, `internal/pdftoimage/service.go`, CLI `--engine` |
| 4. sidecar (`*.sheet-map.json`) を再現 | Proposed Changes > `internal/common/sheetmap/types.go`, `internal/common/sheetmap/write.go`, `cmd/excel-to-pdf/main.go` |
| 5. sheet_entries 追跡 | Proposed Changes > `internal/common/sheetmap/types.go`, `internal/exceltopdf/backend_go_native_windows.go` |
| 6. `--sheets` (1-based) | Proposed Changes > `internal/exceltopdf/sheets_parser.go`, `cmd/excel-to-pdf/main.go` |
| 7. `--dpi` / `--format` | Proposed Changes > `cmd/pdf-to-image/main.go`, `internal/pdftoimage/backend_go_fitz.go` |
| 8. シート名反映ファイル命名 + sanitize | Proposed Changes > `internal/common/sheetmap/sanitize.go`, `internal/pdftoimage/naming.go` |
| 9. CLI I/O 契約 + exit code 契約維持 | Proposed Changes > `cmd/excel-to-pdf/main.go`, `cmd/pdf-to-image/main.go`, `internal/common/exitcode/exitcode.go` |
| 10. 公開 Go API から利用可能 | Proposed Changes > `entext.go`, `excelpdf/excelpdf.go`, `pdfimage/pdfimage.go` |
| 任意1. 非Windows向け legacy 設定化 | Proposed Changes > `cmd/*` の `--engine`、`service.go` の engine resolver |
| 任意2. sidecar versioning 拡張性 | Proposed Changes > `internal/common/sheetmap/types.go` (`Version` 定数化) |

## Proposed Changes

### `features/entext/internal/common/sheetmap`

#### [NEW] `features/entext/internal/common/sheetmap/types_test.go`
* **Description**: sidecar データ構造のシリアライズ契約と必須項目を固定する。
* **Technical Design**:
  * `SheetMap`, `SheetEntry` の JSON キー名を固定。
  * `Version` が必ず `1` で出力されることを検証。
* **Logic**:
  * `sheet_entries[].export_status` が `success|failed` であることをテーブル駆動で検証。

#### [NEW] `features/entext/internal/common/sheetmap/sanitize_test.go`
* **Description**: Python 同等の sanitize 規則を固定。
* **Technical Design**:
  * ```go
    tests := []struct{
      in  string
      out string
    }{
      {"A B", "A_B"},
      {"A///B", "AB"},
      {"__A__B__", "A_B"},
    }
    ```
* **Logic**:
  * 禁止文字削除、空白->`_`、連続`_`圧縮、前後`_`除去を検証。

#### [NEW] `features/entext/internal/common/sheetmap/types.go`
* **Description**: sidecar データ構造を定義。
* **Technical Design**:
  * ```go
    package sheetmap

    const Version = 1

    type SheetEntry struct {
      SheetIndex   int    `json:"sheet_index"`
      SheetName    string `json:"sheet_name"`
      ExportStatus string `json:"export_status"`
      PageCount    int    `json:"page_count"`
      Error        string `json:"error,omitempty"`
    }

    type SheetMap struct {
      Version        int          `json:"version"`
      SourceXLSX     string       `json:"source_xlsx"`
      PDFPath        string       `json:"pdf_path"`
      PageSheetNames []string     `json:"page_sheet_names"`
      SheetEntries   []SheetEntry `json:"sheet_entries"`
    }
    ```
* **Logic**:
  * Python 実装の payload 互換キーを維持する。

#### [NEW] `features/entext/internal/common/sheetmap/write.go`
* **Description**: sidecar ファイル出力を実装。
* **Technical Design**:
  * `func Write(pdfPath string, sourceXLSX string, m SheetMap) (string, error)`
* **Logic**:
  * `<pdf_basename>.sheet-map.json` を PDF と同ディレクトリに UTF-8 で保存。

#### [NEW] `features/entext/internal/common/sheetmap/sanitize.go`
* **Description**: ファイル名 sanitize 関数を実装。
* **Technical Design**:
  * `func SanitizeFilename(name string) string`
* **Logic**:
  * Python 実装と同じ正規化規則を再現。

### `features/entext/internal/exceltopdf`

#### [NEW] `features/entext/internal/exceltopdf/sheets_parser_test.go`
* **Description**: `--sheets` 文字列の 1-based パーサをテスト。
* **Technical Design**:
  * 正常: `"1,3,5"` -> `[]int{1,3,5}`
  * 異常: `"a,b"`, `"0,1"`, 重複
* **Logic**:
  * 不正入力は validation error を返す。

#### [NEW] `features/entext/internal/exceltopdf/backend_go_native_windows_test.go`
* **Description**: go-native backend のシート追跡ロジックをモック境界で検証。
* **Technical Design**:
  * COM 実呼び出しを薄い interface で抽象化し、テストでは fake を使用。
* **Logic**:
  * `sheet_entries` と `page_sheet_names` の件数整合を検証。

#### [NEW] `features/entext/internal/exceltopdf/sheets_parser.go`
* **Description**: `--sheets` パーサを実装。
* **Technical Design**:
  * `func ParseSheetIndices(raw string) ([]int, error)`
* **Logic**:
  * 1-based 整数のみ許容し、空文字は `nil`（全シート）として扱う。

#### [NEW] `features/entext/internal/exceltopdf/backend_go_native_windows.go`
* **Description**: `go-ole` による Excel COM backend を実装（Windows）。
* **Technical Design**:
  * ```go
    type GoNativeBackend struct{}
    func (b *GoNativeBackend) Name() string
    func (b *GoNativeBackend) Convert(ctx context.Context, input, outputDir string, sheets []int) (pdfPath string, sm sheetmap.SheetMap, err error)
    ```
* **Logic**:
  * Excel 起動 -> Workbook オープン -> 各シートを一時 PDF 出力 -> ページ数収集 -> COM 解放。
  * `sheet_entries` を success/failed で埋める。

#### [NEW] `features/entext/internal/exceltopdf/backend_go_native_stub.go`
* **Description**: 非Windows用の go-native backend スタブ。
* **Technical Design**:
  * `//go:build !windows`
* **Logic**:
  * `go-native backend is only available on windows` を返す。

#### [MODIFY] `features/entext/internal/exceltopdf/service.go`
* **Description**: `--engine` と `--sheets`、sidecar 情報返却に対応。
* **Technical Design**:
  * ```go
    type ConvertResult struct {
      PDFPath      string
      SheetMapPath string
      SheetMap     sheetmap.SheetMap
    }
    func (s *Service) Convert(ctx context.Context, input, outputDir, backend, engine, sheets string) (ConvertResult, error)
    ```
* **Logic**:
  * `engine=go-native` は `go-ole` backend 優先・必須。
  * `engine=legacy` のみ既存 backend resolver を許容。

### `features/entext/internal/pdfnative` / `features/entext/internal/pdftoimage`

#### [NEW] `features/entext/internal/pdfnative/merge_render_test.go`
* **Description**: `go-fitz` 連結・レンダリング層の I/O 契約を固定。
* **Technical Design**:
  * `MergePDFs(tempPDFs, outputPDF)` と `RenderPDFPages(pdf, dpi, format)` をテスト。
* **Logic**:
  * 空入力、単一PDF、複数PDFの境界条件を検証。

#### [NEW] `features/entext/internal/pdftoimage/naming_test.go`
* **Description**: sidecar 名称連携（`01_<sheet>.png`）を検証。
* **Technical Design**:
  * sidecar あり/なし、ページ数超過時 fallback 名称のテーブル駆動。
* **Logic**:
  * sidecar不足時は `NN_page.ext` へフォールバック。

#### [NEW] `features/entext/internal/pdfnative/merge_render.go`
* **Description**: `go-fitz` で PDF 結合とページレンダリングを実装。
* **Technical Design**:
  * `func MergePDFs(inputs []string, output string) error`
  * `func RenderPDF(pdfPath string, dpi int, format string) ([]RenderedPage, error)`
* **Logic**:
  * `dpi/72` スケールでレンダリングし、PNG/JPEGを出力可能にする。

#### [NEW] `features/entext/internal/pdftoimage/backend_go_fitz.go`
* **Description**: PDF->Image go-native backend を実装。
* **Technical Design**:
  * `type GoFitzBackend struct{}`
  * `Convert(ctx, inputPDF, outputDir, format string, dpi int, sm *sheetmap.SheetMap) ([]string, error)`
* **Logic**:
  * `go-fitz` レンダリング結果を sanitize 命名で保存。

#### [NEW] `features/entext/internal/pdftoimage/naming.go`
* **Description**: sidecar を用いた出力ファイル名決定ロジック。
* **Technical Design**:
  * `func BuildOutputName(pageIndex int, format string, sm *sheetmap.SheetMap) string`
* **Logic**:
  * `page_sheet_names` が存在すれば `NN_<sanitized>.ext`、なければ `NN_page.ext`。

#### [MODIFY] `features/entext/internal/pdftoimage/service.go`
* **Description**: `--engine`/`--dpi`/`--sheet-map` を受け取る。
* **Technical Design**:
  * `Convert(ctx, inputPDF, outputDir, format, backend, engine string, dpi int, sheetMapPath string)`
* **Logic**:
  * `engine=go-native` は `GoFitzBackend` 優先、`legacy` は既存 backend chain。

### `features/entext/cmd`

#### [MODIFY] `features/entext/cmd/excel-to-pdf/main.go`
* **Description**: `--engine`, `--sheets` を追加し sidecar 出力情報を扱う。
* **Technical Design**:
  * flags:
    * `--engine go-native|legacy` (default: `go-native`)
    * `--sheets "1,3,5"`
* **Logic**:
  * 生成物パス（PDF）は stdout、ログは stderr を維持。
  * sidecar パスは `--output-mode json` 時に返却可能にする。

#### [MODIFY] `features/entext/cmd/pdf-to-image/main.go`
* **Description**: `--engine`, `--dpi`, `--sheet-map` を追加。
* **Technical Design**:
  * flags:
    * `--engine go-native|legacy` (default: `go-native`)
    * `--dpi 200`
    * `--sheet-map <path>`
* **Logic**:
  * sidecar 指定がない場合、`<input_pdf_basename>.sheet-map.json` 自動検出を試みる。

### `features/entext` Public API

#### [NEW] `features/entext/entext_gonative_test.go`
* **Description**: 公開 API の新規引数/Artifact 契約を固定。
* **Technical Design**:
  * `ConvertExcelToPDFWithOptions`, `ConvertPDFToImageWithOptions` の validation テスト。
* **Logic**:
  * `engine`, `dpi`, `sheets` の不正値で validation error。

#### [MODIFY] `features/entext/entext.go`
* **Description**: sidecar と go-native オプションを公開 API に追加。
* **Technical Design**:
  * ```go
    type ExcelPDFOptions struct {
      Backend string
      Engine  string
      Sheets  string
    }
    type PDFImageOptions struct {
      Backend      string
      Engine       string
      DPI          int
      SheetMapPath string
    }
    type FileArtifact struct {
      Paths        []string
      SheetMapPath string
    }
    ```
* **Logic**:
  * 既存APIは互換のため維持し、新規 options API へ委譲。

#### [MODIFY] `features/entext/excelpdf/excelpdf.go`
* **Description**: Converter が `engine` / `sheets` を設定可能にする。
* **Technical Design**:
  * `NewWithOptions(opts ConverterOptions)` を追加。
* **Logic**:
  * API 利用でも CLI 同等の出力契約を維持。

#### [MODIFY] `features/entext/pdfimage/pdfimage.go`
* **Description**: Converter が `dpi` / `sheet-map` / `engine` を設定可能にする。
* **Technical Design**:
  * `NewWithOptions(format string, opts ConverterOptions)`
* **Logic**:
  * sidecar 命名連携を API 利用時も適用。

### `tests` (Root Integration/E2E)

#### [MODIFY] `tests/common_backend_selection_test.go`
* **Description**: `--engine` 追加後の契約を検証。
* **Technical Design**:
  * `go-native` / `legacy` の validation と no-fallback 挙動を追加。
* **Logic**:
  * 引数不正は validation message を確認。

#### [MODIFY] `tests/e2e_backend_pipeline_test.go`
* **Description**: sidecar と `--sheets` / `--dpi` を含む E2E を追加。
* **Technical Design**:
  * 追加ケース:
    * `TestE2EExcelToPDF_GoNative_WritesSheetMap`
    * `TestE2EPDFToImage_GoNative_UsesSheetNames`
    * `TestE2EExcelToPDF_InvalidSheets_ExitCode2`
* **Logic**:
  * 手動コマンド確認ではなく、`go run ./cmd/...` 経由で自動検証。

## Step-by-Step Implementation Guide

1. **TDD Red: sidecar/sanitize/sheets parser**
   * Edit `features/entext/internal/common/sheetmap/types_test.go` and `features/entext/internal/common/sheetmap/sanitize_test.go` to define JSON/sanitize behavior.
   * Edit `features/entext/internal/exceltopdf/sheets_parser_test.go` to define `--sheets` parse expectations.
2. **TDD Red: go-native backend contracts**
   * Edit `features/entext/internal/exceltopdf/backend_go_native_windows_test.go` and `features/entext/internal/pdfnative/merge_render_test.go`.
   * Edit `features/entext/internal/pdftoimage/naming_test.go` for sidecar naming rules.
3. **Implement sidecar foundation**
   * Edit `features/entext/internal/common/sheetmap/types.go`, `write.go`, `sanitize.go`.
4. **Implement Excel go-native path**
   * Edit `features/entext/internal/exceltopdf/sheets_parser.go`.
   * Edit `features/entext/internal/exceltopdf/backend_go_native_windows.go` and `backend_go_native_stub.go`.
   * Edit `features/entext/internal/exceltopdf/service.go` to add `engine`/`sheets` and sidecar result.
5. **Implement PDF go-native path**
   * Edit `features/entext/internal/pdfnative/merge_render.go`.
   * Edit `features/entext/internal/pdftoimage/backend_go_fitz.go`, `naming.go`, `service.go`.
6. **Wire CLI flags and behavior**
   * Edit `features/entext/cmd/excel-to-pdf/main.go` with `--engine`, `--sheets`.
   * Edit `features/entext/cmd/pdf-to-image/main.go` with `--engine`, `--dpi`, `--sheet-map`.
7. **Wire public API**
   * Edit `features/entext/entext.go`, `features/entext/excelpdf/excelpdf.go`, `features/entext/pdfimage/pdfimage.go`.
   * Add `features/entext/entext_gonative_test.go`.
8. **Implement E2E tests before final verification**
   * Edit `tests/e2e_backend_pipeline_test.go` and `tests/common_backend_selection_test.go` with new scenarios.
9. **Documentation updates**
   * Edit `README.md` for go-native usage and prerequisites.
   * Edit `prompts/phases/000-foundation/branches/main/ideas/003-GoNative-ExcelPdfImage-with-GoOLE-GoFitz.md` with finalized flags/contracts.
10. **Run Verification Plan**
   * Execute build first, then integration tests with scoped filters.

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   ```bash
   ./scripts/process/build.sh
   ```
   * **Log Verification**: `sheetmap`, `exceltopdf`, `pdfnative`, `pdftoimage`, `entext_gonative` の新規テストが PASS すること。

2. **Integration Tests (common)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "GoNative|SheetMap|Sanitize|InvalidSheets|Backend"
   ```
   * **Log Verification**: sidecar 生成、sanitize 命名、`--sheets` validation、`go-native|legacy` 切替が期待通りであること。

3. **Integration Tests (llm / chain compatibility)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "llm" --specify "pipeline_chain|image_to_markdown|session"
   ```
   * **Log Verification**: upstream を go-native で生成した画像でも `image-to-markdown` が回帰しないこと。

4. **E2E Tests (新規/追加)**:
   go-native フローの E2E を `tests/` 配下に追加する（手動確認の代替禁止）。

   #### [MODIFY] `tests/e2e_backend_pipeline_test.go`(file://tests/e2e_backend_pipeline_test.go)
   * **テストケース**:
     * `TestE2EExcelToPDF_GoNative_WritesSheetMap`
     * `TestE2EPDFToImage_GoNative_UsesSheetNames`
     * `TestE2EExcelToPDF_InvalidSheets_ExitCode2`
   * **検証ポイント**:
     * sidecar が生成されること。
     * 画像名にシート名が反映されること。
     * 不正 `--sheets` で exit code `2` になること。

   #### [MODIFY] `tests/common_backend_selection_test.go`(file://tests/common_backend_selection_test.go)
   * **テストケース**:
     * `TestExcelToPDFEngineGoNativeValidation`
     * `TestPDFToImageEngineGoNativeValidation`
   * **検証ポイント**:
     * engine/option の validation 契約が CLI/API で一致すること。

### Test Item Design (Bottom-Up)

1. **C (leaf)**: `sheetmap` データ構造、sanitize、`--sheets` parser、`go-fitz` merge/render。
2. **B (middle)**: `exceltopdf` / `pdftoimage` service の engine 分岐と sidecar 連携。
3. **A (top)**: CLI flags + public API + root `tests/` E2E。

#### 観点チェックリスト適用
- 正常系: 全シート、部分シート、dpi変更、png/jpeg。  
- 異常系: 不正 `--sheets`、non-windows go-native、sidecar 欠落。  
- 外部連携: Excel COM / go-fitz レンダリング。  
- データ一貫性: `page_sheet_names` と出力画像名の整合。  
- 状態遷移: `go-native` / `legacy` 切替。  
- 設定反映: `--engine`, `--sheets`, `--dpi`, `--sheet-map`。  
- 副作用: sidecar JSON、画像ファイル、session 出力。

#### テスト項目セルフレビュー（§11.4）
- **網羅性**: 要件1-10を leaf->middle->top の順で網羅。  
- **証拠十分性**: JSON内容・命名規約・exit code を自動検証。  
- **迂回排除**: 手動コマンド確認を完了条件にしない。  
- **依存整合**: COM不可環境の失敗契約も明示的にテスト。

### Post-Test Comprehensive Verdict Plan

全テスト完了後、`testing-rules` §12 の 7観点（スキップ、部分エラー、迂回成功、誤設定、順序依存、カバレッジ、外部状態）を表で記録し、`✅/⚠️/❌` 判定を残す。`⚠️` 以上は再現条件（OS・Excel導入・CGO環境）を明記する。

## Documentation

`prompts/specifications`フォルダ以下にある、既存の仕様書およびドキュメントの内容を解析し、本計画で影響を受けるものを最新の状態に更新する。

#### [MODIFY] `README.md`(file://README.md)
* **更新内容**: `go-ole` / `go-fitz` 前提、`--engine go-native|legacy`、`--sheets`、`--dpi`、`--sheet-map` の利用例を追記。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/003-GoNative-ExcelPdfImage-with-GoOLE-GoFitz.md`(file://prompts/phases/000-foundation/branches/main/ideas/003-GoNative-ExcelPdfImage-with-GoOLE-GoFitz.md)
* **更新内容**: 実装確定後のフラグ名、Artifact 形式、engine 既定値、制約（Windows/CGO）を反映。

## Execution Progress

- [x] Step 1: sidecar/sanitize/sheets parser のテスト追加
- [x] Step 2: go-native 契約テスト（命名/validation）追加
- [x] Step 3: sidecar 基盤 (`types/write/sanitize`) 実装
- [x] Step 4: Excel 側 `engine/sheets` 導入と sidecar 出力実装
- [x] Step 5: PDF 側 `engine/dpi/sheet-map` 導入と命名連携実装
- [x] Step 6: CLI flags (`--engine`, `--sheets`, `--dpi`, `--sheet-map`) 配線
- [x] Step 7: 公開 API options 実装
- [x] Step 8: root `tests/` の E2E/統合テスト更新
- [x] Step 9: README / ideas/003 の最終確定反映
- [x] Step 10: build + integration の検証実行
