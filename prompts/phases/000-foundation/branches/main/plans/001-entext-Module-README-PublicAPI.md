# 001-entext-Module-README-PublicAPI

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/001-entext-Module-README-PublicAPI.md`

## Goal Description
`features/entext` を外部公開可能な `entext` モジュールとして整備し、`github.com/axsh/entext` で `go get` 可能な形にする。あわせてルート `README.md` を追加し、CLI と Go API の両利用導線を明示する。CLI 実装は維持しつつ、外部組み込み向けの安定 API を `github.com/axsh/entext` で提供する。

## User Review Required
1. 公開 import パス方針:
   - ルート package `github.com/axsh/entext` を正とする方針で良いか。
2. モジュール配置方針:
   - 物理ディレクトリも `features/entext` へ変更する方針で良いか。
3. API 安定面:
   - v1 では `github.com/axsh/entext` の公開シグネチャ固定、`internal/` は非互換変更許容という運用で良いか。
4. Go バージョン:
   - `arctic-tern` 依存都合で `go 1.26.4+` が必要な点を README に明記する方針で良いか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 名称を `entext` に統一 | Proposed Changes > `README.md`, `plans/`, `ideas/` の文言更新 |
| `features/entext/go.mod` を `module github.com/axsh/entext` 化 | Proposed Changes > `features/entext/go.mod` |
| 旧 import path を全更新 | Proposed Changes > `features/entext/**/*.go` |
| 命名変更後も既存3CLIを維持 | Proposed Changes > `cmd/*`, `internal/*`（参照整合のみ） |
| `arctic-tern` 利用を維持 | Proposed Changes > `internal/imagetomd/tern/client.go` の依存維持 |
| ルート `README.md` 追加 | Proposed Changes > `README.md` |
| `go get github.com/axsh/entext` で組み込める公開 API | Proposed Changes > `features/entext/*.go`, `features/entext/*` |
| 公開 API を module root に安定化 | Proposed Changes > `features/entext` root package |
| `go mod tidy` + build で依存成立 | Verification Plan > Automated Verification |

## Proposed Changes

### `features/entext` (Go module + runtime)

#### [NEW] `features/entext/entext_test.go`
*   **Description**: 公開 API の型契約（Input/Output/Error）を先に固定する。
*   **Technical Design**:
    *   ```go
        type FileJob struct {
            InputPath string
            OutputDir string
        }
        type FileArtifact struct {
            Paths []string
        }
        type ValidationError struct{ Err error }
        ```
*   **Logic**:
    *   `errors.Is/As` で判定可能なエラー設計を検証。
    *   `Paths` の順序保証（入力順/ページ順）を検証。

#### [NEW] `features/entext/excelpdf/excelpdf_test.go`
*   **Description**: 公開 `excelpdf` API の最小呼び出し契約を固定。
*   **Technical Design**:
    *   ```go
        type Converter interface {
            Convert(ctx context.Context, job entext.FileJob) (entext.FileArtifact, error)
        }
        func New(opts ...Option) Converter
        ```
*   **Logic**:
    *   `-i` 相当の単一入力利用時に `[]Paths{pdf}` を返す。
    *   入力不正時に公開 `ValidationError` を返す。

#### [NEW] `features/entext/pdfimage/pdfimage_test.go`
*   **Description**: `png/jpg` と連番規約 (`_nnn`) の公開 API 契約を固定。
*   **Technical Design**:
    *   ```go
        type Config struct { Format string }
        func New(opts ...Option) Converter
        ```
*   **Logic**:
    *   `format=png|jpg` のみ許可、その他は validation error。
    *   出力順がページ順であることを検証。

#### [NEW] `features/entext/imagemd/imagemd_test.go`
*   **Description**: `arctic-tern` 利用を前提に `imagemd` 公開 API 契約を固定。
*   **Technical Design**:
    *   ```go
        type Config struct {
            ServerURL       string
            Agent           string
            Model           string
            RefPatterns     []string
            StrictGapJudge  bool
            SaveQuestionLog bool
            RoundSleepMS    int
            PhaseSleepMS    int
        }
        func New(opts ...Option) Converter
        ```
*   **Logic**:
    *   `WithServer`, `WithModel`, `WithRefPatterns`, `WithStrictGapJudge` 適用を検証。
    *   互換モード/改善モードの切替を検証。

#### [NEW] `features/entext/tests/common_entext_api_test.go`
*   **Description**: 公開 API を外部利用視点で統合検証する。
*   **Technical Design**:
    *   `//go:build integration`
    *   `TestEnttextAPI_ExcelPDF`, `TestEnttextAPI_PDFImage`, `TestEnttextAPI_ImageMD`
*   **Logic**:
    *   `github.com/axsh/entext` および `github.com/axsh/entext/*` を import して呼び出せることを検証。
    *   CLI 非依存で変換処理へ到達できることを確認。

#### [MODIFY] `features/entext/go.mod`
*   **Description**: module path を `github.com/axsh/entext` に変更し、公開導線を一致させる。
*   **Technical Design**:
    *   ```go
        module github.com/axsh/entext
        go 1.26.4
        ```
*   **Logic**:
    *   `arctic-tern` 依存を維持。
    *   `go mod tidy` 後に import 解決可能な状態を保証。

#### [MODIFY] `features/entext/internal/**/*.go`
*   **Description**: 旧 import path を `github.com/axsh/entext/...` へ更新。
*   **Technical Design**:
    *   旧: `github.com/axsh/tokotachi/features/doc-convert/internal/...` または `github.com/axsh/entext/internal/...`（物理パス変更前）
    *   新: `github.com/axsh/entext/internal/...`
*   **Logic**:
    *   実装ロジックは変更せず import 参照のみ更新。
    *   既存テストを壊さないことを優先。

#### [NEW] `features/entext/entext.go`
*   **Description**: 公開共通型と公開エラー型を定義。
*   **Technical Design**:
    *   ```go
        package entext
        type FileJob struct { InputPath, OutputDir string }
        type BatchJob struct { Inputs []string; OutputDir string }
        type FileArtifact struct { Paths []string; SessionLogPath string }
        type ValidationError struct { Err error }
        func (e *ValidationError) Error() string
        func (e *ValidationError) Unwrap() error
        ```
*   **Logic**:
    *   CLI 契約と同じく「生成物パス中心」の返却を維持。
    *   `errors.As(err, *ValidationError)` を外部から利用可能にする。

#### [NEW] `features/entext/errors.go`
*   **Description**: 公開 sentinel error と helper を定義。
*   **Technical Design**:
    *   ```go
        var (
            ErrInvalidArgs = errors.New("invalid arguments")
            ErrInputRequired = errors.New("input is required")
        )
        func IsValidation(err error) bool
        ```
*   **Logic**:
    *   external caller が validation/runtime を区別できるようにする。

#### [NEW] `features/entext/excelpdf/excelpdf.go`
*   **Description**: `internal/exceltopdf` をラップする公開 API。
*   **Technical Design**:
    *   ```go
        package excelpdf
        type Converter struct { svc *exceltopdf.Service }
        func New(opts ...Option) *Converter
        func (c *Converter) Convert(ctx context.Context, job entext.FileJob) (entext.FileArtifact, error)
        ```
*   **Logic**:
    *   入力検証 -> internal service 呼び出し -> `FileArtifact` 返却。

#### [NEW] `features/entext/pdfimage/pdfimage.go`
*   **Description**: `internal/pdftoimage` をラップする公開 API。
*   **Technical Design**:
    *   ```go
        package pdfimage
        type Converter struct { svc *pdftoimage.Service; cfg Config }
        func (c *Converter) Convert(ctx context.Context, job entext.FileJob) (entext.FileArtifact, error)
        ```
*   **Logic**:
    *   format 検証と連番出力契約を維持。

#### [NEW] `features/entext/imagemd/imagemd.go`
*   **Description**: `internal/imagetomd/*` をラップする公開 API。
*   **Technical Design**:
    *   ```go
        package imagemd
        type Converter struct { cfg Config }
        func New(opts ...Option) *Converter
        func (c *Converter) Convert(ctx context.Context, job entext.FileJob) (entext.FileArtifact, error)
        ```
*   **Logic**:
    *   `arctic-tern` クライアントを生成し analyzer を呼び出す。
    *   markdown 保存 + session 保存 + artifact 返却。

#### [MODIFY] `features/entext/entext.go`
*   **Description**: module root package として subpackage コンストラクタを公開。
*   **Technical Design**:
    *   ```go
        package entext
        func NewExcelPDF(opts ...excelpdf.Option) *excelpdf.Converter
        func NewPDFImage(opts ...pdfimage.Option) *pdfimage.Converter
        func NewImageMD(opts ...imagemd.Option) *imagemd.Converter
        ```
*   **Logic**:
    *   外部利用者の import 深度を下げる。

#### [MODIFY] `features/entext/cmd/**/*.go`
*   **Description**: CLI が `entext` root package / subpackages を利用する構成に変更し、実装重複を排除。
*   **Technical Design**:
    *   `cmd/excel-to-pdf` -> `excelpdf.Converter`
    *   `cmd/pdf-to-image` -> `pdfimage.Converter`
    *   `cmd/image-to-markdown` -> `imagemd.Converter`
*   **Logic**:
    *   CLI は入出力解決と表示責務に限定。
    *   変換ロジックは公開 API 側に集約。

### Repository Root Docs

#### [NEW] `README.md`(file://README.md)
*   **Description**: ルート導線を `entext` 前提に更新し、CLI + Go API の利用方法を提供。
*   **Technical Design**:
    *   Sections:
      - Overview
      - Installation (`go get github.com/axsh/entext`)
      - CLI Usage
      - Go API Usage
      - Requirements (Go 1.26.4+)
*   **Logic**:
    *   外部利用者が README だけで導入できる状態にする。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/001-entext-Module-README-PublicAPI.md`
*   **Description**: 実装後の確定仕様（API シグネチャ・README 内容）に合わせて差分反映。
*   **Technical Design**:
    *   API セクションに確定シグネチャを追記。
*   **Logic**:
    *   仕様と実装の乖離を残さない。

## Step-by-Step Implementation Guide

1. **API 契約テスト先行 (Red)**:
   * Edit `features/entext/entext_test.go`, `excelpdf_test.go`, `pdfimage_test.go`, `imagemd_test.go` to define expected public signatures and behavior.
   * Add `features/entext/tests/common_entext_api_test.go` integration skeleton.
2. **Module Path 変更**:
   * Rename `features/doc-convert` to `features/entext`, then edit `features/entext/go.mod` to `module github.com/axsh/entext`.
   * Run import updates in `features/entext/**/*.go`.
3. **公開 API 基盤実装 (Green)**:
   * Edit `features/entext/entext.go` and `errors.go`.
   * Implement validation error bridging from internal errors.
4. **機能別公開 API 実装**:
   * Edit `features/entext/excelpdf/excelpdf.go`, `features/entext/pdfimage/pdfimage.go`, `features/entext/imagemd/imagemd.go`.
   * Keep `arctic-tern` path through `internal/imagetomd/tern`.
5. **Facade 実装**:
   * Edit `features/entext/entext.go` to expose constructors.
6. **CLI 連携リファクタ**:
   * Edit `features/entext/cmd/excel-to-pdf/main.go`, `features/entext/cmd/pdf-to-image/main.go`, `features/entext/cmd/image-to-markdown/main.go` to call public APIs.
7. **README 追加**:
   * Create `README.md` with install/CLI/API examples and version requirements.
8. **仕様差分反映**:
   * Update `ideas/001-entext-Module-README-PublicAPI.md` with finalized signatures if needed.
9. **Verification Plan 実行**:
   * Run build first, then scoped integration tests, then API integration tests.

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   Run full build and unit tests.
   ```bash
   ./scripts/process/build.sh
   ```
   * **Log Verification**: `features/entext` の公開 API テストが通過し、import rewrite errors がないこと。

2. **Integration Tests (common)**:
   Verify CLI/API contracts related to naming and I/O.
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "entext_api|entext|stdin|output_mode|module"
   ```
   * **Log Verification**: module rename regressions absent, CLI contracts unchanged, public API callable from tests.

3. **Integration Tests (llm)**:
   Verify `imagemd` still works with `arctic-tern`.
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "llm" --specify "image_to_markdown|arctic_tern|session|strict_gap"
   ```
   * **Log Verification**: session creation, streaming response handling, session termination, and session log writes remain intact.

4. **E2E Tests (新規/追加)**:
   公開 API を外部利用者視点で呼び出す E2E 相当テストを `tests/` に追加する。

   #### [NEW] `features/entext/tests/e2e_entext_public_api_test.go`(file://features/entext/tests/e2e_entext_public_api_test.go)
   * **テストケース**:
     * `TestPublicAPI_ExcelToPDF_FromExternalImport`
     * `TestPublicAPI_PDFToImage_FromExternalImport`
     * `TestPublicAPI_ImageToMarkdown_FromExternalImport`
   * **検証ポイント**:
     * `github.com/axsh/entext` import で初期化・実行できる。
     * 返却 artifact のパス契約が期待通り。
     * CLI 依存なしで直接組み込み利用できる。

### Test Item Design (Bottom-Up)

1. **C (leaf)**: `entext` root package types, validation errors, option parsing.
2. **B (middle)**: `excelpdf/pdfimage/imagemd` converters with internal adapters.
3. **A (top)**: CLI callers + external-import style API integration tests.

#### 観点チェックリスト適用
- 正常系: 単一入力、複数入力、公開 API 呼び出し。  
- 異常系: 入力未指定、format 不正、server 未到達。  
- 外部連携: `arctic-tern`, LibreOffice, pdftoppm。  
- データ一貫性: artifact paths, session JSON outputs。  
- 状態遷移: classify -> phases -> markdown output。  
- 設定反映: module path / viper config / options。  
- 副作用: `_sessions` 保存、出力ディレクトリ作成。

#### テスト項目セルフレビュー（§11.4）
- **網羅性**: rename・README・public API・既存CLI維持のすべてを対象化。  
- **証拠十分性**: コンパイル通過だけでなく外部 import 実行経路を検証。  
- **迂回排除**: CLI 経由のみ成功する状態を防ぐため API 直呼びテストを必須化。  
- **依存整合**: leaf -> middle -> top の順で failure localization を可能化。

### Post-Test Comprehensive Verdict Plan

全テスト完了後、`testing-rules` §12 の 7 観点（スキップ、部分エラー、迂回成功、誤設定、順序依存、カバレッジ、外部状態）を表で記録し、`✅/⚠️/❌` 判定を残す。`⚠️` 以上の場合は未解決項目と再実行条件を明記する。

## Documentation

`prompts/specifications` 配下の文書を確認し、公開 module 名称や API 利用手順に影響する箇所を更新する。

#### [MODIFY] `README.md`(file://README.md)
* **更新内容**: 新規追加（entext 概要、go get、CLI/API examples、Go 要件）。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/001-entext-Module-README-PublicAPI.md`(file://prompts/phases/000-foundation/branches/main/ideas/001-entext-Module-README-PublicAPI.md)
* **更新内容**: 実装確定後の API シグネチャ差分反映。

## Execution Progress

- [x] Step 1: ルール読込と実装計画の確定
- [x] Step 2: `features/doc-convert` -> `features/entext` 物理リネーム
- [x] Step 3: `go.mod` の module path を `github.com/axsh/entext` へ更新
- [x] Step 4: 旧 import path を `features/entext/**/*.go` で更新
- [x] Step 5: 公開 root API (`github.com/axsh/entext`) を実装
- [x] Step 6: CLI を公開 API 利用にリファクタ
- [x] Step 7: ルート `README.md` を追加/更新（go get + API usage 記載）
- [x] Step 8: `./scripts/process/build.sh` 実行成功
- [/ ] Step 9: `integration_test.sh` の実行基盤整備（現環境では `tests/` が無くスキップ）
