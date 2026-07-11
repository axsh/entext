# 001 entext Module README PublicAPI

## 背景 (Background)

- 現状の実装では機能名やモジュール名に `doc-convert` が使われているが、プロジェクトとしては `entext` の名称で統一したい。
- モジュール識別子が `github.com/axsh/tokotachi/features/doc-convert` のままだと、リポジトリ名・配布名・インポートパスの整合性が崩れる。
- 今後の利用者・開発者が混乱しないよう、CLI 名、モジュール名、関連ドキュメント表現を一貫した命名へ揃える必要がある。

## 要件 (Requirements)

### 必須要件

1. プロジェクト名称を `doc-convert` ではなく `entext` へ統一すること。
2. `features/entext/go.mod` の module 宣言を `github.com/axsh/entext` に変更すること。
3. Go ソース内で旧 module path (`github.com/axsh/tokotachi/features/doc-convert/...`) を参照している import をすべて更新すること。
4. 生成されるビルド成果物の配置/命名が、`entext` 名称でも破綻しないことを確認すること。
5. 既存の機能要件（3CLI 構成、`--stdin` 契約、`image-to-markdown` 再現アルゴリズム）を維持し、命名変更による機能退行を発生させないこと。
6. `axsh/arctic-tern` 依存を利用した `image-to-markdown` 実装を維持すること。
7. プロジェクトルートに `README.md` を追加し、`entext` の目的・インストール方法・CLI 利用方法・Go API 利用方法を記載すること。
8. 外部プロジェクトが `go get github.com/axsh/entext` で導入し、Go API として組み込み利用できる公開 API を設計・提供すること。

### 追加必須要件（整合性）

1. ドキュメント（仕様書・計画書）で記述しているプロジェクト名も `entext` 表記へ揃えること。
2. 将来の公開/再利用を想定し、README などの導線に旧名称が残る場合は修正対象として洗い出すこと。
3. モジュール名変更後に `go mod tidy` とビルドスクリプト実行で依存解決が成立すること。
4. 公開 API は CLI 実装に依存しすぎないよう、module root package `github.com/axsh/entext` を安定インターフェースとして提供すること。

## 実現方針 (Implementation Approach)

### 1. 命名統一ポリシー

- 実装ディレクトリを `features/doc-convert` から `features/entext` へ変更し、物理パスと module path を一致させる。
- 外部公開される識別子（module path / ドキュメント名 / 導線名）は `entext` で統一する。

### 2. モジュール・import 更新方針

- `features/entext/go.mod`:
  - `module github.com/axsh/entext`
- `features/entext` 配下の全 Go ファイル:
  - `github.com/axsh/tokotachi/features/doc-convert/...` を `github.com/axsh/entext/...` に置換
- 変更後に `go mod tidy` 実行で依存解決を再生成する。

### 3. 機能維持方針

- 命名変更で以下を壊さないことを確認する:
  1. `excel-to-pdf`
  2. `pdf-to-image`
  3. `image-to-markdown`
- `image-to-markdown` は `github.com/axsh/arctic-tern/client` 利用実装を維持し、独自 HTTP 実装へ戻さない。

### 4. ビルド運用方針

- `scripts/process/build.sh` を利用して、unit/build パイプラインの通過を確認する。
- `scripts/process/integration_test.sh` は現行リポジトリ制約（`tests/go.mod` 要件）を考慮し、実行可否と不足条件を明示する。

### 5. 公開 Go API 設計方針

- 外部公開用 API は module root package `github.com/axsh/entext` に定義する。
- 想定パッケージ構成:
  - `github.com/axsh/entext`（Facade + type definitions）
  - `github.com/axsh/entext/excelpdf`
  - `github.com/axsh/entext/pdfimage`
  - `github.com/axsh/entext/imagemd`
- 最小公開インターフェース:
  - `type Converter interface { Convert(ctx context.Context, in Input) (Output, error) }`
  - 各機能ごとに `New(...)` コンストラクタを提供。
  - `imagemd` は `WithServer`, `WithModel`, `WithRefPatterns`, `WithStrictGapJudge` などの Option パターンを採用。
- 戻り値はファイルパス中心で統一し、CLI と同一のデータ契約（1入力 -> 生成物パス群）を保つ。
- 例外/エラー設計:
  - バリデーションエラーと実行時エラーを区別できる型を公開する。
  - `errors.Is` / `errors.As` で判定可能にする。
- 後方互換性:
  - `github.com/axsh/entext` の公開シグネチャを安定面として扱い、`internal/` は自由に変更可能とする。

## 検証シナリオ (Verification Scenarios)

1. モジュール名変更の反映確認
   1. `features/entext/go.mod` の `module` を確認する。
   2. 値が `github.com/axsh/entext` になっていることを確認する。

2. import 統一の確認
   1. `features/entext/**/*.go` を検索する。
   2. `github.com/axsh/tokotachi/features/doc-convert` が残っていないことを確認する。
   3. `github.com/axsh/entext` 参照へ置換されていることを確認する。

3. ビルド確認
   1. `scripts/process/build.sh` を実行する。
   2. `doc-convert` feature の unit/build が PASS することを確認する。

4. 主要 CLI の起動確認
   1. `bin/doc-convert/excel-to-pdf --help`
   2. `bin/doc-convert/pdf-to-image --help`
   3. `bin/doc-convert/image-to-markdown --help`
   4. すべてで usage が表示され、クラッシュしないことを確認する。

5. `arctic-tern` 利用維持確認
   1. `features/entext/internal/imagetomd/tern/client.go` を確認する。
   2. `github.com/axsh/arctic-tern/client` import が存在することを確認する。
   3. `CreateSession/SendMessage/Terminate` フローが使用されることを確認する。

6. ルート `README.md` 追加確認
   1. プロジェクトルートに `README.md` が存在することを確認する。
   2. `entext` 名称、`go get github.com/axsh/entext`、CLI 例、Go API 利用例が記載されていることを確認する。

7. 公開 Go API の利用確認
   1. 外部モジュール想定のサンプルコードで `import "github.com/axsh/entext"` を記述する。
   2. `excelpdf` / `pdfimage` / `imagemd` のコンストラクタ呼び出しと `Convert(...)` が可能であることを確認する。
   3. API 利用時に CLI 実行が必須でない（ライブラリとして直接呼べる）ことを確認する。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `scripts/process/build.sh`

2. 共通機能の統合テスト（実行環境が整っている場合）:
   - `scripts/process/integration_test.sh --categories "common" --specify "doc_convert|stdin|output_mode|module"`

3. LLM 系統合テスト（実行環境が整っている場合）:
   - `scripts/process/integration_test.sh --categories "llm" --specify "image_to_markdown|arctic_tern|session"`

4. 公開 API テスト:
   - `scripts/process/build.sh`
   - `scripts/process/integration_test.sh --categories "common" --specify "entext_api|excelpdf|pdfimage|imagemd"`

## 未実施・未達項目 (Pending Items at Review Point)

1. ルートモジュール整備の未達:
   - 現状は `features/entext/go.mod` が実体であり、プロジェクトルートの `go.mod` は未整備。
   - 要件8の「`go get github.com/axsh/entext` で導入」を厳密に満たす配布形態としては未確定。
2. 検証シナリオ7（公開 Go API の利用確認）の証跡不足:
   - 外部モジュールからの import/実行を恒常的に担保する E2E（external import style）が不足。
   - `excelpdf` / `pdfimage` / `imagemd` の外部利用サンプルを自動検証するテストが未完了。
3. 統合テストコマンド記述の不整合:
   - 本仕様書の `--categories` 付きコマンドは、現行 `scripts/process/integration_test.sh` の対応オプションと不一致。
   - 現行スクリプト前提では `--specify` のみで記載する必要がある。

### 要件対応表

- 要件1-3（名称統一・module 宣言・import 更新）:
  - 単体検証: ファイル内容確認 + grep/rg 検索
  - 自動検証: `scripts/process/build.sh`
- 要件4-5（命名変更後の機能維持）:
  - 単体検証: CLI `--help` スモーク
  - 自動検証: `scripts/process/build.sh`
- 要件6（`arctic-tern` 維持）:
  - 単体検証: `tern/client.go` 実装確認
  - 自動検証: `scripts/process/build.sh` で依存解決成功
- 要件7（ルート `README.md` 追加）:
  - 単体検証: `README.md` の存在と記載内容確認
  - 自動検証: `scripts/process/build.sh`（ドキュメントでの導線崩れがないことをレビュー）
- 要件8（外部組み込み可能な公開 Go API）:
  - 単体検証: `github.com/axsh/entext` の公開シグネチャ確認
  - 自動検証: `scripts/process/build.sh` + API 利用テスト
- 追加必須要件（ドキュメント整合・依存整合）:
  - 単体検証: 仕様/計画の記述チェック
  - 自動検証: `go mod tidy` + `scripts/process/build.sh`

## テストコマンド補足 (Current Script Constraints)

- 現行 `scripts/process/integration_test.sh` は `--categories` を受け付けず、`--specify` のみ対応。
- したがって、上記の統合テスト実行例は次のように読み替える。
  - `scripts/process/integration_test.sh --specify "entext_api|excelpdf|pdfimage|imagemd|stdin|output_mode|module"`
