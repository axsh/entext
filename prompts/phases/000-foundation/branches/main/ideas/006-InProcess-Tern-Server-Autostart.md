# 006 InProcess Tern Server Autostart

## 背景 (Background)

- `image-to-markdown` 実行時に `http://localhost:3100` へ接続できず、Markdown 変換が失敗する事象が発生している。
- 現状は `internal/imagetomd/tern/client.go` が外部 HTTP サーバ前提で `github.com/axsh/arctic-tern/client` を利用しており、サーバ起動責務がツール外にある。
- 利用者視点では「CLIを実行しただけで動く」ことが期待されるため、`axsh/arctic-tern/server` を利用したインプロセス起動を標準化する必要がある。
- `tmp/` 配下の設定ファイルはコミット対象として不適切であり、再現性のある構成管理（Git管理下）へ移す必要がある。
- APIキーの事前投入を運用可能にするため、`arctic-tern/vault` を利用した keyring 設定CLI（`features/vault-cli`）が必要である。

## 要件 (Requirements)

### 必須要件

1. `image-to-markdown` 実行時に、Tern サーバ未起動であっても変換可能であること。
2. `github.com/axsh/arctic-tern/server` を用いて、`entext` プロセス内部でサーバを起動できること。
3. インプロセス起動したサーバは、変換処理終了時に確実に停止されること（正常終了・異常終了ともに）。
4. 既存の外部サーバ接続モード（`--server` 指定）を維持し、後方互換を壊さないこと。
5. エラー時に「起動失敗」か「接続失敗」かを識別可能なメッセージを返すこと。
6. 公開API `ConvertImageToMarkdown` からも同様にインプロセス起動を利用できること。
7. 既存の `StrictGapJudge` / `SaveQuestionLog` / sleep設定などの挙動を変更しないこと。
8. `tmp/tern-config.yaml` 相当の Tern 設定ファイルを読み込んで in-process 起動できること。
9. 設定ファイルには最低限、以下のキーを解釈可能であること（未使用キーは透過でも可）:
   - `llm_gateway.port`
   - `llm_gateway.model_profiles_path`
   - `vault.backend`
   - `default_profile`
   - `providers`
   - `agents`
10. CLI から Tern 設定ファイルを明示指定できること（仮: `--tern-config`）。
11. 設定ファイル未指定時は既定探索順を持つこと（例: 指定パス -> カレント `tern-config.yaml` -> 失敗時エラー）。
12. `llm_gateway.model_profiles_path` は相対パス記述を標準とし、設定ファイル自身の配置ディレクトリ基準で解決すること。
13. Tern 設定ファイルは `tmp/` ではなく、Git管理下の恒久パス（例: `settings/tern/tern-config.yaml`）に配置すること。
14. `github.com/axsh/arctic-tern` は再度 `go get` を実施し、`server` および `vault` 利用に必要なバージョンを `go.mod` / `go.sum` へ反映すること。
15. `features/vault-cli` を新設し、`arctic-tern/vault` を用いて keyring に API キーを設定できること。
16. `features/vault-cli` は少なくとも OpenAI/Anthropic 向けのキー登録コマンドを提供すること（例: `set --provider openai --name default`）。
17. `features/vault-cli` で登録したキーを `image-to-markdown` の in-process Tern 起動時に利用できること。

### 任意要件

1. 実行モードを選択可能にする（例: `auto|external|inproc`）。
2. 将来の並列処理に備え、インプロセスサーバ管理を再利用可能なコンポーネントとして分離する。

## 実現方針 (Implementation Approach)

### 1. Tern接続層の抽象化

- `internal/imagetomd/tern` に「セッションクライアント作成戦略」を導入する。
- 既存 `ArcticClient`（外部HTTP接続）は維持しつつ、`arctic-tern/server` を内部起動する `InProcessClient`（仮称）を追加する。
- `InProcessClient` は以下を責務として持つ:
  - サーバ起動（空きポート確保含む）
  - クライアント接続確立
  - 終了時のシャットダウン

### 2. 実行モードの導入

- `ImageToMarkdownConfig` に Tern 接続モード設定（仮: `TernMode`）を追加する。
- `ImageToMarkdownConfig` に Tern 設定ファイルパス（仮: `TernConfigPath`）を追加する。
- CLI `cmd/image-to-markdown` にモード指定フラグ（仮: `--tern-mode`）を追加し、デフォルトを `auto` とする。
- CLI `cmd/image-to-markdown` に設定ファイル指定フラグ（仮: `--tern-config`）を追加する。
- `auto` の動作:
  1. 指定 `--server` へ接続試行
  2. 接続不可なら in-process 起動へフォールバック
  3. in-process 起動時は `--tern-config`（未指定なら探索結果）を `arctic-tern/server` 起動設定として使用

### 2.5 設定ファイル準拠

- `tmp/tern-config.yaml` をリファレンスとして、以下の運用を設計に反映する:
  - `llm_gateway.port` が指定されている場合は優先して利用（競合時は明示エラー）
  - `llm_gateway.model_profiles_path` は相対パスを優先し、設定ファイル配置ディレクトリ基準で絶対化してサーバへ渡す
  - `vault.backend: keyring` を含む構成で起動できることを保証
  - `agents.codex` と `default_profile` の整合が取れている場合に既定モデル解決できること
- 設定ファイルパス自体は絶対パス（空白・日本語含む）でも読めることを維持しつつ、`model_profiles_path` はポータブル性のため相対パス推奨とする。
- 実体配置は `tmp/` ではなく、`settings/tern/tern-config.yaml`（仮）へ移す。

### 2.6 依存更新方針（arctic-tern）

- 実装前に `github.com/axsh/arctic-tern` を再度 `go get` し、`server` / `vault` 利用に必要な依存解決を確定する。
- 依存更新後は `go mod tidy` を実施し、`go.mod` / `go.sum` をコミット対象とする。

### 2.7 Vault CLI の追加（features/vault-cli）

- `features/vault-cli` を新設し、`arctic-tern/vault` ベースで keyring バックエンドへキー投入する。
- 代表コマンド（仮）:
  - `vault-cli set --provider openai --name default`
  - `vault-cli set --provider anthropic --name default`
  - `vault-cli get --provider openai --name default`（疎通確認用）
- 生成される保存先キーは `tern-config.yaml` の `vault://providers/<provider>/<name>` と一致させる。

### 3. ライフサイクルとエラーハンドリング

- `ConvertImageToMarkdown` 内で `defer` による停止を必須化し、リークを防止する。
- 起動失敗時には `ValidationError` ではなく runtime error として返却する。
- ログには最低限以下を出す:
  - 使用モード（external / inproc / auto-fallback）
  - 起動ポート（inproc時）
  - 失敗フェーズ（boot/connect/session）

### 4. 互換性維持

- 既存の `--server` 引数と `ImageToMarkdownConfig.ServerURL` は有効のまま残す。
- 既存テストで検証済みの変換ロジック（Phase 0-5）には変更を加えない。

## 検証シナリオ (Verification Scenarios)

1. **外部サーバなしでの自動起動成功**
   1. `localhost:3100` で何も起動していない状態にする。
   2. `image-to-markdown` を実行する（`--tern-mode auto` またはデフォルト）。
   3. 変換が成功し、Markdownファイルとセッションログが出力されることを確認する。
   4. 実行完了後、インプロセスで起動したサーバが残留していないことを確認する。

2. **外部サーバ優先利用**
   1. 外部 Tern サーバを `localhost:3100` で起動する。
   2. `image-to-markdown --tern-mode external --server http://localhost:3100` を実行する。
   3. インプロセス起動を行わず、外部サーバ経由で変換完了することを確認する。

3. **外部指定の失敗時のフォールバック**
   1. `--tern-mode auto --server http://localhost:3100` で、外部サーバ停止状態にする。
   2. `--tern-config tmp/tern-config.yaml` を与えて実行する。
   3. 実行後に in-process 起動へ切り替わり、最終的に成功することを確認する。
   4. `tmp/tern-config.yaml` で指定したモデル設定（`gpt-5.3-codex`）が利用されることを確認する。
   5. ログにフォールバック情報が残ることを確認する。

4. **in-process 強制モード**
   1. `--tern-mode inproc --tern-config tmp/tern-config.yaml` を指定して実行する。
   2. `--server` の到達可否に依存せず、内部起動で処理されることを確認する。
   3. 起動ポートが設定ファイル値（例: `14000`）を使用することを確認する。

5. **起動失敗時の失敗形の明確化**
   1. 意図的に `arctic-tern/server` 起動条件を満たさない設定で実行する。
   2. 終了コード1で失敗し、メッセージが起動失敗であることを明示することを確認する。

6. **設定ファイル未指定時の失敗形**
   1. `--tern-mode inproc` で `--tern-config` を指定せず、既定探索先にも設定ファイルがない状態にする。
   2. 設定ファイル未検出エラーで失敗し、解決方法（`--tern-config` 指定）が表示されることを確認する。

7. **Vault CLI によるキー登録**
   1. `features/vault-cli` で OpenAI キーを keyring へ登録する。
   2. `settings/tern/tern-config.yaml` の `vault://providers/openai/default` 参照を有効化した状態で `image-to-markdown --tern-mode inproc` を実行する。
   3. API キー未設定エラーにならず、変換が継続することを確認する。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `./scripts/process/build.sh`

2. image-to-markdown 周辺の統合テスト（既存回帰）:
   - `./scripts/process/integration_test.sh --specify "image_to_markdown|session|llm_image_to_markdown"`

3. in-process 起動機能の統合テスト（新規）:
   - `./scripts/process/integration_test.sh --specify "InProcessTern|TernMode|AutoFallback"`

4. 設定ファイル適用の統合テスト（新規）:
   - `./scripts/process/integration_test.sh --specify "TernConfig|InProcessConfig|VaultKeyring|ModelProfilesPath"`

5. 公開API経由の回帰確認（新規）:
   - `./scripts/process/integration_test.sh --specify "RootAPIValidationErrors|ConvertImageToMarkdown"`

6. Vault CLI の統合テスト（新規）:
   - `./scripts/process/integration_test.sh --specify "VaultCLI|Keyring|TernVaultIntegration"`

### 要件対応表

- 要件1-3（自動起動・内部起動・停止保証）:
  - 単体: `internal/imagetomd/tern` の起動/停止テスト
  - 統合: `--specify "InProcessTern|AutoFallback"`
- 要件4（後方互換）:
  - 既存 `image_to_markdown` 系テスト
- 要件5（エラー識別）:
  - 起動失敗/接続失敗ケースのエラーメッセージ検証テスト
- 要件6（公開API対応）:
  - `ConvertImageToMarkdown` 呼び出しテスト（モード別）
- 要件7（既存挙動維持）:
  - `llm_image_to_markdown_compat` / `llm_image_to_markdown_improved` 回帰テスト
- 要件8-11（設定ファイル対応）:
  - `--specify "TernConfig|InProcessConfig|VaultKeyring|ModelProfilesPath"`
- 要件13-17（配置・依存更新・vault-cli対応）:
  - `scripts/process/build.sh`
  - `--specify "VaultCLI|Keyring|TernVaultIntegration"`

## 実装結果メモ (2026-07-10)

- `internal/imagetomd/tern` に in-process runtime (`auto|external|inproc`) と設定ローダを追加。
- `entext.ImageToMarkdownConfig` に `TernMode` / `TernConfigPath` を追加し、`ConvertImageToMarkdown` で runtime lifecycle を管理。
- `cmd/image-to-markdown` に `--tern-mode` / `--tern-config` を追加。
- `settings/tern/tern-config.yaml` を新設し、`tmp/tern-config.yaml` はサンプル案内へ変更。
- `features/vault-cli` を新設し、`arctic-tern/shared/libs/go/vault` の keyring backend で `set/get` を提供。
- `go mod tidy` 実施により依存情報を更新。
