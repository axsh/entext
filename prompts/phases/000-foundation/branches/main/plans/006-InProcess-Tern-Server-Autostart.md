# 006-InProcess-Tern-Server-Autostart

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/006-InProcess-Tern-Server-Autostart.md`

## Goal Description

`image-to-markdown` の Tern 接続を外部サーバ依存から脱却し、`arctic-tern/server` による in-process 起動を `auto|external|inproc` モードで実現する。あわせて `tmp/` 依存を廃止し、`settings/tern/tern-config.yaml` を基準としたコミット可能な設定運用へ移行し、`arctic-tern/vault` を使った `features/vault-cli` で keyring へ API キー投入できるようにする。

## Execution Status

- [x] Step 1: TDD Red（`internal/imagetomd/tern` / `cmd/image-to-markdown` のテスト追加）
- [x] Step 2: tern config/runtime 実装
- [x] Step 3: 公開 API / CLI 連携
- [x] Step 4: 設定ファイル恒久配置
- [x] Step 5: `features/vault-cli` 追加
- [x] Step 6: 依存更新（`go mod tidy`）
- [x] Step 7: 統合テスト追加
- [x] Step 8: Verification 実行（build + scoped integration）

## User Review Required

1. `settings/tern/tern-config.yaml` をデフォルト設定パスとして固定してよいか。
2. `features/vault-cli` のコマンド名を `vault-cli` で確定してよいか。
3. Vault CLI の初期対象 provider を `openai` / `anthropic` の2つに限定してよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. 外部サーバ未起動でも変換可能 | Proposed Changes > `internal/imagetomd/tern/runtime_test.go`, `internal/imagetomd/tern/runtime.go`, `cmd/image-to-markdown/main.go` |
| 2. `arctic-tern/server` で in-process 起動 | Proposed Changes > `internal/imagetomd/tern/runtime.go` |
| 3. 終了時停止保証 | Proposed Changes > `internal/imagetomd/tern/runtime_test.go`, `internal/imagetomd/tern/runtime.go`, `entext.go` |
| 4. `--server` 後方互換維持 | Proposed Changes > `cmd/image-to-markdown/main_test.go`, `cmd/image-to-markdown/main.go`, `entext.go` |
| 5. 起動失敗/接続失敗の識別 | Proposed Changes > `internal/imagetomd/tern/runtime_test.go`, `internal/imagetomd/tern/errors.go` |
| 6. 公開 API `ConvertImageToMarkdown` から利用 | Proposed Changes > `tests/root_api_validation_test.go`, `entext.go` |
| 7. 既存変換挙動維持 | Proposed Changes > `internal/imagetomd/analyzer/*` は変更しない、`tests/llm_image_to_markdown_*_test.go` で回帰 |
| 8. 設定ファイル読込で in-process 起動 | Proposed Changes > `internal/imagetomd/tern/config_test.go`, `internal/imagetomd/tern/config.go`, `settings/tern/tern-config.yaml` |
| 9. 必須キー解釈 | Proposed Changes > `internal/imagetomd/tern/config_test.go`, `internal/imagetomd/tern/config.go` |
| 10. CLI `--tern-config` 指定 | Proposed Changes > `cmd/image-to-markdown/main_test.go`, `cmd/image-to-markdown/main.go` |
| 11. 設定探索順（指定->カレント->失敗） | Proposed Changes > `internal/imagetomd/tern/config_test.go`, `internal/imagetomd/tern/config.go` |
| 12. `model_profiles_path` 相対解決 | Proposed Changes > `internal/imagetomd/tern/config_test.go`, `internal/imagetomd/tern/config.go` |
| 13. 設定を Git 管理下へ配置 | Proposed Changes > `settings/tern/tern-config.yaml`, `tmp/tern-config.yaml`（廃止導線） |
| 14. `arctic-tern` 再 `go get` 反映 | Proposed Changes > `go.mod`, `go.sum` |
| 15. `features/vault-cli` 新設 | Proposed Changes > `features/vault-cli/go.mod`, `features/vault-cli/cmd/vault-cli/main.go` |
| 16. OpenAI/Anthropic キー登録コマンド | Proposed Changes > `features/vault-cli/cmd/vault-cli/main_test.go`, `features/vault-cli/internal/vaultcli/command.go` |
| 17. vault 登録キーを in-process で利用 | Proposed Changes > `tests/tern_inprocess_vault_integration_test.go`, `internal/imagetomd/tern/runtime.go` |

## Proposed Changes

### `internal/imagetomd/tern`

#### [NEW] `internal/imagetomd/tern/config_test.go`(file://internal/imagetomd/tern/config_test.go)
* **Description**: 設定探索順・必須キー解釈・相対パス解決の契約を先に固定する。
* **Technical Design**:
  * テーブル駆動で以下を検証:
    * 明示パス指定
    * カレント探索成功
    * 未検出エラー
    * `model_profiles_path` 相対->絶対化
  * ```go
    type LoadConfigInput struct {
        ExplicitPath string
        WorkingDir   string
    }

    type TernServerConfig struct {
        Port              int
        ModelProfilesPath string
        VaultBackend      string
        DefaultProvider   string
        DefaultModel      string
    }

    func LoadInProcessConfig(in LoadConfigInput) (TernServerConfig, error)
    ```
* **Logic**:
  * `ExplicitPath != ""` ならそれを優先。
  * 未指定時は `filepath.Join(WorkingDir, "tern-config.yaml")` を探索。
  * `llm_gateway.model_profiles_path` が相対なら `filepath.Join(filepath.Dir(configPath), rel)` で絶対化。

#### [NEW] `internal/imagetomd/tern/runtime_test.go`(file://internal/imagetomd/tern/runtime_test.go)
* **Description**: in-process 起動/停止、auto フォールバック、エラー分類を固定する。
* **Technical Design**:
  * テスト対象:
    * `ResolveMode(auto/external/inproc)`
    * `StartInProcess` / `Shutdown`
    * 接続失敗時 `ErrConnectFailed`
    * 起動失敗時 `ErrBootFailed`
  * ```go
    type Mode string
    const (
        ModeAuto     Mode = "auto"
        ModeExternal Mode = "external"
        ModeInProc   Mode = "inproc"
    )

    type Runtime struct {
        Client   Client
        Shutdown func(context.Context) error
        Endpoint string
        ModeUsed Mode
    }

    func BuildRuntime(ctx context.Context, req RuntimeRequest) (*Runtime, error)
    ```
* **Logic**:
  * `auto`: external ping 成功なら external を利用、失敗時 inproc 起動へ遷移。
  * `external`: external 接続失敗は即エラー。
  * `inproc`: external 可否に関係なく内部起動。
  * `Shutdown` は二重呼び出し安全（idempotent）にする。

#### [NEW] `internal/imagetomd/tern/errors.go`(file://internal/imagetomd/tern/errors.go)
* **Description**: 起動失敗/接続失敗/設定失敗を識別するエラー型を追加する。
* **Technical Design**:
  * ```go
    var (
        ErrBootFailed   = errors.New("tern in-process boot failed")
        ErrConnectFailed = errors.New("tern external connect failed")
        ErrConfigNotFound = errors.New("tern config file not found")
    )
    ```
* **Logic**:
  * `fmt.Errorf("%w: %v", ErrBootFailed, err)` 形式で phase を埋め込む。

#### [NEW] `internal/imagetomd/tern/config.go`(file://internal/imagetomd/tern/config.go)
* **Description**: YAML を読み、in-process 起動に必要な最小設定へ正規化する。
* **Technical Design**:
  * 生 YAML 構造体を明示:
    * ```go
      type rawConfig struct {
          LLMGateway struct {
              Port              int    `yaml:"port"`
              ModelProfilesPath string `yaml:"model_profiles_path"`
          } `yaml:"llm_gateway"`
          Vault struct {
              Backend string `yaml:"backend"`
          } `yaml:"vault"`
          DefaultProfile struct {
              Provider string `yaml:"provider"`
              Model    string `yaml:"model"`
          } `yaml:"default_profile"`
          Providers map[string]any `yaml:"providers"`
          Agents    map[string]any `yaml:"agents"`
      }
      ```
* **Logic**:
  * 必須キー未設定時は `ErrConfigNotFound` / validation runtime error を返す。
  * `model_profiles_path` は相対解決後に `filepath.Clean` する。

#### [NEW] `internal/imagetomd/tern/runtime.go`(file://internal/imagetomd/tern/runtime.go)
* **Description**: `arctic-tern/server` を内部起動するランタイムを実装する。
* **Technical Design**:
  * `RuntimeRequest`:
    * ```go
      type RuntimeRequest struct {
          Mode           Mode
          ExternalServer string
          ConfigPath     string
          Agent          string
          Model          string
          WorkingDir     string
      }
      ```
  * `BuildRuntime` が `Client` を返し、呼び出し側は `defer runtime.Shutdown(...)`。
* **Logic**:
  * inproc 起動時は `LoadInProcessConfig` の結果から `server` を初期化。
  * 起動後 endpoint を `http://127.0.0.1:<port>` として `ArcticClient` を接続。

### `cmd/image-to-markdown`

#### [NEW] `cmd/image-to-markdown/main_test.go`(file://cmd/image-to-markdown/main_test.go)
* **Description**: `--tern-mode` / `--tern-config` のバリデーションを固定する。
* **Technical Design**:
  * ケース:
    * 無効 `--tern-mode` -> exit code 2
    * `--tern-mode inproc` かつ設定未検出 -> exit code 1 with config error
  * CLI 契約として `--server` 後方互換を検証。
* **Logic**:
  * 既存 `output` バリデーションと衝突しないよう flag 解析順を明確化する。

#### [MODIFY] `cmd/image-to-markdown/main.go`(file://cmd/image-to-markdown/main.go)
* **Description**: mode/config 指定を `entext.ImageToMarkdownConfig` へ受け渡す。
* **Technical Design**:
  * `ImageToMarkdownConfig` に追加するフィールド:
    * ```go
      TernMode       string
      TernConfigPath string
      ```
  * 新規 flags:
    * `--tern-mode auto|external|inproc`
    * `--tern-config <path>`
* **Logic**:
  * 既存 `--server` は維持し、`auto` では接続先候補として扱う。

### Root Public API

#### [MODIFY] `tests/root_api_validation_test.go`(file://tests/root_api_validation_test.go)
* **Description**: `ImageToMarkdownConfig` 拡張の公開互換を検証する。
* **Technical Design**:
  * `TernMode` / `TernConfigPath` を含む初期化でコンパイル確認。
* **Logic**:
  * API 利用者が新設定を指定できることを保証する。

#### [MODIFY] `entext.go`(file://entext.go)
* **Description**: `ConvertImageToMarkdown` で tern runtime を利用し、終了時停止を保証する。
* **Technical Design**:
  * `ImageToMarkdownConfig` 拡張:
    * ```go
      type ImageToMarkdownConfig struct {
          ServerURL       string
          Agent           string
          Model           string
          TernMode        string
          TernConfigPath  string
          StrictGapJudge  bool
          SaveQuestionLog bool
          RoundSleepMS    int
          PhaseSleepMS    int
          MaxRounds       int
      }
      ```
* **Logic**:
  * `tern.BuildRuntime` を呼び、`client := runtime.Client` を analyzer へ注入。
  * `defer runtime.Shutdown(ctx)` でリソースを解放。
  * `TernMode` 未指定時は `auto` デフォルト。

### Config Files

#### [NEW] `settings/tern/tern-config.yaml`(file://settings/tern/tern-config.yaml)
* **Description**: コミット対象の標準 Tern 設定を追加する。
* **Technical Design**:
  * `tmp/tern-config.yaml` をベースに相対 `model_profiles_path` を採用。
* **Logic**:
  * `model_profiles_path: "./tern-config.yaml"` を保持し、config-dir 基準解決で可搬性を担保。

#### [MODIFY] `tmp/tern-config.yaml`(file://tmp/tern-config.yaml)
* **Description**: 開発用サンプルであることを明確化し、恒久利用しない方針を追記する。
* **Technical Design**:
  * ヘッダコメントに `settings/tern/tern-config.yaml` を正式版として案内。
* **Logic**:
  * 実運用導線を `tmp` から排除する。

### Vault CLI (`features/vault-cli`)

#### [NEW] `features/vault-cli/cmd/vault-cli/main_test.go`(file://features/vault-cli/cmd/vault-cli/main_test.go)
* **Description**: `set/get` コマンドの引数契約を固定する。
* **Technical Design**:
  * 無効 provider、name 空、secret 未入力をエラー化するテーブルテスト。
* **Logic**:
  * `openai|anthropic` 以外は validation error。

#### [NEW] `features/vault-cli/internal/vaultcli/command_test.go`(file://features/vault-cli/internal/vaultcli/command_test.go)
* **Description**: vault key path 生成規則を検証する。
* **Technical Design**:
  * `providers/openai/default` / `providers/anthropic/default` 生成を検証。
* **Logic**:
  * `vault://providers/<provider>/<name>` と一致する path を返す。

#### [NEW] `features/vault-cli/go.mod`(file://features/vault-cli/go.mod)
* **Description**: 独立機能として vault-cli モジュールを定義する。
* **Technical Design**:
  * `module github.com/axsh/entext/features/vault-cli`
  * `github.com/axsh/arctic-tern` 依存を明記。
* **Logic**:
  * keyring 操作を本体モジュールから分離し、運用ツールとして提供する。

#### [NEW] `features/vault-cli/internal/vaultcli/command.go`(file://features/vault-cli/internal/vaultcli/command.go)
* **Description**: `arctic-tern/vault` を使った set/get 実装を追加する。
* **Technical Design**:
  * ```go
    type SetInput struct {
        Provider string
        Name     string
        Secret   string
    }

    func SetKey(ctx context.Context, in SetInput) error
    func GetKey(ctx context.Context, provider, name string) (string, error)
    ```
* **Logic**:
  * 保存キーは `providers/<provider>/<name>` で統一。

#### [NEW] `features/vault-cli/cmd/vault-cli/main.go`(file://features/vault-cli/cmd/vault-cli/main.go)
* **Description**: CLI エントリポイントを実装する。
* **Technical Design**:
  * `set` / `get` サブコマンドを `cobra` で提供。
* **Logic**:
  * `set` は `--secret` 未指定時に stdin から読む。
  * 成功時は機密値を stdout へ表示しない。

### Module Dependencies

#### [MODIFY] `go.mod`(file://go.mod)
* **Description**: `arctic-tern` を再取得し server/vault API に合わせて更新する。
* **Technical Design**:
  * `go get github.com/axsh/arctic-tern@latest` 相当の更新を反映。
* **Logic**:
  * `internal/imagetomd/tern` から `server` と `client` を併用可能にする。

#### [MODIFY] `go.sum`(file://go.sum)
* **Description**: 依存更新に伴う checksum を反映する。
* **Technical Design**:
  * `go mod tidy` 実行結果を採用。
* **Logic**:
  * 再現可能ビルドを維持する。

### Integration / E2E Tests

#### [NEW] `tests/tern_inprocess_runtime_test.go`(file://tests/tern_inprocess_runtime_test.go)
* **Description**: auto/external/inproc 切替と停止保証を統合で確認する。
* **Technical Design**:
  * `image-to-markdown` をテスト入力画像で起動し、モード別に成功/失敗を検証。
* **Logic**:
  * `auto` フォールバック成功、`external` 失敗、`inproc` 成功の3系統を固定。

#### [NEW] `tests/tern_inprocess_config_test.go`(file://tests/tern_inprocess_config_test.go)
* **Description**: `settings/tern/tern-config.yaml` の読み込みと相対 path 解決を統合で検証する。
* **Technical Design**:
  * `--tern-config settings/tern/tern-config.yaml` 指定実行。
* **Logic**:
  * `model_profiles_path` が config-dir 基準で解決されることを検証。

#### [NEW] `tests/tern_inprocess_vault_integration_test.go`(file://tests/tern_inprocess_vault_integration_test.go)
* **Description**: vault-cli で投入した key を in-process Tern が利用できることを確認する。
* **Technical Design**:
  * `vault-cli set` 実行 -> `image-to-markdown --tern-mode inproc` 実行。
* **Logic**:
  * key 未設定時と設定済み時で挙動差を確認し、正しい provider key が使われることを保証。

## Step-by-Step Implementation Guide

1. **TDD Red: Tern runtime 契約を固定**
   * Edit `internal/imagetomd/tern/config_test.go` to define config loading, search order, and relative path resolution failures.
   * Edit `internal/imagetomd/tern/runtime_test.go` to define `auto|external|inproc` behavior and shutdown guarantees.
   * Edit `cmd/image-to-markdown/main_test.go` to define CLI flag validation for `--tern-mode` and `--tern-config`.

2. **Tern 設定ローダと runtime 実装**
   * Add `internal/imagetomd/tern/errors.go` to classify boot/connect/config failures.
   * Add `internal/imagetomd/tern/config.go` to parse YAML and normalize resolved paths.
   * Add `internal/imagetomd/tern/runtime.go` to boot `arctic-tern/server` in-process and return `Client + Shutdown`.

3. **公開 API / CLI 連携**
   * Edit `entext.go` to add `TernMode` and `TernConfigPath` fields and route runtime lifecycle through `ConvertImageToMarkdown`.
   * Edit `cmd/image-to-markdown/main.go` to wire `--tern-mode` and `--tern-config` into config.
   * Edit `tests/root_api_validation_test.go` for public config compile contract.

4. **設定ファイルを恒久配置へ移行**
   * Add `settings/tern/tern-config.yaml` as committed default config.
   * Edit `tmp/tern-config.yaml` to mark as sample and redirect to `settings/tern`.

5. **Vault CLI を追加**
   * Add `features/vault-cli/go.mod`.
   * Add `features/vault-cli/internal/vaultcli/command.go` and `features/vault-cli/cmd/vault-cli/main.go`.
   * Add `features/vault-cli/internal/vaultcli/command_test.go` and `features/vault-cli/cmd/vault-cli/main_test.go`.

6. **依存更新**
   * Edit `go.mod` via `go get github.com/axsh/arctic-tern@latest`.
   * Edit `go.sum` via `go mod tidy`.

7. **統合テスト追加（E2E相当）**
   * Add `tests/tern_inprocess_runtime_test.go`.
   * Add `tests/tern_inprocess_config_test.go`.
   * Add `tests/tern_inprocess_vault_integration_test.go`.

8. **Verification Plan を実行**
   * Run build first, then scoped integration test batches.
   * Record comprehensive verdict according to testing-rules §12.

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   run the build script.
   ```bash
   ./scripts/process/build.sh
   ```

2. **Integration Tests (image-to-markdown / runtime)**:
   Run integration tests after successful build.
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "InProcessTern|TernMode|AutoFallback|image_to_markdown|session|llm_image_to_markdown"
   ```
   * **Log Verification**: mode選択（external/inproc/auto-fallback）、起動ポート、shutdown実行ログ、失敗フェーズ識別が出ること。

3. **Integration Tests (config path handling)**:
   Run config-focused integration tests.
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "TernConfig|InProcessConfig|ModelProfilesPath|ConfigNotFound"
   ```
   * **Log Verification**: `model_profiles_path` の相対解決結果と設定探索順が期待どおりであること。

4. **Integration Tests (vault-cli and keyring)**:
   Run vault-related integration tests.
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "VaultCLI|Keyring|TernVaultIntegration"
   ```
   * **Log Verification**: `providers/<provider>/<name>` への書込成功、in-process 実行時に vault key が参照されること。

5. **E2E Tests (新規/追加)**:
   新機能の動作を検証するE2Eテストコードを `tests/` 配下に追加する。手動コマンド実行による確認は代替にしない。

   #### [NEW] `tests/tern_inprocess_runtime_test.go`(file://tests/tern_inprocess_runtime_test.go)
   * **テストケース**: `TestImageToMarkdownAutoFallbackToInProc`, `TestImageToMarkdownExternalModeFailsWithoutServer`, `TestImageToMarkdownInProcModeShutsDown`.
   * **検証ポイント**: 自動フォールバック、モード強制、終了時停止保証。

   #### [NEW] `tests/tern_inprocess_config_test.go`(file://tests/tern_inprocess_config_test.go)
   * **テストケース**: `TestTernConfigRelativeModelProfilesPath`, `TestTernConfigSearchOrder`.
   * **検証ポイント**: 相対パス解決と探索順の保証。

   #### [NEW] `tests/tern_inprocess_vault_integration_test.go`(file://tests/tern_inprocess_vault_integration_test.go)
   * **テストケース**: `TestVaultCLISetOpenAIKeyForTern`, `TestInProcTernUsesVaultKey`.
   * **検証ポイント**: keyring 登録と in-process 利用の接続。

### Test Item Design (Bottom-Up)

1. **C (leaf)**: `config.go`（YAML読込・相対解決）、`vaultcli/command.go`（key path 正規化）。
2. **B (middle)**: `runtime.go`（mode 判定・起動停止）、CLI flag 解析。
3. **A (top)**: `image-to-markdown` 実行統合、vault 登録から変換までの実動作。

#### 観点チェックリスト適用 (testing-rules §11.3)
- 正常系: auto/external/inproc で期待経路へ進む。
- 異常系: 無効 mode、設定未検出、起動失敗、接続失敗。
- 外部連携: keyring、arctic-tern/server、client session。
- データ一貫性: 設定読込値と実行時使用値が一致。
- 状態遷移: runtime start -> analyze -> shutdown。
- 設定反映: `--tern-config` と `model_profiles_path` 解決。
- 副作用: プロセス残存や一時ポートリークがない。

#### テスト項目セルフレビュー (testing-rules §11.4)
- **網羅性**: 仕様要件1-17を leaf->top で対応付け済み。
- **証拠十分性**: 実行結果に加えてログ・状態（shutdown/keyring path）を確認対象化。
- **迂回排除**: modeごとに primary path を明示検証し、fallback 偽成功を排除。
- **依存整合**: config/vault leaf 成功を前提に runtime/top テストを実行。

### Post-Test Comprehensive Verdict Plan (testing-rules §12)

全テスト完了後、以下の7項目（スキップ、部分エラー、迂回成功、誤適用、順序依存、カバレッジ、外部状態）を `✅/⚠️/❌` で記録し、総合判定を計画書更新として残す。`⚠️` 以上がある場合は追加検証（特定 `--specify` 再実行）を必須とする。

## Documentation

`prompts/specifications`フォルダ以下にある、既存の仕様書およびドキュメントの内容を解析し、本計画で影響を受けるものを最新の状態に更新します。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/006-InProcess-Tern-Server-Autostart.md`(file://prompts/phases/000-foundation/branches/main/ideas/006-InProcess-Tern-Server-Autostart.md)
* **更新内容**: 実装確定後の最終コマンド名・設定配置パス・vault key path ルールを反映する。

#### [MODIFY] `README.md`(file://README.md)
* **更新内容**: `image-to-markdown` の `--tern-mode` / `--tern-config`、および `settings/tern/tern-config.yaml` 運用を追記する。

#### [NEW] `features/vault-cli/README.md`(file://features/vault-cli/README.md)
* **更新内容**: keyring へのキー登録手順（openai/anthropic）と `tern-config.yaml` との対応関係を記載する。
