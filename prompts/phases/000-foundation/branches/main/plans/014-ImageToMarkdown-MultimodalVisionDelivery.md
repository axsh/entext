# 014-ImageToMarkdown-MultimodalVisionDelivery

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/015-ImageToMarkdown-MultimodalVisionDelivery.md`

## Goal Description

`image-to-markdown` が Vision 必須呼び出しで **arctic-tern v1 multimodal `SendMessage`（text + image ContentPart）** を使うよう改修し、06_List 型の「テキストパス参照のみ → Codex が workdir 探索に逸脱 → plan-only / idle timeout」問題を解消する。併せて Classify / SimpleText への Vision-only 制約、classify plan-only 時の `complex_table` フォールバック、既存 014 agent guard 契約の維持を行う。

## User Review Required

1. **classify plan-only 再試行プロンプト**: 1 回目が plan-only のとき、次の強化文を付与して再 classify する（変更希望があればレビュー時に指示）。
   > 前回は計画文のみでした。ファイル探索・shell は禁止。添付画像を Vision で直接見て、カテゴリー名（simple_text / complex_table / diagram / mixed）のみ即答してください。
2. **`AttachedImageLine` の併記**: multimodal POST で画像バイナリを送るが、プロンプト本文にも `[Attached image: <abs>]` を **残す**（014 互換・デバッグ可読性）。削除希望があれば指示すること。
3. **画像サイズ上限（任意要件）**: 初版は **20MB** 超過で `ErrImageTooLarge` を返す。上限値の変更希望があればレビュー時に指示。
4. **任意要件の先送り**: 最終統合の画像再送オプション、`idle_timeout_seconds` README 追記 — **初版は先送り**（仕様 §任意 1, 4）。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1. `SendImagePrompt` API | `tern/client.go`, `tern/errors.go` |
| A2. agent guard 同等適用 | `client.go` `sendMessageWithHandlers` 共通化 |
| A3. agentservice multimodal 整合 | `SendMessage` + `ImageFile` ContentPart |
| A4. `ErrImageReadFailed` | `tern/errors.go`, `client.go` 読込失敗時 |
| A5. `SendText` IF 維持 | `tern.Client` インターフェース不変 |
| B6. classify / simple_text / execute → SendImagePrompt | `analyzer.go` `sendImagePrompt` |
| B7. assess / question / final → SendText | `analyzer.go` 既存 `sendPrompt` |
| B8. mergeGuardEvents 継続 | `analyzer.go` 両経路 |
| C9–C10. Vision-only 制約 | `prompts.go` `VisionOnlyConstraint` |
| D11. classify plan-only → retry → complex fallback | `analyzer.go`, `session.go` `ClassifyFallbackLog` |
| D12. simple_text retry も multimodal | `analyzer.go` `runSimpleTextPath` |
| D13. 小さな表は complex_table ヒューリスティック | `prompts.go` `ClassifyPrompt` 追記 |
| E14. 014 契約維持 | 既存 tern guard テスト回帰 |
| E15. AgentGuard 統合テスト PASS | `tests/image_to_markdown_*` mock 拡張 |
| E16. arctic-tern v0.1.2+ | `go.mod`（変更なし） |
| 任意1 最終統合画像再送 | **先送り**（User Review #4） |
| 任意2 verbose `step=send_multimodal` | `client.go` Progress コールバック |
| 任意3 20MB 上限 | `client.go` `maxImageBytes`（User Review #3） |
| 任意4 idle_timeout README | **先送り** |

## Proposed Changes

### `internal/imagetomd/tern`

#### [NEW] `internal/imagetomd/tern/client_image_test.go`(file://internal/imagetomd/tern/client_image_test.go)
*   **Description**: RED テスト — multimodal 送信（TDD Step 1）。
*   **Technical Design**:
    *   ```go
        func TestSendImagePromptIncludesImageContentPart(t *testing.T)
        func TestSendImagePromptReturnsErrImageReadFailedOnMissingFile(t *testing.T)
        func TestSendImagePromptHandlesUserInputRequiredThenCompletes(t *testing.T)
        func TestSendImagePromptReturnsErrStreamStallOnIdleTimeout(t *testing.T)
        ```
*   **Logic**:
    *   **IncludesImageContentPart**: httptest `/messages` ハンドラが POST body `content` 配列に `type:"image"` と `source.media_type` を含むことを検証。レスポンス SSE で `simple_text` 返却。
    *   **MissingFile**: 存在しない path → `errors.Is(err, ErrImageReadFailed)`。
    *   **UserInputRequired**: 既存 `client_test.go` と同型の mock SSE（014 回帰）。
    *   **IdleTimeout**: `hangAfterFirstEvent` + 短 idle（`ad9040f` 後の初回イベント後監視）。

#### [MODIFY] `internal/imagetomd/tern/sse_testutil_test.go`(file://internal/imagetomd/tern/sse_testutil_test.go)
*   **Description**: multimodal POST body 検証ヘルパ追加。
*   **Technical Design**:
    *   ```go
        type capturedMessage struct {
            Content []map[string]any
        }

        func parseMessageBody(r *http.Request) (capturedMessage, error)

        func hasImageContentPart(msg capturedMessage) bool
        ```
*   **Logic**:
    *   `messages` ハンドラ内で `json.Decode` し `content` スライスを保存。テストから `hasImageContentPart` で `type == "image"` を確認。

#### [MODIFY] `internal/imagetomd/tern/errors.go`(file://internal/imagetomd/tern/errors.go)
*   **Description**: 画像読込エラー追加。
*   **Technical Design**:
    *   ```go
        var (
            ErrImageReadFailed = errors.New("tern: image file read failed")
            ErrImageTooLarge   = errors.New("tern: image file exceeds size limit")
        )
        const defaultMaxImageBytes = 20 * 1024 * 1024 // 20MB; User Review #3
        ```
*   **Logic**:
    *   `os.Stat` / `os.ReadFile` 前にサイズチェック。超過時 `ErrImageTooLarge`。

#### [MODIFY] `internal/imagetomd/tern/client.go`(file://internal/imagetomd/tern/client.go)
*   **Description**: `SendImagePrompt` 追加、`SendText` と handler ロジック共通化。
*   **Technical Design**:
    *   ```go
        type Client interface {
            CreateSession(ctx context.Context, req CreateSessionRequest) (string, error)
            SendText(ctx context.Context, sessionID string, text string) (string, error)
            SendImagePrompt(ctx context.Context, sessionID string, prompt string, imagePath string) (string, error)
            TerminateSession(ctx context.Context, sessionID string) error
            LastSendGuardEvents() []AgentGuardEvent
        }

        func (c *ArcticClient) SendImagePrompt(ctx context.Context, sessionID, prompt, imagePath string) (string, error)

        func (c *ArcticClient) sendMessageWithHandlers(
            ctx context.Context,
            sessionID string,
            parts []arcticclient.ContentPart,
            multimodalMeta struct{ imagePath string },
        ) (string, error)
        ```
*   **Logic**:
    1. `SendImagePrompt`:
        * `imagePath` を `filepath.Clean` し存在・サイズ検証。
        * `parts, err := arcticclient.NewMessage().Text(prompt).ImageFile(imagePath).Build()` — Build 失敗は `ErrImageReadFailed` に wrap。
        * `opts.Progress` 非 nil 時: `step=send_multimodal image=<basename> prompt_chars=<len(prompt)>`.
        * `sendMessageWithHandlers(ctx, sessionID, parts, {imagePath})` を呼ぶ。
    2. `SendText`:
        * `parts := []arcticclient.ContentPart{{Type:"text", Text:text}}` で `sendMessageWithHandlers` に委譲（既存 idle/guard ロジックを一箇所に集約）。
    3. `sendMessageWithHandlers`:
        * 現行 `SendText` 本体（`UnattendedInputHandler`, idle watchdog, `stream.RunWithHandlers`）を移動。
        * `session.SendMessage(ctx, parts)` → `stream.RunWithHandlers(ctx, session, handlers)`。
        * arctic-tern に `SendImageWithHandlers` が無いため entext 側で実装（仕様どおり）。

### `internal/imagetomd/analyzer`

#### [NEW] `internal/imagetomd/analyzer/prompts_test.go` 追記(file://internal/imagetomd/analyzer/prompts_test.go)
*   **Description**: RED — Vision-only / テーブルヒューリスティック（TDD Step 3）。
*   **Technical Design**:
    *   ```go
        func TestClassifyPrompt_ForbidsShellAndRequiresVision(t *testing.T)
        func TestClassifyPrompt_MentionsSmallTableAsComplexTable(t *testing.T)
        func TestBuildClassifyRetryPrompt_IncludesReinforcement(t *testing.T)
        func TestSimpleTextPrompt_ForbidsShellAndRequiresVision(t *testing.T)
        ```
*   **Logic**:
    *   出力に `shell`, `Vision`, `ファイル探索` 禁止、`添付画像を直接` が含まれること。
    *   Classify に「列見出しとデータ行からなる表形式（行数が少なくても）は complex_table」が含まれること。

#### [MODIFY] `internal/imagetomd/analyzer/prompts.go`(file://internal/imagetomd/analyzer/prompts.go)
*   **Description**: Vision-only 制約・classify 補助・retry プロンプト。
*   **Technical Design**:
    *   ```go
        const VisionOnlyConstraint = `
        - 外部ツール（OCR, tesseract, shell, ファイル探索コマンド等）の使用を禁止する。
        - 作業ディレクトリ内のファイル検索で画像を探すな。添付画像を Vision で直接読め。
        - 自身の Vision 能力のみで即答すること。`

        const ClassifyTableHeuristic = `
        補足: 列見出しとデータ行からなる表形式（行数が少なくても）は complex_table と分類すること。`

        func BuildClassifyPrompt(refContext, absPath string) string {
            return WrapNonInteractivePrompt(
                ClassifyPrompt + ClassifyTableHeuristic + VisionOnlyConstraint +
                refContext + AttachedImageLine(absPath),
            )
        }

        func BuildClassifyRetryPrompt(refContext, absPath string) string {
            return WrapNonInteractivePrompt(
                ClassifyPrompt + ClassifyTableHeuristic + VisionOnlyConstraint +
                "\n\n前回は計画文のみでした。ファイル探索・shell は禁止。添付画像を Vision で直接見て、" +
                "カテゴリー名（simple_text / complex_table / diagram / mixed）のみ即答してください。" +
                refContext + AttachedImageLine(absPath),
            )
        }

        func BuildSimpleTextPrompt(refContext, absPath string) string {
            return WrapNonInteractivePrompt(
                SimpleTextPrompt + VisionOnlyConstraint + refContext + AttachedImageLine(absPath),
            )
        }
        ```
*   **Logic**:
    *   `VisionOnlyConstraint` は `WrapNonInteractivePrompt` **前**に本文へ埋め込み（`UNATTENDED BATCH MODE` 二重付与防止）。
    *   `BuildSimpleTextRetryPrompt` にも `VisionOnlyConstraint` を追加。

#### [MODIFY] `internal/imagetomd/analyzer/session.go`(file://internal/imagetomd/analyzer/session.go)
*   **Description**: classify フォールバックログ追加。
*   **Technical Design**:
    *   ```go
        type ClassifyFallbackLog struct {
            Reason  string `json:"reason"` // plan_only | error
            Retries int    `json:"retries"`
        }

        type SessionLog struct {
            // existing fields...
            ClassifyFallback *ClassifyFallbackLog `json:"classify_fallback,omitempty"`
        }
        ```

#### [NEW] `internal/imagetomd/analyzer/analyzer_test.go` 追記(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: RED — Vision / classify fallback / 呼び出し分離（TDD Step 4）。
*   **Technical Design**:
    *   ```go
        type recordingClient struct {
            queueClient
            prompts     []string
            imagePrompts []imagePromptCall
        }

        type imagePromptCall struct {
            Prompt    string
            ImagePath string
        }

        func (c *recordingClient) SendImagePrompt(ctx context.Context, sessionID, prompt, imagePath string) (string, error)

        func TestAnalyzeClassifyUsesSendImagePrompt(t *testing.T)
        func TestAnalyzeAssessGapUsesSendTextOnly(t *testing.T)
        func TestAnalyzeClassifyFallbackToComplexOnPlanOnly(t *testing.T)
        func TestAnalyzeClassifyFallbackToComplexOnError(t *testing.T)
        func TestAnalyzeSimpleTextUsesSendImagePrompt(t *testing.T)
        ```
*   **Logic**:
    *   **ClassifyUsesSendImagePrompt**: 1 回目 `SendImagePrompt`、`SendText` は classify で呼ばれない。
    *   **AssessGapUsesSendTextOnly**: complex 経路で assess は `SendText`、execute answer は `SendImagePrompt`。
    *   **FallbackOnPlanOnly**: classify 応答が「まず画像を…」→ retry `SendImagePrompt` → 再 plan-only → `log.ClassifyFallback != nil`, `category` 実質 complex 経路（Phase 1 assess が走る）。
    *   **FallbackOnError**: `SendImagePrompt` が `ErrStreamStall` → `classify_fallback reason=error`, complex 経路へ。
    *   **SimpleTextUsesSendImagePrompt**: `simple_text` 分類時、2 回目呼び出しが `SendImagePrompt` で `imagePath` が `dummy.png` の絶対パス。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: `sendImagePrompt` 導入、classify フォールバック、呼び出し置換。
*   **Technical Design**:
    *   ```go
        func (a *Analyzer) sendImagePrompt(ctx context.Context, sessionID, prompt, imagePath string, log *SessionLog) (string, error) {
            out, err := a.client.SendImagePrompt(ctx, sessionID, prompt, imagePath)
            a.mergeGuardEvents(log)
            return out, err
        }

        func (a *Analyzer) runClassify(ctx context.Context, sessionID, absPath string, actx analyzeContext, log *SessionLog) (category string, err error)
        ```
*   **Logic**:
    1. `Analyze` 冒頭 classify を `runClassify` に抽出。
    2. `runClassify`:
        * `resp, err := sendImagePrompt(BuildClassifyPrompt(...))`
        * `err != nil`（`ErrStreamStall` 含む）→ `ClassifyFallback{Reason:"error", Retries:0}` → return `"complex_table", nil`（error は飲み込まず complex 継続; または err を返さず fallback のみ — **方針: fallback して complex 継続、progress `step=classify_fallback reason=error`**）。
        * `looksLikePlanOnly(resp)` → `sendImagePrompt(BuildClassifyRetryPrompt(...))`
        * 再試行後も plan-only または err → `ClassifyFallback{Reason:"plan_only"|"error", Retries:1}` → `"complex_table"`
        * それ以外 → `extractClassification(resp)`
    3. `runSimpleTextPath`: `sendPrompt` → `sendImagePrompt`（初回・retry 両方）。
    4. `runComplexPath` execute: `sendPrompt(answerPrompt)` → `sendImagePrompt(answerPrompt, absPath)`。
    5. assess / GenerateQuestion / final: `sendPrompt` のまま。

### `tests/`

#### [NEW] `tests/image_to_markdown_multimodal_test.go`(file://tests/image_to_markdown_multimodal_test.go)
*   **Description**: 統合テスト — 公開 API 経由で multimodal POST 契約を検証。
*   **Technical Design**:
    *   ```go
        func TestImageToMarkdownMultimodalClassifyIntegration(t *testing.T)
        ```
*   **Logic**:
    *   `newMultimodalMockTern` — `/messages` で POST body に `type:image` を要求。classify 相当で `complex_table` を返し、以降は最小 SSE で完走させるか、classify 1 回で `simple_text` + table 返却の短絡モック。
    *   `entext.ConvertImageToMarkdown` + `TernMode: external` で `| 選択 |` または category 応答を検証。
    *   既存 `image_to_markdown_agent_guard_test.go` の SSE ヘルパを共有またはコピー（tests モジュール内）。

#### [MODIFY] `tests/image_to_markdown_agent_guard_test.go`(file://tests/image_to_markdown_agent_guard_test.go)
*   **Description**: mock server が multimodal JSON body を受理するよう拡張（E15 回帰）。
*   **Logic**:
    *   `/messages` ハンドラで `content` 配列の text-only / text+image 両方を受理。既存 `TestTernClientUserInputRequiredIntegration` PASS 維持。

## Step-by-Step Implementation Guide

1.  **Step 1 — tern multimodal tests (RED)**:
    *   Add `client_image_test.go`, extend `sse_testutil_test.go` with `parseMessageBody` / `hasImageContentPart`.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc` — tern tests FAIL 確認。

2.  **Step 2 — tern SendImagePrompt + refactor (GREEN)**:
    *   Add `ErrImageReadFailed`, `ErrImageTooLarge` to `errors.go`.
    *   Refactor `client.go`: extract `sendMessageWithHandlers`; implement `SendImagePrompt`; migrate `SendText` to delegate.
    *   tern package tests PASS。

3.  **Step 3 — analyzer prompts + session (RED → GREEN)**:
    *   Add prompts tests; implement `VisionOnlyConstraint`, `ClassifyTableHeuristic`, `BuildClassifyRetryPrompt` in `prompts.go`.
    *   Add `ClassifyFallbackLog` to `session.go`.
    *   analyzer prompts tests PASS。

4.  **Step 4 — analyzer wiring + classify fallback (RED → GREEN)**:
    *   Extend `recordingClient` with `SendImagePrompt`; add analyzer tests.
    *   Implement `sendImagePrompt`, `runClassify`, replace call sites in `analyzer.go`.
    *   analyzer tests PASS。

5.  **Step 5 — integration tests**:
    *   Add `tests/image_to_markdown_multimodal_test.go`; update agent_guard mock for multimodal body.
    *   Run full Verification Plan。

6.  **Step 6 — Documentation**:
    *   Update `README.md`: multimodal 配信の説明、06_List Nightly コマンド（仕様 015 シナリオ 1）を 014 agent guard 節に追記。

7.  **Execute Verification Plan** (below).

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```
    *   **Log Verification**: `TestSendImagePromptIncludesImageContentPart`, `TestAnalyzeClassifyUsesSendImagePrompt`, `TestAnalyzeClassifyFallbackToComplexOnPlanOnly` PASS。

2.  **Integration Tests**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "ImageToMarkdown|Multimodal|AgentGuard|TernClient"
    ```
    *   **Log Verification**:
        *   `TestImageToMarkdownMultimodalClassifyIntegration` PASS（POST に `type:image`）。
        *   既存 `TestTernClientUserInputRequiredIntegration` PASS。

3.  **E2E Tests**:
    *   **LLM 実呼び出し E2E は CI 必須外**（仕様 015 / 014 と同方針）。06_List 実画像は Nightly 手順のみ README に記載。
    *   #### [NEW] `tests/image_to_markdown_multimodal_test.go`(file://tests/image_to_markdown_multimodal_test.go)
        *   **テストケース**: `TestImageToMarkdownMultimodalClassifyIntegration`
        *   **検証ポイント**: `ConvertImageToMarkdown` 経路で mock tern が multimodal POST を受信し、変換が完了する

### テスト項目設計（§11 準拠）

| 順序 | 観点 | テスト |
| :--- | :--- | :--- |
| Step 1 | 正常系（image ContentPart） | `TestSendImagePromptIncludesImageContentPart` |
| Step 1 | 異常系（ファイル不存在） | `TestSendImagePromptReturnsErrImageReadFailedOnMissingFile` |
| Step 1 | 014 回帰（user_input_required） | `TestSendImagePromptHandlesUserInputRequiredThenCompletes` |
| Step 2 | 異常系（idle stall） | `TestSendImagePromptReturnsErrStreamStallOnIdleTimeout` |
| Step 3 | 正常系（Vision-only プロンプト） | `TestClassifyPrompt_ForbidsShellAndRequiresVision` |
| Step 3 | 正常系（テーブルヒューリスティック） | `TestClassifyPrompt_MentionsSmallTableAsComplexTable` |
| Step 4 | 迂回排除（classify は image 送信） | `TestAnalyzeClassifyUsesSendImagePrompt` |
| Step 4 | 迂回排除（assess は text のみ） | `TestAnalyzeAssessGapUsesSendTextOnly` |
| Step 4 | 状態遷移（plan-only → complex） | `TestAnalyzeClassifyFallbackToComplexOnPlanOnly` |
| Step 4 | 異常系（classify error → complex） | `TestAnalyzeClassifyFallbackToComplexOnError` |
| Step 4 | simple_text multimodal | `TestAnalyzeSimpleTextUsesSendImagePrompt` |
| Integration | 公開 API + mock multimodal | `TestImageToMarkdownMultimodalClassifyIntegration` |

**§11.4 セルフレビュー結果**: 上記 PASS 時、tern（ContentPart 送信）→ analyzer（呼び出し分離・classify fallback）→ 公開 API（integration）のボトムアップ確認が完了。`hasImageContentPart` により「テキストのみ POST の迂回」を排除できる。06_List 実 LLM 成功は Nightly 任意だが、mock で multimodal 契約は CI で言い切れる。

### 総合判定プロセス（§12）

実装完了後、Verification Plan 実行後に以下を記録する:

```markdown
### 総合判定結果

**判定**: （実装者がテスト実行後に記入）

#### チェック項目
| # | 項目 | 確認方法 |
|---|------|----------|
| 1 | スキップなし | build/integration ログに SKIP なし |
| 2 | multimodal 経路 | TestSendImagePromptIncludesImageContentPart PASS |
| 3 | 014 回帰 | AgentGuard + analyzer 既存テスト PASS |
| 4 | classify fallback | TestAnalyzeClassifyFallbackToComplexOnPlanOnly PASS |
| 5 | Vision-only プロンプト | TestClassifyPrompt_ForbidsShellAndRequiresVision PASS |
| 6 | 公開 API 契約 | TestImageToMarkdownMultimodalClassifyIntegration PASS |
```

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**:
    *   `image-to-markdown` は classify / simple_text / execute で **multimodal 画像添付**（arctic-tern `SendMessage`）を使う旨を 1 段落追加。
    *   06_List Nightly 手順（仕様 015 シナリオ 1）を agent guard 節に追記:
        ```bash
        go run ./cmd/image-to-markdown --tern-mode inproc \
          --tern-config ./settings/tern/tern-config.yaml \
          -i tmp/output/pc/images/06_List_出力選択.png \
          --output-dir tmp/output/pc/md --verbose
        ```
    *   成功条件: 10 分以内終了、MD に プレナビ/44、プレ管理/47、本ナビ/50。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/015-ImageToMarkdown-MultimodalVisionDelivery.md`(file://prompts/phases/000-foundation/branches/main/ideas/015-ImageToMarkdown-MultimodalVisionDelivery.md)
*   **更新内容**: 実装計画 014 へのリンク注記を末尾に 1 行追加（実装着手後）。
