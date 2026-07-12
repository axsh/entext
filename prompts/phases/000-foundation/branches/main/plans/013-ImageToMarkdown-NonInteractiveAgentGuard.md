# 013-ImageToMarkdown-NonInteractiveAgentGuard

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/014-ImageToMarkdown-NonInteractiveAgentGuard.md`

## Goal Description

`image-to-markdown` パイプラインを **無人バッチ実行**として安定化する。`arctic-tern v0.1.2` の `user_input_required` イベントと `SendTextWithHandlers` / `Session.Respond` を entext `tern` クライアントに統合し、06_List 型の無限ハングを解消する。併せて analyzer 側で非対話プロンプトの横断付与、`simple_text` ショートパスの画像再添付、テキストベース interactive 検知、complex_table 降格を実装する。

## User Review Required

1. **自動応答の既定文**: 自由記述 `user_input_required` には次の固定文を返す。変更希望があればレビュー時に指示すること。
   > 無人バッチ実行です。確認や追加質問は不要です。添付画像を忠実に Markdown 化し、質問せずテーブルまたはリストを即時出力してください。
2. **`Choices` 付き質問**: 決定不能時は **先頭選択肢**を返し、verbose に `step=agent_guard kind=user_input_required choices=<n> picked=0` を記録する（仕様「決定不能なら先頭 + verbose 警告」）。
3. **タイムアウト既定値**: 総タイムアウト **600s**、アイドルタイムアウト **120s**（イベント更新なし）。開発・CI 用に `SendOptions` で短縮注入可能。
4. **自動応答上限**: 1 `SendText` あたり **3 回**。4 回目で `ErrInteractiveInputRequired`。simple_text では analyzer がこれを捕捉し complex_table 降格（stall エラーは降格しない）。
5. **任意要件（仕様 §任意）**: `OnSystem`/`node_*` ログ、Codex approval 設定注入、Terminate 後リトライ、メトリクス JSON、Choices の session 保存 — **初版は先送り**。`agent_guard` に `prompt_id` / `content_prefix` / `auto_response_count` のみ記録。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1. 全 LLM 呼び出しに非対話サフィックス | `prompts.go` `WrapNonInteractivePrompt`, `analyzer.go` 全 `SendText` 前 |
| A2. 単一定数、`ExecutionQuestionSuffix` 統合 | `prompts.go` `NonInteractiveExecutionSuffix` |
| A3–A4. 禁止/必須事項 | `NonInteractiveExecutionSuffix` 本文 |
| A5. simple_text に画像+refContext+サフィックス | `analyzer.go` `buildSimpleTextPrompt` |
| A6. 007–013 契約維持 | 既存プロンプト本文は変更せずサフィックス追加のみ |
| B7. SendTextWithHandlers ベース移行 | `tern/client.go` |
| B8. OnUserInputRequired 自動応答 | `tern/interactive.go` `UnattendedInputHandler` |
| B9. user_input_required ログ記録 | `interactive.go` + `analyzer.go` `recordAgentGuard` |
| B10. OnToolUse/OnToolResult verbose | `client.go` handlers + Progress callback |
| B11. looksLikeInteractiveQuestion 補助 | `quality.go` |
| B12. simple_text → complex_table 降格 | `analyzer.go` `runSimpleTextPath` / `fallbackToComplexPath` |
| C13–C15. タイムアウト、ErrStreamStall | `tern/send_options.go`, `client.go` context deadline |
| C16. AnalyzeOptions/SendOptions 上書き | `send_options.go`, `NewClientWithOptions` |
| C17. stall 時は降格しない | `analyzer.go` error 分岐 |
| D18–D19. CLI/API 同一、Client IF 維持 | `entext.go` 経由、`tern.Client` シグネチャ不変 |
| D20. verbose agent_guard progress | `analyzer.go` / `client.go` Progress |
| D21. 007/008 回帰 | 既存 analyzer テスト + integration_test |
| D22. arctic-tern v0.1.2+ | `go.mod`（反映済み） |
| 任意1–5 | **先送り**（User Review #5） |

## Proposed Changes

### `internal/imagetomd/tern`

#### [NEW] `internal/imagetomd/tern/errors.go`(file://internal/imagetomd/tern/errors.go)
*   **Description**: typed errors for stall and interactive limit.
*   **Technical Design**:
    *   ```go
        var (
            ErrStreamStall              = errors.New("tern: stream stalled waiting for completion")
            ErrInteractiveInputRequired = errors.New("tern: user input required limit exceeded")
        )
        ```
*   **Logic**:
    *   `errors.Is(err, ErrStreamStall)` / `ErrInteractiveInputRequired` で analyzer が分岐可能にする。

#### [NEW] `internal/imagetomd/tern/send_options.go`(file://internal/imagetomd/tern/send_options.go)
*   **Description**: per-client SendText 設定。
*   **Technical Design**:
    *   ```go
        type ProgressFunc func(format string, args ...any)

        type SendOptions struct {
            TotalTimeout      time.Duration // default 600s
            IdleTimeout       time.Duration // default 120s; 0 disables idle watchdog
            MaxAutoResponses  int           // default 3
            Progress          ProgressFunc  // optional; verbose agent_guard / tool_use
        }

        func DefaultSendOptions() SendOptions {
            return SendOptions{
                TotalTimeout:     600 * time.Second,
                IdleTimeout:      120 * time.Second,
                MaxAutoResponses: 3,
            }
        }
        ```
*   **Logic**:
    *   `ArcticClient` に `opts SendOptions` フィールドを追加。`NewClient` / `NewClientWithHTTP` は `DefaultSendOptions()` を使用。

#### [NEW] `internal/imagetomd/tern/interactive_test.go`(file://internal/imagetomd/tern/interactive_test.go)
*   **Description**: RED テスト — 無人自動応答ポリシー（TDD Step 1）。
*   **Technical Design**:
    *   ```go
        func TestUnattendedInputHandler_AutoResponseFreeText(t *testing.T)
        func TestUnattendedInputHandler_PicksFirstChoice(t *testing.T)
        func TestUnattendedInputHandler_ExceedsMaxReturnsError(t *testing.T)
        func TestUnattendedInputHandler_RecordsGuardEvents(t *testing.T)
        ```
*   **Logic**:
    *   `MaxAutoResponses=3` で 4 回目 `Handle(ev)` が `( "", ErrInteractiveInputRequired)`。
    *   `Choices: []string{"A","B"}` → 返却 `"A"`。
    *   自由記述 → 固定文（User Review #1）を返却。

#### [NEW] `internal/imagetomd/tern/interactive.go`(file://internal/imagetomd/tern/interactive.go)
*   **Description**: `OnUserInputRequired` 実装本体。
*   **Technical Design**:
    *   ```go
        const UnattendedAutoResponse = "無人バッチ実行です。確認や追加質問は不要です。添付画像を忠実に Markdown 化し、質問せずテーブルまたはリストを即時出力してください。"

        type AgentGuardEvent struct {
            Kind          string `json:"kind"` // "user_input_required"
            PromptID      string `json:"prompt_id,omitempty"`
            ContentPrefix string `json:"content_prefix,omitempty"` // first 120 runes
            ChoicesCount  int    `json:"choices_count,omitempty"`
            PickedIndex   int    `json:"picked_index,omitempty"` // -1 if free text
            AutoResponse  bool   `json:"auto_response"`
        }

        type UnattendedInputHandler struct {
            maxResponses int
            progress     ProgressFunc
            events       []AgentGuardEvent
        }

        func NewUnattendedInputHandler(maxResponses int, progress ProgressFunc) *UnattendedInputHandler

        func (h *UnattendedInputHandler) Handle(ev arcticclient.UserInputRequiredEvent) (string, error)

        func (h *UnattendedInputHandler) Events() []AgentGuardEvent
        ```
*   **Logic**:
    *   呼び出しごとに `autoResponseCount++`。`> maxResponses` なら `ErrInteractiveInputRequired`。
    *   `len(ev.Choices) > 0` → `response = ev.Choices[0]`, `PickedIndex=0`。
    *   それ以外 → `response = UnattendedAutoResponse`, `PickedIndex=-1`。
    *   `ContentPrefix` = `ev.Content` の先頭 120 文字（UTF-8 rune 安全 truncate）。
    *   `progress` 非 nil 時: `step=agent_guard kind=user_input_required prompt_id=%s choices=%d auto_response=true`。

#### [NEW] `internal/imagetomd/tern/sse_testutil_test.go`(file://internal/imagetomd/tern/sse_testutil_test.go)
*   **Description**: httptest 用 SSE ストリームビルダ（TDD Step 2 支援）。
*   **Technical Design**:
    *   ```go
        func writeSSE(w io.Writer, events []sseEvent) // sseEvent{Type, Content, ToolName, PromptID, Choices}
        func newTestArcticServer(handler func(w http.ResponseWriter, r *http.Request)) *httptest.Server
        ```
*   **Logic**:
    *   arctic-tern SSE 形式: `data: {"type":"text","content":"..."}\n\n` … 最終 `data: [DONE]\n\n`。
    *   `user_input_required` 例:
        ```json
        {"type":"user_input_required","content":"Which column?","prompt_id":"p1","choices":["colA","colB"]}
        ```
    *   続行ストリームは別 HTTP ハンドラ（`/respond`）で返す。

#### [NEW] `internal/imagetomd/tern/client_test.go`(file://internal/imagetomd/tern/client_test.go)
*   **Description**: RED テスト — SendTextWithHandlers 統合（TDD Step 2）。既存 `client_response_test.go` は `finalizeResponse` のみなので別ファイル。
*   **Technical Design**:
    *   ```go
        func TestSendTextHandlesUserInputRequiredThenCompletes(t *testing.T)
        func TestSendTextReturnsErrInteractiveInputRequiredOnFourthPrompt(t *testing.T)
        func TestSendTextReturnsErrStreamStallOnIdleTimeout(t *testing.T)
        func TestSendTextRespectsTotalDeadline(t *testing.T)
        func TestSendTextRecordsToolUseViaProgress(t *testing.T)
        ```
*   **Logic**:
    *   **HandlesUserInputRequired**: messages → `user_input_required` → respond → `text` + `[DONE]`。最終本文にテーブル行が含まれる。
    *   **FourthPrompt**: 連続 4 回 `user_input_required` → `ErrInteractiveInputRequired`。
    *   **IdleTimeout**: `[DONE]` なし・イベント無し → 短 `IdleTimeout` で `ErrStreamStall`。
    *   **TotalDeadline**: 長い sleep ストリーム → 短 `TotalTimeout` で `ErrStreamStall`。
    *   **ToolUse**: `tool_use` イベント → Progress に `step=stream_tool_use tool=...`。

#### [MODIFY] `internal/imagetomd/tern/client.go`(file://internal/imagetomd/tern/client.go)
*   **Description**: `SendText` を `SendTextWithHandlers` ベースへ全面移行。旧 `stream.Run()` 経路を削除。
*   **Technical Design**:
    *   ```go
        type ArcticClient struct {
            client   *arcticclient.Client
            mu       sync.Mutex
            sessions map[string]*arcticclient.Session
            opts     SendOptions
        }

        func NewClientWithSendOptions(baseURL string, opts SendOptions) *ArcticClient
        ```
    *   `Client` インターフェースは変更しない:
        ```go
        SendText(ctx context.Context, sessionID string, text string) (string, error)
        ```
*   **Logic**:
    1. `CreateSession` 後、session を map に保持（現行どおり）。
    2. `SendText`:
        * `ctx, cancel := context.WithTimeout(ctx, c.opts.TotalTimeout)` — 親 ctx がより短い場合は親を尊重（`context.WithDeadline` で min）。
        * `var texts []string`; `handler := NewUnattendedInputHandler(c.opts.MaxAutoResponses, c.opts.Progress)`。
        * `handlers := arcticclient.StreamHandlers{ OnText: append non-empty chunks, OnToolUse: log, OnToolResult: optional log, OnUserInputRequired: func(ev) { return handler.Handle(ev) }, OnError: return fmt.Errorf, OnResult: markDone }`。
        * **アイドル watchdog**: goroutine で `IdleTimeout` ごとに最終イベント時刻を確認。超過時 cancel ctx（→ `ErrStreamStall` にラップ）。
        * `session.SendTextWithHandlers(ctx, text, handlers)` を呼ぶ。arctic-tern が内部で `Respond` ループを実行。
        * 戻り値: `finalizeResponse(texts, "")` + handler.Events() を client 側に一時保持（analyzer が session log へ写すため `LastSendGuardEvents()` メソッドを追加してもよい）。
    3. `NewClient`: `WithNoTimeout()` を **廃止**。代わりに `http.Client{Timeout: 0}` + **SendText 単位の context deadline**（HTTP 全体は stream 長時間対応、deadline は ctx で制御）。または `WithHTTPClient(&http.Client{Timeout: 0})` + ctx。
    4. `OnResult` の `ev.Text` フォールバックは v0.1.2 `StreamHandlers.OnResult func()` に変更されているため、本文は **OnText のみ**集約（`finalizeResponse` は OnText 優先のまま）。

#### [MODIFY] `internal/imagetomd/tern/client_response_test.go`(file://internal/imagetomd/tern/client_response_test.go)
*   **Description**: 既存 finalizeResponse テスト維持（回帰）。

### `internal/imagetomd/analyzer`

#### [NEW] `internal/imagetomd/analyzer/quality_test.go` 追記 / `prompts_test.go` 新規
*   **Description**: RED — プロンプト・品質ガード（TDD Step 3）。
*   **Technical Design**:
    *   `prompts_test.go`:
        ```go
        func TestWrapNonInteractivePrompt_AppendsSuffixOnce(t *testing.T)
        func TestWrapNonInteractivePrompt_Idempotent(t *testing.T)
        func TestBuildSimpleTextPrompt_IncludesImageAndSuffix(t *testing.T)
        func TestAssessGapPrompt_IncludesNonInteractiveSuffix(t *testing.T)
        ```
    *   `quality_test.go` 追記:
        ```go
        func TestLooksLikeInteractiveQuestion_DetectsJapaneseConfirm(t *testing.T)
        func TestLooksLikeInteractiveQuestion_IgnoresTableWithQuestionMark(t *testing.T)
        func TestLooksLikeInteractiveQuestion_DetectsEnglishCouldYou(t *testing.T)
        ```
*   **Logic**:
    *   `looksLikeInteractiveQuestion` 条件（仕様 B11）:
        1. trim 後空 → false
        2. `|` を含む Markdown テーブル行が 1 行以上 → false（データ出力とみなす）
        3. `- ` で始まるリスト行が 2 行以上 → false
        4. 以下いずれか → true:
           * `(?i)(確認してください|教えてください|どちら|選択してください|please confirm|which one|could you)`
           * 末尾 `?` or `？` かつ len < 200
           * `(?i)\b(y/n|yes/no)\b`

#### [MODIFY] `internal/imagetomd/analyzer/prompts.go`(file://internal/imagetomd/analyzer/prompts.go)
*   **Description**: 非対話サフィックス統合。`ExecutionQuestionSuffix` を置換。
*   **Technical Design**:
    *   ```go
        const NonInteractiveExecutionSuffix = `

        **CRITICAL — UNATTENDED BATCH MODE**
        - Do NOT ask questions or request confirmation. No human is available to answer.
        - Do NOT plan, explain, or say "I will…" / "確認します". Output the requested data immediately.
        - If information seems ambiguous, choose the most faithful transcription from the attached image and proceed.
        - Output ONLY the requested format (Markdown table / list / category name / SUFFICIENT|INSUFFICIENT).`

        // ExecutionQuestionSuffix is deprecated alias for backward compat within package.
        const ExecutionQuestionSuffix = NonInteractiveExecutionSuffix

        func WrapNonInteractivePrompt(base string) string {
            if strings.Contains(base, "UNATTENDED BATCH MODE") {
                return base
            }
            return strings.TrimRight(base, "\n") + NonInteractiveExecutionSuffix
        }

        func BuildSimpleTextPrompt(refContext, absPath string) string {
            return WrapNonInteractivePrompt(
                SimpleTextPrompt + refContext + AttachedImageLine(absPath),
            )
        }

        func BuildClassifyPrompt(refContext, absPath string) string {
            return WrapNonInteractivePrompt(
                ClassifyPrompt + refContext + AttachedImageLine(absPath),
            )
        }
        ```
*   **Logic**:
    *   `AssessGapPrompt` 末尾の `return b.String()` 前に `WrapNonInteractivePrompt` は **適用しない**（関数内で `b.WriteString(NonInteractiveExecutionSuffix)` を追加 — 二重付与防止）。
    *   `GenerateQuestionPrompt` / `GenerateMarkdownPrompt` / `GenerateMarkdownRetryPrompt` の return 前に `WrapNonInteractivePrompt` を適用。

#### [MODIFY] `internal/imagetomd/analyzer/quality.go`(file://internal/imagetomd/analyzer/quality.go)
*   **Description**: `looksLikeInteractiveQuestion` 追加。
*   **Technical Design**:
    *   ```go
        func looksLikeInteractiveQuestion(text string) bool
        func hasMarkdownDataLines(text string) bool // |...| or - list
        ```

#### [MODIFY] `internal/imagetomd/analyzer/session.go`(file://internal/imagetomd/analyzer/session.go)
*   **Description**: SessionLog に agent guard / fallback フィールド追加。
*   **Technical Design**:
    *   ```go
        type AgentGuardLog struct {
            Kind          string `json:"kind,omitempty"`
            PromptID      string `json:"prompt_id,omitempty"`
            ContentPrefix string `json:"content_prefix,omitempty"`
            ChoicesCount  int    `json:"choices_count,omitempty"`
            AutoResponses int    `json:"auto_responses,omitempty"`
            Reason        string `json:"reason,omitempty"`
        }

        type SimpleTextFallbackLog struct {
            Reason   string `json:"reason"` // interactive_text | plan_only | empty | interactive_limit
            Retries  int    `json:"retries"`
        }

        type SessionLog struct {
            // existing fields...
            AgentGuardEvents  []AgentGuardLog        `json:"agent_guard_events,omitempty"`
            SimpleTextFallback *SimpleTextFallbackLog `json:"simple_text_fallback,omitempty"`
        }
        ```

#### [NEW] `internal/imagetomd/analyzer/analyzer_test.go` 追記
*   **Description**: RED — simple_text プロンプト・降格（TDD Step 4）。
*   **Technical Design**:
    *   ```go
        func TestAnalyzeSimpleTextPromptIncludesNonInteractiveSuffixAndImage(t *testing.T)
        func TestAnalyzeSimpleTextRetriesOnInteractiveQuestion(t *testing.T)
        func TestAnalyzeSimpleTextFallsBackToComplexTable(t *testing.T)
        func TestAnalyzeSimpleTextDoesNotFallbackOnStreamStall(t *testing.T)
        func TestSessionLogRecordsAgentGuardEvents(t *testing.T)
        ```
*   **Logic**:
    *   **PromptIncludes**: recordingClient で classify=`simple_text` → 2 回目 prompt に `Attached image:` + `UNATTENDED BATCH MODE`。
    *   **Retries**: 2 回目 `Could you confirm?` → 3 回目 valid table → success、SendText 3 回。
    *   **FallsBack**: 2 回目・3 回目 interactive → Phase 1 assess 応答が queue に必要 → `log.SimpleTextFallback != nil`, `log.ShortPath == false`。
    *   **NoFallbackOnStall**: mock client が `ErrStreamStall` → error return、fallback なし。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: simple_text 経路刷新、全 SendText プロンプト統一、降格ロジック。
*   **Technical Design**:
    *   ```go
        func (a *Analyzer) sendPrompt(ctx context.Context, sessionID, prompt string) (string, error)

        func (a *Analyzer) runSimpleTextPath(ctx context.Context, sessionID, absPath string, actx analyzeContext, log *SessionLog) (string, error)

        func (a *Analyzer) runComplexPath(...) (string, error) // 既存 for-loop + final synthesis を抽出

        func (a *Analyzer) mergeTernGuardEvents(log *SessionLog, events []tern.AgentGuardEvent)

        func (a *Analyzer) isSimpleTextOutputInsufficient(md string) (bool, string)
        ```
*   **Logic**:
    1. **classify**: `BuildClassifyPrompt(actx.refContext, absPath)` に変更。
    2. **simple_text** (`runSimpleTextPath`):
        * `prompt := BuildSimpleTextPrompt(actx.refContext, absPath)`
        * `md, err := a.sendPrompt(...)` — 成功後 `mergeTernGuardEvents`
        * `isSimpleTextOutputInsufficient(md)`:
           * empty → reason `empty`
           * `looksLikePlanOnly(md)` → `plan_only`
           * `looksLikeInteractiveQuestion(md)` → `interactive_text`
        * insufficient かつ retry==0 → 強化プロンプト:
           ```go
           reinforced := WrapNonInteractivePrompt(
               SimpleTextPrompt + "\n\n前回の応答は不十分でした。質問や確認は禁止。添付画像の全テキストを Markdown テーブルで即時出力してください。" +
               actx.refContext + AttachedImageLine(absPath))
           ```
        * retry 後も insufficient → `log.SimpleTextFallback = &SimpleTextFallbackLog{Reason: ..., Retries: 1}` → **`category` は log に残しつつ** `runComplexPath` へ（`simple_text` ショートパスは使わない）。
        * `errors.Is(err, tern.ErrInteractiveInputRequired)` → fallback reason `interactive_limit`
        * `errors.Is(err, tern.ErrStreamStall)` → **そのまま error return**（降格しない）
    3. **execute**: 既存 `ExecutionQuestionSuffix` → `NonInteractiveExecutionSuffix`（同一定数）。
    4. **AssessGap / GenerateQuestion / Final**: `WrapNonInteractivePrompt` 適用済み関数を使用。
    5. **progress**: fallback 時 `step=simple_text_fallback reason=%s`。

### `tests/`

#### [NEW] `tests/image_to_markdown_agent_guard_test.go`(file://tests/image_to_markdown_agent_guard_test.go)
*   **Description**: 統合テスト — 公開 API 経路の契約（LLM 非依存部分）。
*   **Technical Design**:
    *   ```go
        func TestImageToMarkdownAgentGuardInvalidModeStillFails(t *testing.T) // 既存パターン回帰
        func TestTernClientUserInputRequiredIntegration(t *testing.T) // tern パッケージの httptest server を直接利用
        ```
*   **Logic**:
    *   実 LLM / inproc tern は **CI 必須外**（仕様どおり）。理由: 決定的挙動は `internal/imagetomd/tern` の httptest でカバー。CLI 契約は既存 `image_to_markdown_logging_test.go` で維持。
    *   `TestTernClientUserInputRequiredIntegration`: `NewClientWithSendOptions(testServerURL, short timeouts)` で 06_List 相当 SSE（user_input_required → table text）を返し、`| 選択 |` を含む本文が得られること。

## Step-by-Step Implementation Guide

1.  **Step 1 — tern errors + send_options + interactive (RED → GREEN)**:
    *   Add `errors.go`, `send_options.go`, `interactive_test.go`, `interactive.go`.
    *   Implement `UnattendedInputHandler` per User Review #1–#4.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc` (tern package tests).

2.  **Step 2 — SSE testutil + client SendTextWithHandlers (RED → GREEN)**:
    *   Add `sse_testutil_test.go`, `client_test.go`.
    *   Rewrite `client.go` `SendText` to use `SendTextWithHandlers`.
    *   Remove `WithNoTimeout()`; use context deadlines.
    *   build PASS for `./internal/imagetomd/tern/...`.

3.  **Step 3 — analyzer prompts + quality (RED → GREEN)**:
    *   Add `prompts_test.go`, extend `quality_test.go`.
    *   Modify `prompts.go`, `quality.go`.
    *   build PASS for analyzer unit tests.

4.  **Step 4 — session log + analyzer simple_text/fallback (RED → GREEN)**:
    *   Extend `session.go` structs.
    *   Add analyzer tests; implement `runSimpleTextPath`, classify/build prompt changes in `analyzer.go`.
    *   Refactor complex path into callable block for fallback.
    *   build PASS.

5.  **Step 5 — integration test**:
    *   Add `tests/image_to_markdown_agent_guard_test.go`.
    *   Run full Verification Plan.

6.  **Step 6 — Documentation**:
    *   Update `cmd/image-to-markdown` help text or project README section if present.

7.  **Execute Verification Plan** (below).

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```
    *   **Log Verification**: `internal/imagetomd/tern` の `TestSendTextHandlesUserInputRequiredThenCompletes`, `TestSendTextReturnsErrStreamStallOnIdleTimeout` PASS。`analyzer` の `TestAnalyzeSimpleTextFallsBackToComplexTable` PASS。

2.  **Integration Tests**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "ImageToMarkdown|imagetomd|image-to-markdown|AgentGuard"
    ```
    *   **Log Verification**:
        *   既存 `ImageToMarkdown` / logging / csv_hint 回帰 PASS。
        *   新規 `TestTernClientUserInputRequiredIntegration` PASS。

3.  **E2E Tests**:
    *   **LLM 実呼び出し E2E は CI 必須としない**（仕様 Nightly 任意）。理由: `user_input_required` ループは httptest SSE スタブで決定的に検証可能。06_List 実画像は Nightly 手順のみ README に記載。
    *   #### [NEW] `tests/image_to_markdown_agent_guard_test.go`(file://tests/image_to_markdown_agent_guard_test.go)
        *   **テストケース**: `TestTernClientUserInputRequiredIntegration`
        *   **検証ポイント**: 自動応答後に Markdown テーブル本文が返る、4 回目 interactive で typed error

### テスト項目設計（§11 準拠）

| 順序 | 観点 | テスト |
| :--- | :--- | :--- |
| Step 1 | 正常系（自由記述自動応答） | `TestUnattendedInputHandler_AutoResponseFreeText` |
| Step 1 | 正常系（Choices 先頭） | `TestUnattendedInputHandler_PicksFirstChoice` |
| Step 1 | 異常系（上限超過） | `TestUnattendedInputHandler_ExceedsMaxReturnsError` |
| Step 2 | 正常系（Respond ループ完走） | `TestSendTextHandlesUserInputRequiredThenCompletes` |
| Step 2 | 異常系（stall） | `TestSendTextReturnsErrStreamStallOnIdleTimeout` |
| Step 2 | 境界（総 deadline） | `TestSendTextRespectsTotalDeadline` |
| Step 2 | ログ（tool_use） | `TestSendTextRecordsToolUseViaProgress` |
| Step 3 | 正常系（プロンプト付与） | `TestBuildSimpleTextPrompt_IncludesImageAndSuffix` |
| Step 3 | 正常系（interactive 検知） | `TestLooksLikeInteractiveQuestion_DetectsJapaneseConfirm` |
| Step 3 | 副作用なし（テーブル除外） | `TestLooksLikeInteractiveQuestion_IgnoresTableWithQuestionMark` |
| Step 4 | 正常系（simple_text retry） | `TestAnalyzeSimpleTextRetriesOnInteractiveQuestion` |
| Step 4 | 状態遷移（降格） | `TestAnalyzeSimpleTextFallsBackToComplexTable` |
| Step 4 | stall は降格しない | `TestAnalyzeSimpleTextDoesNotFallbackOnStreamStall` |
| Step 4 | セッションログ | `TestSessionLogRecordsAgentGuardEvents` |
| Integration | tern httptest | `TestTernClientUserInputRequiredIntegration` |

**§11.4 セルフレビュー結果**: 上記 PASS 時、末端（UnattendedInputHandler）→ client（SendTextWithHandlers）→ analyzer（simple_text/fallback）のボトムアップ確認が完了。`user_input_required` 未処理による無限ハングは Step 2 で、06_List 相当の自動応答完走は Step 2 + Step 4 で LLM なしに言い切れる。stall 降格禁止は Step 4 で担保。

### 総合判定プロセス（§12）

実装完了後、Verification Plan 実行後に以下を記録する:

```markdown
### 総合判定結果

**判定**: PASS（2026-07-12）

#### チェック項目
| # | 項目 | 結果 |
|---|------|------|
| 1 | スキップなし | build/integration ログに SKIP なし |
| 2 | 部分エラーなし | stderr に panic なし |
| 3 | primary path | SendText が SendTextWithHandlers 経由（client_test PASS） |
| 4 | 007/008 回帰 | 既存 analyzer + csv_hint 単体 PASS |
| 5 | stall 降格禁止 | TestAnalyzeSimpleTextDoesNotFallbackOnStreamStall PASS |
| 6 | simple_text プロンプト | Attached image + UNATTENDED BATCH MODE（analyzer_test PASS） |
| 7 | integration | TestTernClientUserInputRequiredIntegration PASS |
```

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**:
    *   `image-to-markdown` は無人バッチ実行であり、`arctic-tern v0.1.2+` の `user_input_required` を自動応答する旨を 1 段落追加。
    *   Nightly 検証用 CLI 例（06_List）:
        ```bash
        go run ./cmd/image-to-markdown --tern-mode inproc \
          -i tmp/output/pc/images/06_List_出力選択.png \
          --output-dir tmp/output/pc/md --verbose
        ```
    *   成功条件: 10 分以内終了、MD にプレナビ/プレ管理/本ナビ行。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/014-ImageToMarkdown-NonInteractiveAgentGuard.md`(file://prompts/phases/000-foundation/branches/main/ideas/014-ImageToMarkdown-NonInteractiveAgentGuard.md)
*   **更新内容**: 実装計画 013 へのリンク注記を末尾に 1 行追加（実装着手後）。
