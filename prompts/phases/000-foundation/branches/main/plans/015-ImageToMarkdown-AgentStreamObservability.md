# 015-ImageToMarkdown-AgentStreamObservability

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/016-ImageToMarkdown-AgentStreamObservability.md`

## Goal Description

06_List Nightly 失敗の調査を可能にするため、**二層の観測手段**を整備する。(1) arctic-tern **trace ログ**用 committed 設定ファイルと README 手順、(2) entext **`--verbose` 時のストリーム trace**（tern `SendOptions.Progress` 接続 + `OnText` / `OnToolResult` / `OnError` の stderr 出力）。014/015 の agent guard・multimodal 契約はログ追加のみで維持する。

## User Review Required

1. **trace 設定の配置**: 既定 `settings/tern/tern-config.yaml` は `log` 未設定（info 相当）のまま維持し、**`settings/tern/tern-config-trace.yaml` を新設**して Nightly 観測時のみ `--tern-config` で指定する方針。既定 config を trace にしないことで通常実行のログ量を抑える。
2. **任意要件の先送り**: session JSON `stream_trace_events`、`--trace-stream` 独立フラグ、`OnResult` の `step=stream_done` — **初版は先送り**（stderr trace のみ）。
3. **プレビュー長**: text / tool_result は **120 runes**、stream_error は **200 runes**（仕様 016 どおり）。変更希望があればレビュー時に指示。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1 trace 有効化手順（config 新設） | `settings/tern/tern-config-trace.yaml` |
| A2 trace 時の arctic-tern 既存ログ利用 | README 説明（entext 変更不要） |
| A3 README「エージェント応答の詳細観測」 | `README.md` |
| B4 verbose 時 Progress を tern に接続 | `runtime.go`, `entext.go` |
| B5 既存 tern Progress 行の出力 | `entext.go` 配線 → 既存 `client.go` |
| B6 新規 stream trace（OnText/OnToolResult/OnError） | `stream_trace.go`, `client.go` |
| B7 verbose 時のみ trace | `entext.go`（Progress nil = 無出力） |
| B8 `[image-to-markdown] ` プレフィクス | `entext.go` 共有 Progress |
| B9 014/015 非破壊 | 既存統合テスト回帰 |
| C10 tern httptest 単体 | `client_stream_trace_test.go` |
| C11 公開 API 統合 | `tests/image_to_markdown_stream_trace_test.go` |
| 任意1 session stream_trace_events | **先送り**（User Review #2） |
| 任意2 `--trace-stream` | **先送り** |
| 任意3 `step=stream_done` | **先送り** |

## Proposed Changes

### `settings/tern`

#### [NEW] `settings/tern/tern-config-trace.yaml`(file://settings/tern/tern-config-trace.yaml)
*   **Description**: Nightly / デバッグ用 trace ログ設定（本番既定 config とは分離）。
*   **Technical Design**:
    *   ```yaml
        # Arctic Tern Server Configuration (trace / debug)
        llm_gateway:
          port: 14000
          model_profiles_path: "./model_profiles.yaml"

        vault:
          backend: keyring

        agent_service:
          disable_sandbox: true

        log:
          level: trace
          outputs:
            - type: stdout
        ```
*   **Logic**:
    *   `tern-config.yaml` の内容をベースに `log.level: trace` のみ追加。
    *   inproc 起動時 `--tern-config settings/tern/tern-config-trace.yaml` で agentservice が `SSE stream event` / `message content` /（debug 以上）`CLI stderr line` を stdout に出す。

### `internal/imagetomd/tern`

#### [NEW] `internal/imagetomd/tern/client_stream_trace_test.go`(file://internal/imagetomd/tern/client_stream_trace_test.go)
*   **Description**: RED — ストリーム trace 単体テスト（TDD Step 2）。
*   **Technical Design**:
    *   ```go
        func TestSendTextLogsStreamTextChunksViaProgress(t *testing.T)
        func TestSendTextLogsStreamToolUseViaProgress(t *testing.T)
        func TestSendTextLogsStreamToolResultViaProgress(t *testing.T)
        func TestSendTextLogsStreamErrorViaProgress(t *testing.T)
        func TestSendTextOmitsStreamTraceWhenProgressNil(t *testing.T)
        ```
*   **Logic**:
    *   **StreamText**: mock SSE が `{Type:"text", Content:"alpha"}` と `{Type:"text", Content:"beta"}` を返す → Progress に `step=stream_text` が 2 回、`preview=alpha` / `preview=beta` を含む。
    *   **StreamToolUse**: `{Type:"tool_use", ToolName:"read_file"}` → `step=stream_tool_use tool=read_file`（既存回帰 + Progress 接続確認）。
    *   **StreamToolResult**: `{Type:"tool_result", Content:"file contents here"}` → `step=stream_tool_result chars=N preview=...`。
    *   **StreamError**: `{Type:"error", Content:"something failed"}` → `step=stream_error msg=...` の後 Send 失敗。
    *   **ProgressNil**: `NewClient`（Progress なし）→ Progress ログ 0 件、応答集約は従来どおり。

#### [NEW] `internal/imagetomd/tern/stream_trace.go`(file://internal/imagetomd/tern/stream_trace.go)
*   **Description**: ストリーム trace 用定数・プレビュー整形ヘルパ。
*   **Technical Design**:
    *   ```go
        const (
            streamTextPreviewMaxRunes      = 120
            streamToolResultPreviewMaxRunes = 120
            streamErrorPreviewMaxRunes     = 200
        )

        func formatStreamLogPreview(s string, maxRunes int) string

        func emitStreamProgress(progress ProgressFunc, format string, args ...any)
        ```
*   **Logic**:
    1. `formatStreamLogPreview`:
        * `strings.TrimSpace(s)` 後、`\r` / `\n` / `\t` を **単一スペース**に置換（1 行ログ化）。
        * `truncateRunes`（`interactive.go` 既存）で maxRunes 切り詰め。
    2. `emitStreamProgress`: `progress == nil` なら no-op。否则 `progress(format, args...)`。

#### [MODIFY] `internal/imagetomd/tern/client.go`(file://internal/imagetomd/tern/client.go)
*   **Description**: `sendMessageWithHandlers` の StreamHandlers に trace ログ追加。
*   **Technical Design**:
    *   `OnText` 内（非空チャンク append 前後）:
        ```go
        emitStreamProgress(c.opts.Progress,
            "step=stream_text chars=%d preview=%s",
            len(textChunk),
            formatStreamLogPreview(textChunk, streamTextPreviewMaxRunes),
        )
        ```
    *   `OnToolResult`:
        ```go
        emitStreamProgress(c.opts.Progress,
            "step=stream_tool_result chars=%d preview=%s",
            len(content),
            formatStreamLogPreview(content, streamToolResultPreviewMaxRunes),
        )
        ```
    *   `OnError`:
        ```go
        emitStreamProgress(c.opts.Progress,
            "step=stream_error msg=%s",
            formatStreamLogPreview(errMsg, streamErrorPreviewMaxRunes),
        )
        ```
*   **Logic**:
    *   応答集約・idle watchdog・agent guard ロジックは **変更しない**。
    *   `OnToolUse` / `SendImagePrompt` の既存 Progress 行はそのまま（B5）。

#### [MODIFY] `internal/imagetomd/tern/runtime.go`(file://internal/imagetomd/tern/runtime.go)
*   **Description**: `BuildRuntime` へ Progress 注入。
*   **Technical Design**:
    *   ```go
        type RuntimeRequest struct {
            Mode           Mode
            ExternalServer string
            ConfigPath     string
            Agent          string
            Model          string
            WorkingDir     string
            Progress       ProgressFunc // NEW
        }

        func newClientForRuntime(endpoint string, progress ProgressFunc) Client {
            opts := DefaultSendOptions()
            opts.Progress = progress
            return NewClientWithSendOptions(endpoint, opts)
        }
        ```
*   **Logic**:
    1. `buildExternalRuntime` / `buildInProcRuntime` / `waitForHealth` 内の `NewClient` / `NewClientWithHTTP` を `newClientForRuntime(endpoint, req.Progress)` に置換（health check 用一時 client も Progress 付きで可 — trace 不要だが nil Progress で問題なし）。
    2. `Progress == nil` 時は従来と同じ（ストリーム trace 無出力）。

#### [MODIFY] `internal/imagetomd/tern/runtime_test.go`(file://internal/imagetomd/tern/runtime_test.go)
*   **Description**: Progress が client に渡ることの smoke テスト（mock server 不要の compile-time / 軽量テストは client_stream_trace で担保。runtime は `newClientForRuntime` が `SendOptions.Progress` をセットすることを直接テスト可）。

#### [MODIFY] `internal/imagetomd/tern/sse_testutil_test.go`(file://internal/imagetomd/tern/sse_testutil_test.go)
*   **Description**: trace テスト用 SSE イベント拡張（必要なら `tool_result` / `error` 型を mock stream に追加）。

### `entext.go`

#### [MODIFY] `entext.go`(file://entext.go)
*   **Description**: verbose 用 Progress を analyzer と `BuildRuntime` で共有。
*   **Technical Design**:
    *   ```go
        var progress tern.ProgressFunc
        if cfg.Verbose && !cfg.Quiet {
            progress = func(format string, args ...any) {
                _, _ = fmt.Fprintf(os.Stderr, "[image-to-markdown] "+format+"\n", args...)
            }
        }

        runtime, err := tern.BuildRuntime(ctx, tern.RuntimeRequest{
            Mode:           tern.Mode(cfg.TernMode),
            ExternalServer: cfg.ServerURL,
            ConfigPath:     cfg.TernConfigPath,
            Agent:          cfg.Agent,
            Model:          cfg.Model,
            WorkingDir:     ".",
            Progress:       progress,
        })

        an := analyzer.New(client, cfg.Agent, cfg.Model, analyzer.AnalyzeOptions{
            // ...
            Progress: progress, // 同一関数
        })
        ```
*   **Logic**:
    *   `cfg.Quiet` 時は `progress == nil` → analyzer / tern とも trace 無出力（B7）。
    *   既存 `runtime_ready` 行は `progress` 定義後に維持。

### `tests/`

#### [NEW] `tests/image_to_markdown_stream_trace_test.go`(file://tests/image_to_markdown_stream_trace_test.go)
*   **Description**: 統合テスト — 公開 API + mock tern + Verbose で stderr trace 検証。
*   **Technical Design**:
    *   ```go
        func TestImageToMarkdownStreamTraceIntegration(t *testing.T)

        func captureStderr(t *testing.T, fn func()) string
        ```
*   **Logic**:
    1. `newMultimodalMockTern` を再利用。messageStreams:
        * 1 回目 classify: `[{Type:"text", Content:"simple_text"}, {Type:"result"}]`
        * 2 回目 simple: `[{Type:"text", Content:"| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |"}, {Type:"result"}]`
    2. `captureStderr` で `os.Stderr` を pipe 差し替え、`entext.ConvertImageToMarkdown(..., Verbose: true)` 実行。
    3. stderr に以下を含むこと:
        * `step=send_multimodal`
        * `step=stream_text`（`preview=` に `simple_text` 断片）
        * `step=classify_done`（analyzer 側）
    4. `Verbose: false` の対照テストは `TestSendTextOmitsStreamTraceWhenProgressNil`（単体）で担保。統合では quiet 相当を省略可。

## Step-by-Step Implementation Guide

1.  **Step 1 — Tern trace config + README（設定のみ）**:
    *   Add `settings/tern/tern-config-trace.yaml`.
    *   Update `README.md`「エージェント応答の詳細観測」節（verbose / trace config / tee 例 / rollout jsonl 条件）。
    *   Commit.

2.  **Step 2 — stream trace unit tests (RED)**:
    *   Add `client_stream_trace_test.go`; extend `sse_testutil_test.go` if needed for `tool_result` / `error` SSE types.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc` — tern tests FAIL 確認。

3.  **Step 3 — stream trace implementation (GREEN)**:
    *   Add `stream_trace.go`; modify `client.go` handlers.
    *   tern package stream trace tests PASS。

4.  **Step 4 — Runtime Progress wiring**:
    *   Modify `runtime.go` (`RuntimeRequest.Progress`, `newClientForRuntime`).
    *   Modify `entext.go` shared Progress。
    *   Run build — existing tern/analyzer tests PASS。

5.  **Step 5 — Integration test**:
    *   Add `tests/image_to_markdown_stream_trace_test.go`.
    *   Run Verification Plan。

6.  **Step 6 — Documentation link**:
    *   Update spec `016` 末尾の Implementation plan リンク。

7.  **Execute Verification Plan** (below).

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```
    *   **Log Verification**: `TestSendTextLogsStreamTextChunksViaProgress`, `TestSendTextLogsStreamToolResultViaProgress`, `TestSendTextOmitsStreamTraceWhenProgressNil` PASS。

2.  **Integration Tests**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "ImageToMarkdown|StreamTrace|TernClient|Multimodal|AgentGuard"
    ```
    *   **Log Verification**:
        *   `TestImageToMarkdownStreamTraceIntegration` PASS（stderr に `step=stream_text` + `step=send_multimodal`）。
        *   既存 `TestImageToMarkdownMultimodalClassifyIntegration` / `TestTernClientUserInputRequiredIntegration` PASS。

3.  **E2E Tests**:
    *   **LLM 実呼び出し E2E は CI 必須外**（仕様 016 シナリオ 3 — Nightly 手動観測のみ）。
    *   #### [NEW] `tests/image_to_markdown_stream_trace_test.go`(file://tests/image_to_markdown_stream_trace_test.go)
        *   **テストケース**: `TestImageToMarkdownStreamTraceIntegration`
        *   **検証ポイント**: mock tern 経由で `ConvertImageToMarkdown` + `Verbose: true` 時、stderr に entext ストリーム trace が出力される

### テスト項目設計（§11 準拠）

| 順序 | 観点 | テスト |
| :--- | :--- | :--- |
| Step 2 | 正常系（OnText trace） | `TestSendTextLogsStreamTextChunksViaProgress` |
| Step 2 | 正常系（OnToolUse 既存 + 接続） | `TestSendTextLogsStreamToolUseViaProgress` |
| Step 2 | 正常系（OnToolResult trace） | `TestSendTextLogsStreamToolResultViaProgress` |
| Step 2 | 異常系（OnError trace） | `TestSendTextLogsStreamErrorViaProgress` |
| Step 2 | verbose オフ相当（Progress nil） | `TestSendTextOmitsStreamTraceWhenProgressNil` |
| Step 5 | 公開 API + shared Progress | `TestImageToMarkdownStreamTraceIntegration` |
| Regression | 014/015 multimodal + guard | 既存 Multimodal / AgentGuard 統合 PASS |

**§11.4 セルフレビュー結果**: 上記 PASS 時、tern（OnText/OnToolResult trace）→ runtime/entext（Progress 配線）→ 公開 API（stderr キャプチャ統合）のボトムアップ確認が完了。Progress nil テストにより「verbose なしでのログ漏れ」も排除できる。Tern trace config は README + committed yaml で Nightly 手動観測可能（CI では entext trace のみ自動化）。

### 総合判定プロセス（§12）

実装完了後、Verification Plan 実行後に以下を記録する:

```markdown
### 総合判定結果

**判定**: （実装者がテスト実行後に記入）

#### チェック項目
| # | 項目 | 確認方法 |
|---|------|----------|
| 1 | スキップなし | build/integration ログに SKIP なし |
| 2 | stream_text trace | TestSendTextLogsStreamTextChunksViaProgress PASS |
| 3 | Progress 配線 | TestImageToMarkdownStreamTraceIntegration PASS |
| 4 | 014/015 回帰 | Multimodal + AgentGuard 統合 PASS |
| 5 | trace config | tern-config-trace.yaml 存在 + README 手順 |
| 6 | verbose-off 無出力 | TestSendTextOmitsStreamTraceWhenProgressNil PASS |
```

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**:
    *   `image-to-markdown` 節に **「エージェント応答の詳細観測」** サブセクション追加:
        *   通常: `--verbose` → パイプライン `step=*` + entext ストリーム `step=stream_*` / `step=send_multimodal` / `step=stream_tool_use` / `step=agent_guard`
        *   詳細（Tern サーバー）: `--tern-config settings/tern/tern-config-trace.yaml`
        *   Nightly tee 例（仕様 016 シナリオ 3 コマンド）
        *   `.codex/sessions/.../rollout-*.jsonl` は Codex セッション完走時に残る旨

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/016-ImageToMarkdown-AgentStreamObservability.md`(file://prompts/phases/000-foundation/branches/main/ideas/016-ImageToMarkdown-AgentStreamObservability.md)
*   **更新内容**: 実装計画 015 へのリンク注記を末尾に 1 行追加（実装着手後）。
