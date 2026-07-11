# 002-Conversion-Backend-Selection-And-Fallback

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/002-Conversion-Backend-Selection-And-Fallback.md`

## Goal Description
`excel-to-pdf` と `pdf-to-image` にバックエンド選択 (`--backend`) と `auto` フォールバック機構を実装し、利用不可環境でも「試行順・失敗理由を可視化したエラー」で正しく終了できる状態にする。検証時はツールチェーン (`xlsx -> pdf -> image -> markdown`) を `entext` コマンドのみで実行し、ツール外代替処理を成功扱いしない。

## Progress
- [x] 1. TDD Red (backend core tests)
- [x] 2. 共通エラー型実装
- [x] 3. Excel backends 実装
- [x] 4. PDF backends 実装
- [x] 5. CLI/API 反映
- [/] 6. Integration/E2E tests 追加
- [x] 7. README 更新
- [x] 8. Verification 実行

## User Review Required
1. `excel-to-pdf` の `auto` 優先順（Windows: `excel-com` -> `libreoffice`、非Windows: `libreoffice`）で確定してよいか。
2. `pdf-to-image` の `auto` 優先順（`pdftoppm` -> `magick`）で確定してよいか。
3. `excel-com` 実装を PowerShell 経由で行う方針（Excel COM 呼び出し）でよいか。
4. `magick` 実装を PNG/JPG 出力の共通 backend として採用してよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| excel-to-pdf 複数 backend (`libreoffice`, `excel-com`) | Proposed Changes > `internal/exceltopdf/backend_*.go` |
| pdf-to-image 複数 backend (`pdftoppm`, `magick`) | Proposed Changes > `internal/pdftoimage/backend_*.go` |
| `--backend` オプション追加 | Proposed Changes > `cmd/excel-to-pdf/main.go`, `cmd/pdf-to-image/main.go` |
| 指定 backend はフォールバックしない | Proposed Changes > backend orchestrator (`resolver.go`) |
| `auto` は優先順試行 + 全失敗でエラー | Proposed Changes > `resolver.go`, `errors.go` |
| 失敗時に試行一覧と理由を出力 | Proposed Changes > `errors.go`, CLI エラーフォーマット |
| ツールチェーン検証を entext コマンドのみで実施 | Verification Plan > Integration + E2E Tests |
| ad-hoc 処理を成功扱いしない | Verification Plan / Documentation に証跡ルールを明記 |
| `--backend --help` に前提ソフト明記 | Proposed Changes > CLI flags description + README |
| DEBUG ログで選択/試行結果追跡 | Proposed Changes > backend orchestrator logging |

## Proposed Changes

### `features/entext` Backend Core

#### [NEW] `features/entext/internal/exceltopdf/backend_contract_test.go`
*   **Description**: `excel-to-pdf` backend 契約（Name/Convert/availability）を固定するテスト。
*   **Technical Design**:
    *   ```go
        type Backend interface {
            Name() string
            Convert(ctx context.Context, inputPath string, outputDir string) (string, error)
        }
        ```
*   **Logic**:
    *   backend 名称衝突防止。
    *   不正入力時の validation 統一。

#### [NEW] `features/entext/internal/exceltopdf/resolver_test.go`
*   **Description**: `--backend` 指定時と `auto` 時の分岐・試行順・集約エラーをテーブル駆動で検証。
*   **Technical Design**:
    *   ```go
        func ResolveBackendChain(mode string, osName string) ([]Backend, error)
        func RunWithFallback(ctx context.Context, chain []Backend, input string, out string) (string, error)
        ```
*   **Logic**:
    *   指定 mode では chain 長が 1。
    *   `auto` では OS に応じた優先順。
    *   全失敗時に `BackendAggregateError` を返す。

#### [NEW] `features/entext/internal/pdftoimage/backend_contract_test.go`
*   **Description**: `pdf-to-image` backend 契約を固定。
*   **Technical Design**:
    *   ```go
        type Backend interface {
            Name() string
            Convert(ctx context.Context, inputPDF string, outputDir string, format string) ([]string, error)
        }
        ```
*   **Logic**:
    *   format 検証の統一。
    *   出力順序の保証。

#### [NEW] `features/entext/internal/pdftoimage/resolver_test.go`
*   **Description**: `pdftoppm` / `magick` の選択ロジック・フォールバック挙動を固定。
*   **Technical Design**:
    *   `ResolveBackendChain(mode string)` + fallback executor
*   **Logic**:
    *   指定 backend は単独実行。
    *   `auto` は順序試行。
    *   全失敗時に集約エラー。

#### [NEW] `features/entext/internal/common/backend/errors.go`
*   **Description**: バックエンド試行失敗を共通構造で表現するエラー型。
*   **Technical Design**:
    *   ```go
        type BackendAttemptError struct { Backend string; Err error }
        type BackendAggregateError struct { Attempts []BackendAttemptError }
        ```
*   **Logic**:
    *   `Error()` で `tried: backend(reason)` 形式を構築。
    *   `errors.As` 可能にする。

#### [NEW] `features/entext/internal/common/backend/errors_test.go`
*   **Description**: 集約エラーメッセージのフォーマットと unwrap 挙動を検証。
*   **Logic**:
    *   失敗一覧が順序付きで含まれることを確認。

### `features/entext` Excel-to-PDF

#### [NEW] `features/entext/internal/exceltopdf/backend_libreoffice.go`
*   **Description**: 既存 LibreOffice 変換を backend 実装として分離。
*   **Technical Design**:
    *   `type LibreOfficeBackend struct{}`
*   **Logic**:
    *   実行コマンドと出力規約は現行互換。

#### [NEW] `features/entext/internal/exceltopdf/backend_excel_com_windows.go`
*   **Description**: Windows 向け Excel COM backend 実装。
*   **Technical Design**:
    *   `type ExcelCOMBackend struct{}`
    *   PowerShell で COM オブジェクト作成 -> `ExportAsFixedFormat` 実行。
*   **Logic**:
    *   Windows 以外では unsupported を返す。
    *   Excel 未導入時の明確なエラー返却。

#### [NEW] `features/entext/internal/exceltopdf/resolver.go`
*   **Description**: backend mode (`auto|libreoffice|excel-com`) を解決する orchestrator。
*   **Technical Design**:
    *   ```go
        type BackendMode string
        const (ModeAuto ModeLibreOffice ModeExcelCOM)
        func ResolveChain(mode BackendMode, goos string) []Backend
        ```
*   **Logic**:
    *   Windows auto: ExcelCOM -> LibreOffice
    *   非Windows auto: LibreOffice
    *   指定 mode は単独 chain

#### [MODIFY] `features/entext/internal/exceltopdf/service.go`
*   **Description**: 単一 backend 呼び出しから orchestrator 呼び出しへ変更。
*   **Technical Design**:
    *   `Convert(..., backendMode string)` へ拡張
*   **Logic**:
    *   fallback executor で試行・ログ・集約エラー対応。

### `features/entext` PDF-to-Image

#### [NEW] `features/entext/internal/pdftoimage/backend_pdftoppm.go`
*   **Description**: 既存 pdftoppm 実装を backend 分離。

#### [NEW] `features/entext/internal/pdftoimage/backend_magick.go`
*   **Description**: `magick` backend 実装を追加。
*   **Technical Design**:
    *   `magick convert -density ...` 形式（環境に応じた実行形式を吸収）
*   **Logic**:
    *   中間出力を既存 `<basename>_<nnn>.<ext>` 規約へ正規化。

#### [NEW] `features/entext/internal/pdftoimage/resolver.go`
*   **Description**: backend mode (`auto|pdftoppm|magick`) 解決。
*   **Logic**:
    *   auto: pdftoppm -> magick

#### [MODIFY] `features/entext/internal/pdftoimage/service.go`
*   **Description**: 単一 backend 実行から fallback orchestrator 実行へ変更。
*   **Technical Design**:
    *   `Convert(..., format string, backendMode string)` へ拡張

### CLI / Public API

#### [MODIFY] `features/entext/cmd/excel-to-pdf/main.go`
*   **Description**: `--backend` フラグ追加と backend mode 受け渡し。
*   **Technical Design**:
    *   `--backend` default `auto`
    *   help: `libreoffice|excel-com|auto`
*   **Logic**:
    *   指定 mode 時は即失敗（フォールバック無し）。

#### [MODIFY] `features/entext/cmd/pdf-to-image/main.go`
*   **Description**: `--backend` フラグ追加。
*   **Technical Design**:
    *   help: `pdftoppm|magick|auto`

#### [MODIFY] `features/entext/entext.go`
*   **Description**: 公開 API に backend mode 引数または Option を追加。
*   **Technical Design**:
    *   `ConvertExcelToPDF(..., backend string)`
    *   `ConvertPDFToImage(..., format string, backend string)`
*   **Logic**:
    *   CLI と API で同一 backend 選択挙動を保証。

#### [NEW] `features/entext/entext_backend_test.go`
*   **Description**: 公開 API で backend 指定/auto の差を検証。

### Integration / E2E Tests

#### [NEW] `features/entext/tests/common_backend_selection_test.go`
*   **Description**: backend 指定・auto・集約エラーを統合レベルで検証。
*   **テストケース**:
    *   `TestExcelToPDF_BackendSpecified_NoFallback`
    *   `TestExcelToPDF_AutoFallback_Order`
    *   `TestPDFToImage_BackendSpecified_NoFallback`
    *   `TestPDFToImage_AutoFallback_Order`
    *   `TestBackendAggregateError_ContainsAttempts`

#### [NEW] `features/entext/tests/e2e_entext_pipeline_backend_test.go`
*   **Description**: `entext` コマンドのみでチェーンを検証する E2E 相当テスト。
*   **テストケース**:
    *   `TestPipeline_AutoBackend_XLSXToMarkdown`
    *   `TestPipeline_BackendUnavailable_ShouldFailWithAttempts`

### Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **Description**: backend 指定方法・auto ルール・前提ソフト・失敗時エラー例を追記。
*   **更新内容**:
    *   `--backend` examples
    *   required external dependencies
    *   aggregate error sample

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/002-Conversion-Backend-Selection-And-Fallback.md`(file://prompts/phases/000-foundation/branches/main/ideas/002-Conversion-Backend-Selection-And-Fallback.md)
*   **更新内容**: 実装確定後の backend mode 名称/優先順を確定値で反映。

## Step-by-Step Implementation Guide

1. **TDD Red (backend core tests)**:
   * Edit `internal/exceltopdf/*_test.go` and `internal/pdftoimage/*_test.go` to define backend selection/fallback expectations.
   * Add `internal/common/backend/errors_test.go`.
2. **共通エラー型実装**:
   * Edit `internal/common/backend/errors.go` with aggregate error structure.
3. **Excel backends 実装**:
   * Add `backend_libreoffice.go`, `backend_excel_com_windows.go`, `resolver.go`.
   * Update `internal/exceltopdf/service.go` to use resolver and fallback runner.
4. **PDF backends 実装**:
   * Add `backend_pdftoppm.go`, `backend_magick.go`, `resolver.go`.
   * Update `internal/pdftoimage/service.go` for mode-based execution.
5. **CLI/API 反映**:
   * Edit `cmd/excel-to-pdf/main.go`, `cmd/pdf-to-image/main.go` with `--backend`.
   * Edit `entext.go` API signatures to include backend mode.
6. **Integration/E2E tests 追加**:
   * Add `tests/common_backend_selection_test.go`.
   * Add `tests/e2e_entext_pipeline_backend_test.go`.
7. **README 更新**:
   * Add backend selection docs and failure diagnostics.
8. **Verification 実行**:
   * Run build first, then scoped integration tests.

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   ```bash
   ./scripts/process/build.sh
   ```
   * **Log Verification**: backend resolver tests pass; no compile regression in existing 3 CLI commands.

2. **Integration Tests (common)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "excel_to_pdf_backend|pdf_to_image_backend|backend_auto|backend_error_aggregate"
   ```
   * **Log Verification**: specified backend no-fallback behavior; auto fallback order; aggregated attempts list in errors.

3. **Integration Tests (llm)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "llm" --specify "image_to_markdown|pipeline_chain|session_log"
   ```
   * **Log Verification**: image-to-markdown still works when upstream image conversion succeeds.

4. **E2E Tests (新規/追加)**:
   `entext` CLI のみでパイプラインを検証するテストを `tests/` 配下に追加する。

   #### [NEW] `features/entext/tests/e2e_entext_pipeline_backend_test.go`(file://features/entext/tests/e2e_entext_pipeline_backend_test.go)
   * **テストケース**:
     * `TestPipeline_AutoBackend_XLSXToMarkdown`
     * `TestPipeline_BackendUnavailable_ShouldFailWithAttempts`
   * **検証ポイント**:
     * ツール外代替処理なしで変換チェーン成功/失敗を判定できる。
     * 失敗時に試行 backend と理由がログ/エラーに残る。

### Test Item Design (Bottom-Up)

1. **C (leaf)**: backend implementations + aggregate error formatter.
2. **B (middle)**: service orchestration (`mode -> chain -> run`).
3. **A (top)**: CLI flag wiring + end-to-end conversion chain.

#### 観点チェックリスト適用
- 正常系: backend 指定成功、auto 成功。  
- 異常系: backend 未導入、指定 backend 不一致、全失敗。  
- 外部連携: COM, libreoffice, pdftoppm, magick の呼び出し。  
- データ一貫性: 生成ファイル名規約、順序、artifact paths。  
- 状態遷移: fallback chain の試行と停止条件。  
- 設定反映: `--backend` / API backend 引数。  
- 副作用: 中間ファイル・ログ・session 出力。

#### テスト項目セルフレビュー（§11.4）
- **網羅性**: 指定/auto/失敗集約/チェーン検証を包括。  
- **証拠十分性**: エラー文言と試行履歴まで検証対象化。  
- **迂回排除**: Python等の外部代替処理をテスト手順から排除。  
- **依存整合**: backend unit -> service -> CLI/E2E 順に確認。

### Post-Test Comprehensive Verdict Plan

全テスト完了後、`testing-rules` §12 の 7 項目（スキップ、部分エラー、迂回成功、誤設定、順序依存、カバレッジ、外部状態）をチェック表で記録し、`✅/⚠️/❌` を判定する。`⚠️` 以上は不足条件（未導入ツール、環境依存）を明記する。
