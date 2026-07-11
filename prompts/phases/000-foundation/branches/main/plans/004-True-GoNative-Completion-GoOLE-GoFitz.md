# 004-True-GoNative-Completion-GoOLE-GoFitz

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/004-True-GoNative-Completion-GoOLE-GoFitz.md`

## Goal Description

暫定実装の `go-native` 経路を、仕様通りの「真の Go ネイティブ実装」に置換する。具体的には `excel-to-pdf --engine go-native` を `go-ole` 直接 COM 制御で、`pdf-to-image --engine go-native` を `go-fitz` 直接レンダリングで実装し、sidecar の正確性（実シート名・実ページ数・失敗シート情報）と CLI/API 契約（exit code, stdout/stderr）を完了させる。

## Execution Progress

- [x] 1. TDD Red: sidecar/compat/sheetsの契約固め
- [x] 2. TDD Red: go-ole/go-fitz 経路固定
- [x] 3. Sidecar 互換処理の実装
- [x] 4. Excel go-native 完了実装
- [x] 5. PDF go-native 完了実装
- [x] 6. CLI 契約修正
- [x] 7. 公開 API の整合
- [x] 8. E2E テスト実装（必須）
- [x] 9. ドキュメント更新
- [x] 10. Verification Plan 実行

## User Review Required

1. シート単位処理で一部シート失敗時、最終終了コードを `1`（部分失敗）にしつつ、成功分 PDF と sidecar を残す方針でよいか。
2. 非Windowsで `--engine go-native` を指定した `excel-to-pdf` は、`legacy` に自動フォールバックせず即失敗とする方針でよいか。
3. `pdf-to-image` の sidecar 自動検出優先順位を `--sheet-map` 明示指定 > `<input>.sheet-map.json` 自動検出の順で確定してよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. `excel-to-pdf --engine go-native` を go-ole 直接COMで実装 | Proposed Changes > `internal/exceltopdf/backend_go_native_windows.go` |
| 2. `pdf-to-image --engine go-native` を go-fitz 直接レンダリングで実装 | Proposed Changes > `internal/pdfnative/merge_render.go`, `internal/pdftoimage/backend_go_fitz.go` |
| 3. シート単位処理（取得/選択/出力/ページ数算出） | Proposed Changes > `internal/exceltopdf/backend_go_native_windows.go`, `internal/exceltopdf/sheets_parser.go` |
| 4. sidecar に実データ記録 | Proposed Changes > `internal/common/sheetmap/types.go`, `internal/common/sheetmap/write.go`, `internal/exceltopdf/backend_go_native_windows.go` |
| 5. sidecar 連携命名 (`01_<sheet>.ext`) と不足時フォールバック | Proposed Changes > `internal/pdftoimage/naming.go`, `internal/pdftoimage/service.go` |
| 6. `--dpi` が go-native レンダリングに反映 | Proposed Changes > `internal/pdfnative/merge_render.go`, `internal/pdftoimage/backend_go_fitz.go`, `cmd/pdf-to-image/main.go` |
| 7. `--format` (`png|jpeg`) 受付 | Proposed Changes > `internal/pdftoimage/service.go`, `cmd/pdf-to-image/main.go`, API validation |
| 8. 引数エラー時 exit code `2` | Proposed Changes > `cmd/excel-to-pdf/main.go`, `cmd/pdf-to-image/main.go`, `internal/common/exitcode/exitcode.go` |
| 9. go-native 制約明示（非Windows/CGO） | Proposed Changes > `README.md`, `internal/exceltopdf/backend_go_native_stub.go` |
| 10. 公開 Go API から利用可能 | Proposed Changes > `entext.go`, `excelpdf/excelpdf.go`, `pdfimage/pdfimage.go` |
| 任意1. sidecar version 互換方針 | Proposed Changes > `internal/common/sheetmap/types.go`, `internal/common/sheetmap/read_compat.go` |
| 任意2. go-native debug ログ | Proposed Changes > `internal/exceltopdf/backend_go_native_windows.go`, `internal/pdftoimage/service.go` |

## Proposed Changes

### `features/entext/internal/common/sheetmap`

#### [NEW] `features/entext/internal/common/sheetmap/read_compat_test.go`
* **Description**: sidecar の version 互換読込契約を固定するテストを追加する。
* **Technical Design**:
  * `version=1` 正常読込
  * `version` 欠落時は `1` と同等扱い
  * 未来 version は warning 相当で読込（hard fail しない）
* **Logic**:
  * 既存 sidecar を壊さず将来拡張できることを検証する。

#### [MODIFY] `features/entext/internal/common/sheetmap/types_test.go`
* **Description**: `sheet_entries` の実ページ数と `page_sheet_names` 展開整合を追加検証。
* **Technical Design**:
  * テーブル駆動で `page_count` 展開関数を検証。
* **Logic**:
  * `page_count=2` のシートは `page_sheet_names` に2回展開されることを保証する。

#### [NEW] `features/entext/internal/common/sheetmap/read_compat.go`
* **Description**: 互換読込処理を追加する。
* **Technical Design**:
  * ```go
    func ReadCompat(path string) (*SheetMap, error)
    ```
* **Logic**:
  * version 欠落/未知versionでも可能な範囲で読込継続し、致命フォーマット不一致時のみ error。

### `features/entext/internal/exceltopdf`

#### [NEW] `features/entext/internal/exceltopdf/backend_go_native_windows_test.go`
* **Description**: go-ole 実装のコアロジックをテスト可能化するため、COMアダプタ境界を導入して Red テストを追加。
* **Technical Design**:
  * ```go
    type excelApp interface {
      OpenWorkbook(path string) (workbook, error)
      Close() error
    }
    type workbook interface {
      SheetCount() int
      SheetName(i int) (string, error)
      ExportSheetPDF(i int, out string) error
      Close() error
    }
    ```
* **Logic**:
  * `--sheets` 指定/未指定、シート失敗時の `sheet_entries` 記録、`page_sheet_names` 展開を検証。

#### [MODIFY] `features/entext/internal/exceltopdf/sheets_parser_test.go`
* **Description**: 境界値（空白混在、重複、負数、末尾カンマ）のテストケースを拡張。
* **Technical Design**:
  * `tests := []struct{name, in string; want []int; wantErr bool}{...}`
* **Logic**:
  * 引数パーサの曖昧解釈を排除する。

#### [MODIFY] `features/entext/internal/exceltopdf/backend_go_native_windows.go`
* **Description**: 暫定実装（固定 `Sheet1`）を削除し、go-ole 直接制御へ置換。
* **Technical Design**:
  * ```go
    type GoNativeBackend struct {
      newApp func() (excelApp, error)
    }
    func (b *GoNativeBackend) Convert(ctx context.Context, input, outputDir string, sheets []int) (string, sheetmap.SheetMap, error)
    ```
* **Logic**:
  * 実シート名取得
  * 対象シート決定
  * シートごと PDF 出力
  * 出力PDFのページ数算出
  * `sheet_entries` と `page_sheet_names` 構築
  * COM確実解放（defer）

#### [MODIFY] `features/entext/internal/exceltopdf/service.go`
* **Description**: `engine=go-native` で上記 backend のみを使用し、暫定/legacy 混在を除去。
* **Technical Design**:
  * `ConvertWithOptions(..., engine string, sheets []int) (ConvertResult, error)`
* **Logic**:
  * go-native は COM経路必須
  * 非Windowsでは stub error を返す（fallbackしない）

### `features/entext/internal/pdfnative`

#### [NEW] `features/entext/internal/pdfnative/merge_render_test.go`
* **Description**: go-fitz レンダリング層の契約テストを追加。
* **Technical Design**:
  * 単一ページ PDF から 200/300 DPI で画像サイズ比較
  * `png` / `jpeg` 出力テスト
* **Logic**:
  * DPI の実効反映を自動検証する。

#### [NEW] `features/entext/internal/pdfnative/merge_render.go`
* **Description**: go-fitz を使った PDF 結合・ページレンダリングを実装。
* **Technical Design**:
  * ```go
    type RenderedPage struct {
      PageIndex int
      Width     int
      Height    int
      Bytes     []byte
      Format    string
    }
    func MergePDFs(inputs []string, output string) error
    func RenderPages(pdfPath string, dpi int, format string) ([]RenderedPage, error)
    ```
* **Logic**:
  * `dpi/72` スケールで各ページをレンダリングする。

### `features/entext/internal/pdftoimage`

#### [NEW] `features/entext/internal/pdftoimage/backend_go_fitz_test.go`
* **Description**: go-native backend が外部CLIを経由せず go-fitz を使う契約を固定。
* **Technical Design**:
  * renderer dependency injection で呼び出し経路を検証。
* **Logic**:
  * `engine=go-native` 時に `magick/pdftoppm` chain を使わない。

#### [MODIFY] `features/entext/internal/pdftoimage/naming_test.go`
* **Description**: `page_sheet_names` 不足時 `NN_page.ext` へフォールバックする境界ケースを拡張。
* **Technical Design**:
  * sidecar あり/なし/件数不足のテーブル追加。
* **Logic**:
  * 命名規約の一貫性を担保する。

#### [NEW] `features/entext/internal/pdftoimage/backend_go_fitz.go`
* **Description**: go-fitz 直接レンダリング backend を実装。
* **Technical Design**:
  * ```go
    type GoFitzBackend struct {
      render func(pdfPath string, dpi int, format string) ([]pdfnative.RenderedPage, error)
    }
    func (b *GoFitzBackend) Convert(ctx context.Context, inputPDF, outputDir, format string, dpi int, sm *sheetmap.SheetMap) ([]string, error)
    ```
* **Logic**:
  * RenderPages -> sidecar命名 -> ファイル保存

#### [MODIFY] `features/entext/internal/pdftoimage/service.go`
* **Description**: `engine=go-native` で `GoFitzBackend` を利用するよう修正（暫定 `NewMagickBackendWithDPI` を撤去）。
* **Technical Design**:
  * `ConvertWithOptions(..., engine, dpi, sheetMapPath)`
* **Logic**:
  * go-native: `GoFitzBackend` のみ
  * legacy: 既存 chain

#### [MODIFY] `features/entext/internal/pdftoimage/backend_magick.go`
* **Description**: 暫定 `NewMagickBackendWithDPI` を削除し、legacy 専用 backend として単純化。
* **Technical Design**:
  * `NewMagickBackend()` のみ維持。
* **Logic**:
  * go-native経路との混同を防ぐ。

### `features/entext/cmd`

#### [NEW] `features/entext/cmd/excel-to-pdf/main_test.go`
* **Description**: `--sheets` 不正入力時に exit code `2` を返す CLI 契約テストを追加。
* **Technical Design**:
  * コマンド実行ヘルパー経由で stderr と終了コードを検証。
* **Logic**:
  * validation error を runtime error (`1`) に落とさない。

#### [NEW] `features/entext/cmd/pdf-to-image/main_test.go`
* **Description**: `--format` / `--engine` 不正値の exit code 契約を追加。
* **Technical Design**:
  * `gif` / `unknown-engine` を入力するケース。
* **Logic**:
  * 不正引数で `2`、実行時失敗で `1` を維持。

#### [MODIFY] `features/entext/cmd/excel-to-pdf/main.go`
* **Description**: validation 判定で直接 `os.Exit(2)` している経路を `exitcode` に集約し、二重終了判定を排除。
* **Technical Design**:
  * `exitcode.FromError()` を利用。
* **Logic**:
  * error種別に一貫した終了コードを返す。

#### [MODIFY] `features/entext/cmd/pdf-to-image/main.go`
* **Description**: 上記と同様に終了コード経路を整理。
* **Technical Design**:
  * validation/runtime の分岐を helper 化。
* **Logic**:
  * E2E の終了コード期待値を安定化。

### `features/entext` Public API

#### [NEW] `features/entext/entext_gonative_completion_test.go`
* **Description**: options API の最終契約（engine/dpi/sheets/sheet-map）を固定。
* **Technical Design**:
  * `ConvertExcelToPDFWithOptions` と `ConvertPDFToImageWithOptions` の validation test 追加。
* **Logic**:
  * 不正 `engine/format/sheets` が `ValidationError` になることを確認。

#### [MODIFY] `features/entext/entext.go`
* **Description**: API options 経路を最終実装へ統一。
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
  * go-native 完了仕様へ API を追従。

### `tests` (Root Integration/E2E)

#### [MODIFY] `tests/e2e_backend_pipeline_test.go`
* **Description**: go-native 完了要件を E2E で担保する。
* **Technical Design**:
  * 追加/更新ケース:
    * `TestE2EExcelToPDF_GoNative_WritesRealSheetMap`
    * `TestE2EExcelToPDF_GoNative_SheetsSubset`
    * `TestE2EPDFToImage_GoNative_DPIAffectsSize`
    * `TestE2EPDFToImage_GoNative_UsesSheetNames`
    * `TestE2EExcelToPDF_InvalidSheets_ExitCode2`
* **Logic**:
  * 手動確認を排除し、実ファイル生成と契約を自動検証する。

#### [MODIFY] `tests/common_backend_selection_test.go`
* **Description**: `go-native` と `legacy` の経路差分を追加検証。
* **Technical Design**:
  * `engine` 指定ごとの error message / behavior を比較。
* **Logic**:
  * go-native で legacy backend 呼び出しを迂回しないことを保証。

## Step-by-Step Implementation Guide

1. **TDD Red: sidecar/compat/sheetsの契約固め**
   * Edit `features/entext/internal/common/sheetmap/read_compat_test.go` and `types_test.go` to define sidecar expansion and version compatibility.
   * Edit `features/entext/internal/exceltopdf/sheets_parser_test.go` to add edge cases for `--sheets`.

2. **TDD Red: go-ole/go-fitz 経路固定**
   * Edit `features/entext/internal/exceltopdf/backend_go_native_windows_test.go` to define real sheet extraction and failure tracking behavior.
   * Edit `features/entext/internal/pdfnative/merge_render_test.go` and `features/entext/internal/pdftoimage/backend_go_fitz_test.go` to enforce go-fitz rendering path and DPI effect.

3. **Sidecar 互換処理の実装**
   * Edit `features/entext/internal/common/sheetmap/read_compat.go`.
   * Update `types.go`/`write.go` as needed for compatibility and strict field semantics.

4. **Excel go-native 完了実装**
   * Edit `features/entext/internal/exceltopdf/backend_go_native_windows.go` to implement direct go-ole COM workflow.
   * Keep `backend_go_native_stub.go` for non-Windows explicit failure.
   * Edit `features/entext/internal/exceltopdf/service.go` to ensure go-native path does not fallback to legacy.

5. **PDF go-native 完了実装**
   * Add `features/entext/internal/pdfnative/merge_render.go`.
   * Add `features/entext/internal/pdftoimage/backend_go_fitz.go`.
   * Edit `features/entext/internal/pdftoimage/service.go` to switch go-native path from magick to go-fitz.
   * Remove temporary `NewMagickBackendWithDPI` by editing `backend_magick.go`.

6. **CLI 契約修正**
   * Add tests in `features/entext/cmd/excel-to-pdf/main_test.go` and `features/entext/cmd/pdf-to-image/main_test.go`.
   * Edit `main.go` files to unify validation/runtime exit behavior (`2` vs `1`).

7. **公開 API の整合**
   * Add `features/entext/entext_gonative_completion_test.go`.
   * Edit `features/entext/entext.go`, `excelpdf/excelpdf.go`, `pdfimage/pdfimage.go` for final go-native contract.

8. **E2E テスト実装（必須）**
   * Edit `tests/e2e_backend_pipeline_test.go` and `tests/common_backend_selection_test.go` with go-native completion scenarios.
   * Ensure no `t.Skip*` usage; environment prerequisites absence must fail explicitly.

9. **ドキュメント更新**
   * Edit `README.md` to document Windows/Excel prerequisite and go-fitz CGO requirements.
   * Edit `prompts/phases/000-foundation/branches/main/ideas/004-True-GoNative-Completion-GoOLE-GoFitz.md` to reflect finalized flags and contracts.

10. **Verification Plan 実行**
   * Run full build first, then scoped integration tests.

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   ```bash
   ./scripts/process/build.sh
   ```
   * **Log Verification**: `sheetmap`, `exceltopdf`, `pdfnative`, `pdftoimage`, `entext_gonative_completion` の新規/更新テストが PASS すること。

2. **Integration Tests (common)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "GoNative|SheetMap|Sanitize|InvalidSheets|DPI|Engine|Backend|PublicAPI"
   ```
   * **Log Verification**: sidecar 実データ、subset sheets、go-native DPI反映、引数エラー exit code `2`、legacy差分が確認できること。

3. **Integration Tests (llm / chain regression)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "llm" --specify "pipeline_chain|image_to_markdown|session"
   ```
   * **Log Verification**: upstream 変更後も `image-to-markdown` の回帰が無いこと。

4. **E2E Tests (新規/追加)**:
   `tests/` の E2E で go-native 完了要件を検証する（手動確認の代替禁止）。

   #### [MODIFY] `tests/e2e_backend_pipeline_test.go`(file://tests/e2e_backend_pipeline_test.go)
   * **テストケース**:
     * `TestE2EExcelToPDF_GoNative_WritesRealSheetMap`
     * `TestE2EExcelToPDF_GoNative_SheetsSubset`
     * `TestE2EPDFToImage_GoNative_DPIAffectsSize`
     * `TestE2EPDFToImage_GoNative_UsesSheetNames`
     * `TestE2EExcelToPDF_InvalidSheets_ExitCode2`
   * **検証ポイント**:
     * sidecar に実シート名・実ページ数が入る。
     * dpi差で出力寸法が変わる。
     * 引数エラーが exit code `2` になる。

### Test Item Design (Bottom-Up)

1. **C (leaf)**: `sheetmap` 互換読込、`--sheets` parser、go-fitz render、sanitize命名。
2. **B (middle)**: `exceltopdf`/`pdftoimage` service の go-native/legacy 分岐と sidecar 連携。
3. **A (top)**: CLI exit code 契約、公開 API、root `tests/` E2E。

#### 観点チェックリスト適用
- 正常系: 全シート、部分シート、png/jpeg、200/300dpi。  
- 異常系: 不正 `--sheets`、不正 `--engine`、不正 `--format`。  
- 外部連携: Excel COM / go-fitz / ファイル出力。  
- データ一貫性: sidecar と画像命名の一致。  
- 状態遷移: `go-native` と `legacy` の経路分離。  
- 設定反映: `--engine`, `--sheets`, `--dpi`, `--sheet-map`。  
- 副作用: sidecar/画像の残存確認、リソース解放。  

#### テスト項目セルフレビュー（§11.4）
- **網羅性**: 仕様の必須要件1-10を leaf->middle->top で網羅。  
- **証拠十分性**: 実ファイル内容（JSON/画像寸法/命名）を検証し、ログ確認のみで終わらない。  
- **迂回排除**: go-native が magick/pdftoppm へ迂回しないことをテストで固定。  
- **依存整合**: 末端テスト通過後に統合/E2Eへ進む構成。  

### Post-Test Comprehensive Verdict Plan

全テスト完了後、`testing-rules` §12 の 7項目（スキップ、部分エラー、迂回成功、誤設定、順序依存、カバレッジ、外部状態）をチェック表で評価し、`✅/⚠️/❌` 判定を記録する。`⚠️` 以上の際は、OS制約（Windows/Excel導入）と CGO 前提を明示し、再検証条件を添える。

## Documentation

`prompts/specifications` 配下ドキュメントを確認し、本計画の影響範囲を更新する。

#### [MODIFY] `README.md`(file://README.md)
* **更新内容**: go-native の実要件（Windows + Excel + CGO/go-fitz）と `--engine/--sheets/--dpi/--sheet-map` の確定挙動を追記。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/004-True-GoNative-Completion-GoOLE-GoFitz.md`(file://prompts/phases/000-foundation/branches/main/ideas/004-True-GoNative-Completion-GoOLE-GoFitz.md)
* **更新内容**: 実装確定後の例外仕様（部分失敗時の扱い、version互換読込、制約）を反映。
