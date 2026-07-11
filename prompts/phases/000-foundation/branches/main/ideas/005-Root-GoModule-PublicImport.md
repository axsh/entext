# 005 Root GoModule PublicImport

## 背景 (Background)

- 現在の `module github.com/axsh/entext` 宣言は `features/entext/go.mod` に存在し、リポジトリルートには `go.mod` がない。
- この構成では、外部利用者が期待する `import "github.com/axsh/entext"` と、実ファイル配置の整合が崩れる。
- 既存の `tests/go.mod` は `replace github.com/axsh/entext => ../features/entext` に依存しており、外部環境を正確に再現できていない。
- 公開モジュールとしての要件（`go get github.com/axsh/entext` で導入し、そのまま import できる）を満たすため、選択肢A（ルート正式モジュール化）を実施する。

## 要件 (Requirements)

### 必須要件

1. リポジトリルートに `go.mod` を配置し、`module github.com/axsh/entext` を定義すること。
2. `import "github.com/axsh/entext"` で公開 API（`FileJob`, `ConvertExcelToPDFWithOptions` など）へ到達できる配置にすること。
3. `github.com/axsh/entext/excelpdf`, `github.com/axsh/entext/pdfimage`, `github.com/axsh/entext/imagemd` が外部から import 可能であること。
4. `cmd/`, `internal/`, `excelpdf/`, `pdfimage/`, `imagemd/` をモジュールルート基準へ再配置し、旧 `features/entext` 依存を除去すること。
5. `scripts/process/build.sh` と `scripts/process/integration_test.sh` において、新しい配置で build/test が成立すること。
6. `tests/go.mod` の `replace` 依存を見直し、外部利用検証に近い形へ更新すること（少なくとも `../features/entext` 依存を除去する）。
7. `README.md` の導線（`go get github.com/axsh/entext`、import 例、CLI パス）が実配置と一致すること。
8. 既存機能（`excel-to-pdf`, `pdf-to-image`, `image-to-markdown`）の仕様と挙動を維持すること。

### 任意要件

1. 過渡期の作業効率のため、短期間のみ `go.work` または互換レイヤを使って移行してもよい（最終的には不要化する）。
2. 既存ブランチとの差分レビュー容易性を高めるため、移動と機能変更を段階分離して実施してもよい。

## 実現方針 (Implementation Approach)

### 1. ルートモジュール化

- ルートへ `go.mod` / `go.sum` を作成し、`module github.com/axsh/entext` を定義する。
- 既存 `features/entext/go.mod` は最終的に廃止し、単一モジュールに統合する。

### 2. ディレクトリ再配置

- `features/entext` 配下の実装を、ルート配下へ移設する。
  - `features/entext/cmd` -> `cmd`
  - `features/entext/internal` -> `internal`
  - `features/entext/excelpdf` -> `excelpdf`
  - `features/entext/pdfimage` -> `pdfimage`
  - `features/entext/imagemd` -> `imagemd`
  - `features/entext/entext.go` -> ルート `entext.go`
- import 文は新モジュールルート前提で再解決する。

### 3. スクリプト・テスト基盤の整合

- `scripts/process/build.sh` の feature 前提ロジックを見直し、ルートモジュールを直接 build/test できるようにする。
- `tests/` の外部利用検証は、`replace => ..`（ルート）または疑似外部モジュール方式へ切り替える。
- `go run ./cmd/...` 実行ディレクトリ依存を最小化する（相対パス基準の明確化）。

### 4. 公開 API 契約の維持

- 既存公開シンボルの破壊的変更を避ける。
- 破壊的変更が不可避な場合は、互換関数または移行ガイドを同時提供する。

## 検証シナリオ (Verification Scenarios)

1. ルートモジュール成立確認
   1. ルートで `go env GOMOD` を実行する。
   2. 出力が `.../entext/go.mod` になることを確認する。
   3. `features/entext/go.mod` が不要化されていることを確認する。

2. 外部 import 成立確認（最重要）
   1. 一時ディレクトリに新規 Go モジュールを作成する。
   2. `go get github.com/axsh/entext@<対象コミット>` を実行する。
   3. `import "github.com/axsh/entext"` を含む最小コードを `go build` する。
   4. build が成功することを確認する。

3. サブパッケージ import 成立確認
   1. 同様に `excelpdf`, `pdfimage`, `imagemd` を import した最小コードを作成する。
   2. `go build` が成功することを確認する。

4. CLI 配置整合確認
   1. `go run ./cmd/excel-to-pdf --help`
   2. `go run ./cmd/pdf-to-image --help`
   3. `go run ./cmd/image-to-markdown --help`
   4. すべて usage 表示で終了し、実行時パニックがないことを確認する。

5. 既存 go-native 経路の回帰確認
   1. `excel-to-pdf --engine go-native` で PDF + sidecar が生成されることを確認する。
   2. `pdf-to-image --engine go-native` で sidecar 命名が維持されることを確認する。
   3. 既存 E2E（DPI差分/命名）を通過することを確認する。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `scripts/process/build.sh`

2. モジュール配置・公開 API・go-native 回帰の統合テスト:
   - `scripts/process/integration_test.sh --specify "GoNative|SheetMap|Sanitize|InvalidSheets|DPI|Engine|Backend|PublicAPI"`

3. 連鎖回帰（影響範囲最終確認）:
   - `scripts/process/integration_test.sh --specify "pipeline_chain|image_to_markdown|session"`

4. 外部 import E2E（新規追加）:
   - `scripts/process/integration_test.sh --specify "ExternalImport|GoGetRootModule|PublicPackageImport"`
   - 備考: 既存テストに存在しない場合は `tests/` に追加実装する。

### 要件対応表

- 要件1-4（ルートモジュール化・再配置）:
  - `scripts/process/build.sh`
  - `scripts/process/integration_test.sh --specify "PublicAPI|GoGetRootModule|ExternalImport"`
- 要件5-6（スクリプト/テスト整合）:
  - `scripts/process/build.sh`
  - `scripts/process/integration_test.sh --specify "Backend|Engine|ExternalImport"`
- 要件7（README 整合）:
  - `scripts/process/integration_test.sh --specify "PublicAPI|GoGetRootModule"`
- 要件8（機能維持）:
  - `scripts/process/integration_test.sh --specify "GoNative|SheetMap|DPI|pipeline_chain|image_to_markdown|session"`

## 実装結果メモ (2026-07-10)

- ルートへ `go.mod` / `go.sum` を配置し、`go env GOMOD` が `.../entext/go.mod` を返すことを確認。
- `entext.go`, `cmd`, `internal`, `excelpdf`, `pdfimage`, `imagemd` をルートへ展開し、root module 前提で build 可能化。
- `tests/go.mod` の `replace` を `../features/entext` から `..` へ変更。
- 新規 `tests/external_import_e2e_test.go` で疑似外部モジュールの `go get github.com/axsh/entext` + `go build ./...` を自動検証。
- `scripts/process/build.sh` は root module を先に build/test し、同一 module path の `features/entext` は重複実行をスキップする仕様へ変更。
- `README.md` の CLI 配置説明を `features/entext/cmd` から `cmd` へ更新。

### 実装確定

- `features/entext/go.mod` は削除済み。ルート `go.mod` を唯一のモジュール定義として運用する。
