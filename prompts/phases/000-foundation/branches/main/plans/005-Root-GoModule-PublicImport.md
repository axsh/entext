# 005-Root-GoModule-PublicImport

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/005-Root-GoModule-PublicImport.md`

## Goal Description

`github.com/axsh/entext` をリポジトリルートの正式 Go モジュールとして成立させ、外部プロジェクトから `import "github.com/axsh/entext"` / `go get github.com/axsh/entext` が実運用可能な構成へ移行する。あわせて、`features/entext` 前提のディレクトリ・ビルド・テスト経路をルートモジュール前提へ統一し、既存 CLI/API 機能の回帰を防ぐ。

## Execution Status

- [x] Step 1: TDD Red（`tests/external_import_e2e_test.go`, `tests/go.mod`, `tests/e2e_backend_pipeline_test.go`）
- [x] Step 2: Root Module Scaffold（root `go.mod` / `go.sum` / `entext_root_import_test.go`）
- [x] Step 3: コード配置移行（`cmd`, `internal`, `excelpdf`, `pdfimage`, `imagemd`, `entext.go`）
- [x] Step 4: CLI/API 回帰テスト更新（既存 CLI テスト移行 + root public compile contract）
- [x] Step 5: `scripts/process/build.sh` の root module 対応
- [x] Step 6: 旧モジュール残骸整理（`features/entext/go.mod` を削除）
- [x] Step 7: `README.md` 導線更新（`cmd` パスを root 前提へ修正）
- [x] Step 8: Verification Plan 実行（build + integration 指定3系統 実施）

## User Review Required

1. 既存 `features/entext` ディレクトリを最終的に削除（または空ディレクトリ化）してよいか。
2. 外部 import E2E の「`go get`」は、ネットワーク依存を避けるため `replace github.com/axsh/entext => <repo-root>` 付きの疑似外部モジュール方式で確定してよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. ルートに `go.mod` を配置し `module github.com/axsh/entext` を定義 | Proposed Changes > Root Module (`go.mod`, `go.sum`) |
| 2. `import "github.com/axsh/entext"` で公開 API 到達 | Proposed Changes > Public API Relocation (`entext.go`), External Import E2E |
| 3. `excelpdf` / `pdfimage` / `imagemd` 外部 import 可能 | Proposed Changes > Package Relocation (`excelpdf`, `pdfimage`, `imagemd`), `tests/external_import_e2e_test.go` |
| 4. `cmd/internal/...` をルート基準へ再配置 | Proposed Changes > Tree Migration (`cmd`, `internal`, package dirs) |
| 5. `build.sh` / `integration_test.sh` が新配置で成立 | Proposed Changes > `scripts/process/build.sh`, `tests/go.mod`, integration test updates |
| 6. `tests/go.mod` の `../features/entext` 依存除去 | Proposed Changes > `tests/go.mod`, external import helper |
| 7. README 導線を実配置へ一致 | Proposed Changes > `README.md` |
| 8. 既存機能仕様維持（3CLI + go-native + image-to-markdown） | Proposed Changes > Existing tests migration + regression test updates |
| 任意1. 過渡期 `go.work` / 互換レイヤ | **Not Implemented in this plan**（単一モジュール移行で不要化するため採用しない） |
| 任意2. 移動と機能変更の段階分離 | Step-by-Step Guide で `移動` と `振る舞い変更` を分離して実施 |

## Proposed Changes

### Root Module

#### [NEW] `go.mod`(file://go.mod)
* **Description**: ルートモジュール定義を新設し、公開 import パスを物理配置と一致させる。
* **Technical Design**:
  * `module github.com/axsh/entext`
  * 既存 `features/entext/go.mod` の require 群を継承する。
  * ```go
    module github.com/axsh/entext

    go 1.26.4

    require (
        github.com/axsh/arctic-tern v0.1.0
        github.com/go-ole/go-ole v1.3.0
        github.com/gen2brain/go-fitz v1.24.15
        // ...
    )
    ```
* **Logic**:
  * ルートで `go env GOMOD` が `.../entext/go.mod` を返す状態を作る。
  * 既存 API/CLI パッケージの import 解決をルート基準に切り替える。

#### [NEW] `go.sum`(file://go.sum)
* **Description**: ルートモジュールの依存ハッシュを管理する。
* **Technical Design**:
  * `go mod tidy` で生成される完全なチェックサム一覧を保持。
* **Logic**:
  * `build.sh` と integration 実行時に追加ダウンロードなしで再現可能にする。

### Public API / Package Relocation

#### [NEW] `entext_root_import_test.go`(file://entext_root_import_test.go)
* **Description**: ルート package `entext` の公開シグネチャが移行後も維持されることを固定する。
* **Technical Design**:
  * ```go
    func TestRootPublicTypesAndFunctionsCompile(t *testing.T) {
        var _ FileJob
        _, _ = ConvertExcelToPDFWithOptions(context.Background(), FileJob{}, ExcelPDFOptions{})
        _, _ = ConvertPDFToImageWithOptions(context.Background(), FileJob{}, "png", PDFImageOptions{})
    }
    ```
* **Logic**:
  * ルート package の export が存在し、コンパイル可能である事実を最小テストで保証する。

#### [MODIFY] `entext.go`(file://entext.go)
* **Description**: `features/entext/entext.go` からルートへ移設し、実装のエントリポイントを統一する。
* **Technical Design**:
  * 既存構造体定義をそのまま継承:
    * ```go
      type FileJob struct {
          InputPath string
          OutputDir string
      }
      type FileArtifact struct {
          Paths        []string
          SheetMapPath string
      }
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
      ```
  * 既存公開関数を保持:
    * `ConvertExcelToPDF`
    * `ConvertExcelToPDFWithBackend`
    * `ConvertExcelToPDFWithOptions`
    * `ConvertPDFToImage`
    * `ConvertPDFToImageWithBackend`
    * `ConvertPDFToImageWithOptions`
    * `ConvertImageToMarkdown`
* **Logic**:
  * シグネチャ互換を維持し、利用側コード変更を不要にする。

#### [MODIFY] `excelpdf/excelpdf.go`(file://excelpdf/excelpdf.go)
* **Description**: ルートモジュール前提へ import 経路を更新し、外部利用可能性を維持する。
* **Technical Design**:
  * `import "github.com/axsh/entext"` を前提に wrapper 実装を維持。
* **Logic**:
  * `github.com/axsh/entext/excelpdf` 単体 import が通ることを保証する。

#### [MODIFY] `pdfimage/pdfimage.go`(file://pdfimage/pdfimage.go)
* **Description**: 上記と同様に import 経路をルートモジュールへ合わせる。
* **Technical Design**:
  * `entext.ConvertPDFToImageWithOptions` 呼び出しの維持。
* **Logic**:
  * `github.com/axsh/entext/pdfimage` の公開 API 互換を保持する。

#### [MODIFY] `imagemd/imagemd.go`(file://imagemd/imagemd.go)
* **Description**: image-to-markdown wrapper の公開 import パスを維持する。
* **Technical Design**:
  * 既存 Option/Convert 経路を保持し、モジュール基準のみ更新。
* **Logic**:
  * 3rd party から `github.com/axsh/entext/imagemd` を直接利用可能にする。

### Tree Migration (`features/entext` -> root)

#### [NEW] `cmd/excel-to-pdf/main_test.go`(file://cmd/excel-to-pdf/main_test.go)
* **Description**: ルート配置移行後も CLI の validation exit code 契約が維持されることを固定する。
* **Technical Design**:
  * `go run ./cmd/excel-to-pdf ...` 実行ヘルパーで `exit code 2` を検証。
* **Logic**:
  * パス移動後の CLI 解決が崩れても即座に検出できるようにする。

#### [NEW] `cmd/pdf-to-image/main_test.go`(file://cmd/pdf-to-image/main_test.go)
* **Description**: `--format` / `--engine` validation の `exit code 2` 契約を移行後にも保証する。
* **Technical Design**:
  * 不正引数ケースを table-driven で確認。
* **Logic**:
  * ルート移行によるコマンド実行文脈差分の回帰を防止する。

#### [MODIFY] `cmd/excel-to-pdf/main.go`(file://cmd/excel-to-pdf/main.go)
* **Description**: import と実行パス基準をルートモジュールへ移行する。
* **Technical Design**:
  * `github.com/axsh/entext/internal/...` import をルート配下前提で再解決。
* **Logic**:
  * `go run ./cmd/excel-to-pdf --help` がルートから直接実行可能になるようにする。

#### [MODIFY] `cmd/pdf-to-image/main.go`(file://cmd/pdf-to-image/main.go)
* **Description**: 上記と同様。
* **Technical Design**:
  * ルート基準 import を採用し、挙動変更は行わない。
* **Logic**:
  * 既存の backend/engine/dpi/sheet-map 契約を維持する。

#### [MODIFY] `cmd/image-to-markdown/main.go`(file://cmd/image-to-markdown/main.go)
* **Description**: ルート移行後も CLI 実行経路を維持する。
* **Technical Design**:
  * `arctic-tern` 経路含む既存フローは不変。
* **Logic**:
  * モジュール境界変更のみで機能退行を出さない。

#### [MODIFY] `internal/...`(file://internal)
* **Description**: `features/entext/internal` 一式をルートへ移設し import を一括更新する。
* **Technical Design**:
  * 対象パッケージ:
    * `internal/common/...`
    * `internal/exceltopdf/...`
    * `internal/pdftoimage/...`
    * `internal/pdfnative/...`
    * `internal/imagetomd/...`
    * `internal/logger/...`
  * `pdfnative` の Windows 互換 shim (`mingw_compat_windows_cgo.go`) を保持。
* **Logic**:
  * 既存機能ロジック（go-native含む）を維持したまま、モジュールパスのみ整合させる。

#### [MODIFY] `features/entext/go.mod`(file://features/entext/go.mod)
* **Description**: ルートモジュール化後に廃止対象として扱う。
* **Technical Design**:
  * 最終状態では削除、または参照されない状態を保証。
* **Logic**:
  * 二重モジュール状態を解消し、`go env GOMOD` の一意性を確保する。

### Scripts / Tests

#### [NEW] `tests/external_import_e2e_test.go`(file://tests/external_import_e2e_test.go)
* **Description**: 疑似外部モジュールを作成して `go get` / `import` 成立を E2E で固定する。
* **Technical Design**:
  * ```go
    func TestExternalImportRootModule(t *testing.T)
    func TestExternalImportSubPackages(t *testing.T)
    ```
  * テスト内で `t.TempDir()` に新規モジュールを作成し、以下を実行:
    * `go mod init example.com/ext`
    * `go mod edit -replace github.com/axsh/entext=<repo-root>`
    * `go get github.com/axsh/entext`
    * `go build ./...`
* **Logic**:
  * ネットワーク依存なしで「外部利用者視点の import 成立」を自動検証する。

#### [MODIFY] `tests/go.mod`(file://tests/go.mod)
* **Description**: 旧 `../features/entext` 依存を除去し、ルートモジュール参照へ切り替える。
* **Technical Design**:
  * `replace github.com/axsh/entext => ..`
* **Logic**:
  * tests モジュールが実配置と一致したモジュール解決を行う。

#### [MODIFY] `tests/e2e_backend_pipeline_test.go`(file://tests/e2e_backend_pipeline_test.go)
* **Description**: `go run` 実行ディレクトリと相対パス解決をルート配置前提へ更新する。
* **Technical Design**:
  * `toolCommand` の `cmd.Dir` を `..`（repo root）へ変更。
  * 入出力 path 正規化 helper をルート実行基準で調整。
* **Logic**:
  * 既存の go-native E2E を移行後も同等に通す。

#### [MODIFY] `scripts/process/build.sh`(file://scripts/process/build.sh)
* **Description**: ルートモジュールを build/test 対象として扱う。
* **Technical Design**:
  * `features/*` 走査に依存しない root module 分岐を追加。
  * 既存の feature/module build 処理と競合しない順序で実行。
* **Logic**:
  * `./scripts/process/build.sh` 一発で root module の unit/build が実行される状態を作る。

### Documentation

#### [MODIFY] `README.md`(file://README.md)
* **Description**: パス導線と import 例をルートモジュール構成へ合わせる。
* **Technical Design**:
  * `go get github.com/axsh/entext`
  * `go run ./cmd/...` 例
  * `import "github.com/axsh/entext"` 例
* **Logic**:
  * ドキュメントの手順で実際に導入・実行できる状態を保証する。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/005-Root-GoModule-PublicImport.md`(file://prompts/phases/000-foundation/branches/main/ideas/005-Root-GoModule-PublicImport.md)
* **Description**: 実装完了後の確定仕様（削除対象、検証手順、制約）を追記する。
* **Technical Design**:
  * 採用した migration 手順と最終配置を反映。
* **Logic**:
  * 仕様と実装結果の乖離を防止する。

## Step-by-Step Implementation Guide

1. **TDD Red: 外部 import 契約の先行固定**
   * Edit `tests/external_import_e2e_test.go` to add failing tests for root import and sub-package imports.
   * Edit `tests/go.mod` to use `replace github.com/axsh/entext => ..`.
   * Edit `tests/e2e_backend_pipeline_test.go` to prepare root-based command execution.

2. **Root Module Scaffold 作成**
   * Add `go.mod` and `go.sum` at repository root.
   * Add `entext_root_import_test.go` to lock public API compile contract at root.

3. **コード配置移行（機能変更なし）**
   * Move `features/entext/entext.go` to root `entext.go`.
   * Move `features/entext/cmd` to `cmd`.
   * Move `features/entext/internal` to `internal`.
   * Move `features/entext/excelpdf`, `pdfimage`, `imagemd` to root package directories.
   * Update imports to root module path where needed.

4. **CLI/API 回帰テスト更新**
   * Add/adjust `cmd/excel-to-pdf/main_test.go` and `cmd/pdf-to-image/main_test.go`.
   * Ensure `entext.go` public functions and option structs remain signature-compatible.

5. **ビルドスクリプト調整**
   * Edit `scripts/process/build.sh` to build/test root module first, then keep existing feature handling.
   * Ensure no raw `go test` workflow leak in scripts execution path.

6. **旧モジュール残骸の整理**
   * Remove or deprecate `features/entext/go.mod` after successful root build/test.
   * Validate `go env GOMOD` from repo root points to root `go.mod`.

7. **ドキュメント更新**
   * Edit `README.md` for root module layout and updated run/import instructions.
   * Edit `ideas/005-Root-GoModule-PublicImport.md` with finalized implementation notes.

8. **Verification Plan 実行**
   * Run full `build.sh` first.
   * Then run scoped integration tests including newly added external import E2E.

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   ```bash
   ./scripts/process/build.sh
   ```
   * **Log Verification**: ルート `go.mod` が使用され、`cmd/*`, `internal/*`, `entext_root_import_test` が PASS すること。

2. **Integration Tests (module/public API/go-native regression)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "GoNative|SheetMap|Sanitize|InvalidSheets|DPI|Engine|Backend|PublicAPI"
   ```
   * **Log Verification**: 既存 go-native E2E が継続成功し、ルート配置移行で回帰がないこと。

3. **Integration Tests (external import E2E)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "ExternalImport|GoGetRootModule|PublicPackageImport"
   ```
   * **Log Verification**: 一時外部モジュールで `go get github.com/axsh/entext` 後に `go build ./...` が成功すること。

4. **Integration Tests (chain regression)**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "pipeline_chain|image_to_markdown|session"
   ```
   * **Log Verification**: `image-to-markdown` 系の既存回帰がないこと。

5. **E2E Tests (新規/追加)**:
   新機能の動作確認は手動コマンド実行で代替せず、`tests/` にE2Eテストとして実装する。

   #### [NEW] `tests/external_import_e2e_test.go`(file://tests/external_import_e2e_test.go)
   * **テストケース**:
     * `TestExternalImportRootModule`
     * `TestExternalImportSubPackages`
   * **検証ポイント**:
     * ルート module を `go get` した外部モジュールが `import "github.com/axsh/entext"` で build 成功する。
     * `excelpdf`, `pdfimage`, `imagemd` import でも build 成功する。

   #### [MODIFY] `tests/e2e_backend_pipeline_test.go`(file://tests/e2e_backend_pipeline_test.go)
   * **テストケース**:
     * 既存 go-native E2E 一式（`TestE2EExcelToPDFGoNativeWritesRealSheetMap` など）
   * **検証ポイント**:
     * ルート実行ディレクトリへ移行後も同一テストが PASS する。

### Test Item Design (Bottom-Up)

1. **C (leaf)**: `go.mod`/import 解決、path helper、root public compile contract。
2. **B (middle)**: CLI command resolution (`go run ./cmd/...`)、script build flow、tests module replace 解決。
3. **A (top)**: 外部 import E2E、既存 go-native E2E、公開 API 回帰。

#### 観点チェックリスト適用
- 正常系: root import、sub-package import、CLI help 実行、go-native pipeline 実行。  
- 異常系: 不正引数（exit code 2）既存テスト継続。  
- 外部連携: Go module resolver、Excel COM、go-fitz。  
- データ一貫性: sidecar と画像命名の一致を既存E2Eで確認。  
- 状態遷移: `features/entext` 前提から root module 前提への遷移確認。  
- 設定反映: `replace` 設定、build script 対象、CLI 実行基準。  
- 副作用: 旧モジュール残骸が build 解決へ影響しないこと。  

#### テスト項目セルフレビュー（§11.4）
- **網羅性**: 仕様必須要件1-8を C->B->A で段階的にカバー。  
- **証拠十分性**: `go get` + `go build` の実行結果で import 成立を直接証明。  
- **迂回排除**: `replace ../features/entext` を除去し、ルートを明示参照。  
- **依存整合**: leaf（module解決）通過後に CLI/E2E を実行する順序で設計。  

### Post-Test Comprehensive Verdict Plan

全テスト完了後、`testing-rules` §12 の7項目（スキップ、部分エラー、迂回成功、誤設定、順序依存、カバレッジ、外部状態）をチェック表で評価し、`✅/⚠️/❌` 判定を記録する。`⚠️` 以上の場合は、追加で必要な確認（例: 実リモート `go get` 検証）を明記する。

## Documentation

`prompts/specifications` 配下ドキュメントを確認し、本計画の影響範囲を更新する。

#### [MODIFY] `README.md`(file://README.md)
* **更新内容**: ルート module 前提の導入・import・CLI実行手順へ更新。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/005-Root-GoModule-PublicImport.md`(file://prompts/phases/000-foundation/branches/main/ideas/005-Root-GoModule-PublicImport.md)
* **更新内容**: 実装確定後の最終配置、削除対象、検証結果の反映。
