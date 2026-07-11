# 007-ImageToMarkdown-ReferenceParity

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/007-ImageToMarkdown-ReferenceParity.md`

## Goal Description
`tmp/reference/image-to-markdown` を参照実装（正解）として、`internal/imagetomd` を処理フロー・応答集約・最終出力品質・inproc安定性の観点で一致させる。最終成果物は必ず「画像内容のMarkdown本文」とし、Phaseログ混入や過剰な情報欠落を防止する。

## User Review Required
1. 参照実装に合わせる範囲として、`plan-only` 判定は「警告ログ用途」に留め、本文除外には使わない方針で確定してよいか。
2. 回帰テストの正解データは `tmp/reference/image-to-markdown/output/*.md` を `tests/testdata/reference_parity/` に固定コピーして管理してよいか。
3. 動作確認容易性のため、`--verbose` 時に Phase/Round単位の詳細ログ（開始・終了・判定・文字数・再試行理由）を `stderr` へ常時出力する方針で確定してよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. 参照実装と同等の処理順序・責務 | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go`, `internal/imagetomd/analyzer/prompts.go` |
| 2. 最終成果物に解析ログを混入させない | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go`, `internal/imagetomd/analyzer/prompts.go`, `tests/image_to_markdown_reference_parity_test.go` |
| 3. `01_変更履歴` の必須文字列・行データを含む | Proposed Changes > `tests/image_to_markdown_reference_parity_test.go`, `tests/testdata/reference_parity/*` |
| 4. `02_書き換えルール` が空でなくメイン表を含む | Proposed Changes > `tests/image_to_markdown_reference_parity_test.go`, `internal/imagetomd/tern/client.go` |
| 5. `OnText` 優先の本文集約 | Proposed Changes > `internal/imagetomd/tern/client.go`, `internal/imagetomd/tern/client_response_test.go` |
| 6. 本文候補の過剰破棄を防止 | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go`, `internal/imagetomd/analyzer/fallback_test.go` |
| 7. Phase3/4拡張の上限・発火回数制御 | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go`, `internal/imagetomd/analyzer/analyzer_test.go` |
| 8. inproc起動失敗頻度低減 | Proposed Changes > `internal/imagetomd/tern/runtime.go`, `internal/imagetomd/tern/runtime_test.go` |
| 9. CLI契約維持 | Proposed Changes > `cmd/image-to-markdown/main.go`, `cmd/image-to-markdown/main_test.go` |
| 10. 公開APIでも同品質 | Proposed Changes > `entext.go`, `tests/root_api_validation_test.go` |
| 11. `--verbose` で詳細ログ統一 | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go`, `cmd/image-to-markdown/main.go` |
| 12. 固定入力ゴールデンテスト追加 | Proposed Changes > `tests/image_to_markdown_reference_parity_test.go`, `tests/testdata/reference_parity/*` |
| 13. 動作確認用ログを強化（原因追跡可能） | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go`, `internal/imagetomd/tern/runtime.go`, `cmd/image-to-markdown/main.go`, `tests/image_to_markdown_logging_test.go` |

## Proposed Changes

### `internal/imagetomd/tern`

#### [MODIFY] `internal/imagetomd/tern/client.go`(file://internal/imagetomd/tern/client.go)
*   **Description**: ストリーム応答の採用順序を参照実装準拠（`OnText` 優先）へ統一する。
*   **Technical Design**:
    *   `SendText` の戻り値構築を `OnText` 集約中心に変更。
    *   `OnResult` は補助（`OnText` が空のときのみフォールバック）として扱う。
    *   ```go
        func finalizeResponse(texts []string, resultText string) string {
            joined := strings.TrimSpace(strings.Join(texts, "\n"))
            if joined != "" {
                return joined
            }
            return strings.TrimSpace(resultText)
        }
        ```
*   **Logic**:
    *   本文テーブルが `OnText` に出る経路を最優先。
    *   `streamErr` がある場合は現行どおり失敗扱い。

#### [NEW] `internal/imagetomd/tern/client_response_test.go`(file://internal/imagetomd/tern/client_response_test.go)
*   **Description**: 応答採用順序の退行防止テストを追加する。
*   **Technical Design**:
    *   ケース1: `texts` 非空 + `resultText` 計画文 -> `texts` 採用。
    *   ケース2: `texts` 空 -> `resultText` 採用。
*   **Logic**:
    *   参照実装 `collectStreamText` 相当の契約を固定。

#### [MODIFY] `internal/imagetomd/tern/runtime.go`(file://internal/imagetomd/tern/runtime.go)
*   **Description**: inproc起動安定性を改善し、`agent service port was not assigned` を低減する。
*   **Technical Design**:
    *   `waitForAgentPort` と `waitForHealth` の待機時間・ポーリング条件を見直す。
    *   `Launch` 失敗を早期に取得できるよう `launchErrCh` 監視を強化。
    *   ```go
        func waitForAgentPort(ctx context.Context, srv *arcticserver.Server, timeout time.Duration) (int, error)
        func waitForHealth(ctx context.Context, endpoint string, timeout time.Duration) error
        ```
*   **Logic**:
    *   ポート未割当だけで即失敗しない。
    *   明示的に「起動失敗」「ヘルス未到達」を識別して返す。

#### [MODIFY] `internal/imagetomd/tern/runtime_test.go`(file://internal/imagetomd/tern/runtime_test.go)
*   **Description**: 起動安定性改善に対応した失敗形・待機挙動の契約テストを更新する。
*   **Technical Design**:
    *   `ErrBootFailed` を返す条件の精密化。
    *   タイムアウト境界のテストケースを追加。
*   **Logic**:
    *   スキップ禁止（`t.Skip*` 不使用）。

### `internal/imagetomd/analyzer`

#### [MODIFY] `internal/imagetomd/analyzer/prompts.go`(file://internal/imagetomd/analyzer/prompts.go)
*   **Description**: 参照実装の `PhaseDefinition` / `Phases` / 各Prompt文面を正本として一致させる。
*   **Technical Design**:
    *   構造体定義:
      ```go
      type PhaseDefinition struct {
          Num       int
          Name      string
          Goal      string
          MaxRounds int
      }
      ```
    *   `Phases` の4フェーズ目標文を参照実装の内容へ統一。
*   **Logic**:
    *   参照実装で効いている質問生成規約（開始文字指定・Vision限定・会話文禁止）を保持。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: 参照実装と同等の処理責務へ収束し、過剰なヒューリスティックを整理する。
*   **Technical Design**:
    *   `Analyze` は次の順序を厳守:
      1) classify, 2) phase 1-4 adaptive loop, 3) final synthesis
    *   `adaptiveLoop` は `phase.MaxRounds` を基準にし、Phase3/4拡張は最大1回・追加2roundまで。
    *   plan-only 判定は「ログ注記」のみ（本文除外に使わない）。**制御フロー分岐には利用しない**。
    *   「plan-onlyを最初から避ける」ため、実行プロンプトを強化:
      * 回答先頭強制（例: `|` もしくは `-`）
      * 計画文禁止 + データ即出力
      * 計画文を出した場合は同一応答内で直ちにデータ本体を続けるよう指示
    *   Round実行時の再試行は「通信失敗/空応答」のみ。`plan-only` だけを理由に追加分岐しない。
    *   ```go
      type AnalyzeOptions struct {
          StrictGapJudge  bool
          SaveQuestionLog bool
          RoundSleepMS    int
          PhaseSleepMS    int
          MaxRounds       int // >0 なら phase.MaxRounds を上書き
          Progress        func(format string, args ...any)
      }
      ```
*   **Logic**:
    *   最終Markdownが空/ログ形式なら失敗として扱う（誤成功を禁止）。
    *   `knownInfo` への追記は回答本文を常に使用。
    *   **原則**: 参照実装に存在しない分岐は導入しない。必要な分岐は「再現性を守る安全網」のみに限定する。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: 参照準拠の責務・拡張上限・ログ混入禁止のテストを追加する。
*   **Technical Design**:
    *   Phaseループ順序、max-round override、拡張回数上限の検証。
    *   final output が `Phase/Q/A` 形式でないことの判定テスト。
*   **Logic**:
    *   参照実装との差分が再発したら即失敗する。

#### [MODIFY] `internal/imagetomd/analyzer/fallback_test.go`(file://internal/imagetomd/analyzer/fallback_test.go)
*   **Description**: 既存フォールバック関連テストを、参照準拠方針（誤成功防止）に合わせて更新する。
*   **Technical Design**:
    *   「本文を破棄しない」「解析ログを最終成果物としない」契約を明文化。
*   **Logic**:
    *   失敗を隠さない設計へ統一。

### CLI / Public API

#### [MODIFY] `cmd/image-to-markdown/main.go`(file://cmd/image-to-markdown/main.go)
*   **Description**: `--verbose` 時の進捗ログ項目を参照実装互換（Phase/Round中心）へ整理する。
*   **Technical Design**:
    *   `stderr` のみ出力、`stdout` 契約不変。
    *   ログ項目（最低限）:
      * `image`, `session_id`, `mode`, `endpoint`
      * `phase`, `round`, `step`（assess/generate/execute/synthesize）
      * `sufficient`, `answer_chars`, `retry_reason`
      * `elapsed_ms`（phase単位/全体）
*   **Logic**:
    *   `--quiet` 時は詳細ログ無効。

#### [NEW] `tests/image_to_markdown_logging_test.go`(file://tests/image_to_markdown_logging_test.go)
*   **Description**: 動作確認ログの契約（出力先・項目・ノイズ混入防止）を固定する。
*   **Technical Design**:
    *   `--verbose` 実行時に `stderr` に必要キーが出ることを検証。
    *   `stdout` にログが混入しないことを検証。
    *   `--quiet` で詳細ログが抑止されることを検証。
*   **Logic**:
    *   ログ設計の退行を防止し、調査容易性を維持する。

#### [MODIFY] `cmd/image-to-markdown/main_test.go`(file://cmd/image-to-markdown/main_test.go)
*   **Description**: CLI契約（stdin/output-mode/exit code）非退行を確認する。
*   **Technical Design**:
    *   verbose追加後も `stdout` にログが混入しないことを検証。

#### [MODIFY] `entext.go`(file://entext.go)
*   **Description**: `ConvertImageToMarkdown` で analyzer/ternの参照準拠挙動をAPI経由でも有効化する。
*   **Technical Design**:
    *   `ImageToMarkdownConfig` の運用整合（verbose/quiet/tern-mode/rounds）を維持。
*   **Logic**:
    *   CLI専用分岐を置かずAPIと同一ロジック。

#### [MODIFY] `tests/root_api_validation_test.go`(file://tests/root_api_validation_test.go)
*   **Description**: 公開API経由での設定受け渡しと変換成功条件を確認する。
*   **Technical Design**:
    *   `ImageToMarkdownConfig` で inproc/verbose/max-rounds を設定したコンパイル・実行契約テスト。

### Integration / E2E-style Regression (tests module)

#### [NEW] `tests/image_to_markdown_reference_parity_test.go`(file://tests/image_to_markdown_reference_parity_test.go)
*   **Description**: 固定画像2枚に対する参照準拠回帰テストを追加する。
*   **Technical Design**:
    *   入力:
      - `tmp/entext-test/images/pc/01_変更履歴.png`
      - `tmp/entext-test/images/pc/02_書き換えルール.png`
    *   比較:
      - 出力が空でない
      - 必須トークン存在（No.43/44、REQ IDs、見出し列）
      - 禁止トークン不在（`Q:`, `A:`, `### Phase`）
      - 参照出力との正規化比較（空白正規化 + 重要トークン一致率）
*   **Logic**:
    *   手動コマンド確認の代替として自動判定を実施。

#### [NEW] `tests/testdata/reference_parity/01_変更履歴.md`(file://tests/testdata/reference_parity/01_変更履歴.md)
*   **Description**: 参照実装の正解データを固定化する。

#### [NEW] `tests/testdata/reference_parity/02_書き換えルール.md`(file://tests/testdata/reference_parity/02_書き換えルール.md)
*   **Description**: 参照実装の正解データを固定化する。

## Step-by-Step Implementation Guide

1.  **Reference Baseline Freeze (TDD-RED 準備)**:
    *   Add `tests/testdata/reference_parity/*.md` from `tmp/reference/image-to-markdown/output/`.
    *   Define normalized comparison policy in a new test helper.
2.  **Write Failing Parity Tests First**:
    *   Edit `tests/image_to_markdown_reference_parity_test.go` to assert required tokens / forbidden tokens / non-empty output.
    *   Ensure current implementation fails these assertions first.
3.  **Fix Stream Aggregation Path**:
    *   Edit `internal/imagetomd/tern/client.go` to prefer `OnText` payload.
    *   Add/Update `internal/imagetomd/tern/client_response_test.go`.
4.  **Align Analyzer Flow to Reference**:
    *   Edit `internal/imagetomd/analyzer/prompts.go` with `PhaseDefinition` parity details.
    *   Edit `internal/imagetomd/analyzer/analyzer.go` to keep reference order and reduce destructive heuristics.
    *   Add/Update `internal/imagetomd/analyzer/analyzer_test.go` and `fallback_test.go`.
5.  **Stabilize In-Process Runtime**:
    *   Edit `internal/imagetomd/tern/runtime.go` and `runtime_test.go` to improve wait and launch-error handling.
6.  **Wire Through CLI/API Contracts**:
    *   Edit `cmd/image-to-markdown/main.go`, `main_test.go`, `entext.go`, `tests/root_api_validation_test.go`.
7.  **Add Observability Contract (Logging)**:
    *   Edit `internal/imagetomd/analyzer/analyzer.go` and `internal/imagetomd/tern/runtime.go` to emit phase/round/runtime logs.
    *   Add `tests/image_to_markdown_logging_test.go` to pin required log fields and stdout/stderr separation.
8.  **Run Automated Verification and Review Gap to Reference**:
    *   Execute build + integration scripts.
    *   Run parity tests and inspect mismatch report.
9.  **Comprehensive Verdict (Required)**:
    *   Fill the post-test verdict checklist (skip/error/fallback/config/path) and decide ✅/⚠️/❌.

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    run the build script.
    ```bash
    ./scripts/process/build.sh
    ```

2.  **Integration Tests**:
    Run integration tests.
    ```bash
    ./scripts/process/integration_test.sh --specify "ImageToMarkdown|ReferenceParity|Tern|Config|RootAPIValidation"
    ```
    *   **Log Verification**: `stdout` にパスのみ、`stderr` に詳細ログ。`agent service port was not assigned` の発生有無と頻度を記録する。
    *   **動作確認ログ必須項目**:
        * `phase start/end`, `round start/end`, `sufficient`, `answer_chars`
        * `runtime mode`, `boot/connect failure reason`, `session id`
        * `final synthesis start/end`, `output_chars`

3.  **E2E Tests (新規/追加)**:
    新機能の動作を検証するE2Eテストコードを `tests/` 配下に追加する。  
    手動コマンド実行による確認は、E2Eテストコード化の**代替にはならない**。  
    既存の E2E テストインフラ (`tests/external_import_e2e_test.go` 系のヘルパー) を活用する。

    #### [NEW] `image_to_markdown_reference_parity_test.go`(file://tests/image_to_markdown_reference_parity_test.go)
    *   **テストケース**: `TestReferenceParity_ChangeHistory`, `TestReferenceParity_RewriteRules`, `TestReferenceParity_NoPhaseReportTokens`
    *   **検証ポイント**:
        * 出力非空
        * 必須列/ID文字列の存在
        * `Q:` `A:` `### Phase` の不在
        * 正規化した重要トークンセットが参照実装の閾値を満たす

    #### [NEW] `image_to_markdown_logging_test.go`(file://tests/image_to_markdown_logging_test.go)
    *   **テストケース**: `TestVerboseLogsContainPhaseRoundMetrics`, `TestQuietSuppressesDebugLogs`, `TestStdoutContainsOnlyResultPath`
    *   **検証ポイント**:
        * 調査に必要なログキーが `stderr` に出る
        * `stdout` は結果パス/JSONのみ
        * ログの有無が `verbose/quiet` で期待通り

4.  **Post-Test Comprehensive Verdict (from testing-rules §12)**:
    ```bash
    ./scripts/process/integration_test.sh --specify "ReferenceParity|NoPhaseReport|InProcess"
    ```
    *   実行後に以下を必ず記録:
        * スキップの有無（ゼロ）
        * 部分エラー/WARNの有無
        * フォールバックによる偽成功の有無
        * 設定誤適用（unexpected adapter/path）の有無
        * 判定: ✅/⚠️/❌ と理由

## Documentation

`prompts/specifications` および関連ドキュメントを確認し、本計画の影響を反映する。

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**: `image-to-markdown` の参照準拠方針、`--verbose` ログの意味、期待される最終出力（Phaseログ非混入）を追記。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/007-ImageToMarkdown-ReferenceParity.md`(file://prompts/phases/000-foundation/branches/main/ideas/007-ImageToMarkdown-ReferenceParity.md)
*   **更新内容**: 実装中に確定した差分（採用/不採用判断）を「実装結果メモ」として追記。

## 実行結果メモ（2026-07-11）

- `internal/imagetomd/analyzer/analyzer.go`  
  `plan-only` 判定を本文除外に使わないよう修正し、`--verbose` 時の追跡性向上のため `step=...` 形式で Phase/Round/再試行理由/文字数/経過時間ログを追加。
- `internal/imagetomd/analyzer/analyzer_test.go` を新規追加し、最終合成が Phase レポートだった場合の再試行・失敗契約・進捗ログ契約を固定。
- `internal/imagetomd/analyzer/fallback_test.go` を更新し、非空回答をコーパスに残す方針へ整合。
- `tests/image_to_markdown_logging_test.go` を新規追加し、`stdout` 非混入・`verbose`/`quiet` の出力契約を固定。
- `tests/image_to_markdown_reference_parity_test.go` と `tests/testdata/reference_parity/*.md` を追加し、禁止トークン (`Q:`, `A:`, `### Phase`) 不在と必須トークン存在を固定。
- `entext.go` に `runtime_ready` ログを追加し、`mode`/`endpoint` を可視化。

### 総合判定結果

**判定**: ⚠️ 条件付き確認完了

#### テスト結果サマリ
- 全テスト数: 実行ログ上の対象スコープで成功（Build + 指定 Integration）
- 成功: すべて成功
- 失敗: 0
- 事実上スキップ: 0

#### チェック項目の結果
| # | チェック項目 | 結果 | 備考 |
|---|------------|------|------|
| 1 | スキップされたテスト | ✅ | `t.Skip*` なし |
| 2 | 部分的なエラー | ✅ | 失敗ログなし |
| 3 | フォールバックによる偽成功 | ⚠️ | 実画像の live 参照比較（外部 LLM 実行）までは未実施 |
| 4 | アダプタ・コンフィグ誤適用 | ✅ | `runtime mode`/`endpoint` 可視化済み |
| 5 | テスト間依存 | ✅ | 新規テストは独立実行可能 |
| 6 | カバレッジ妥当性 | ⚠️ | 参照実装との完全一致比較は固定ゴールデン契約まで |
| 7 | 外部システム状態 | ✅ | 本回の検証は外部依存最小の範囲で成功 |

#### 判定理由
自動テストで契約退行防止（ログ混入防止・Phaseレポート最終出力防止・再試行契約）は担保できた。一方、参照実装との「実画像入力に対する完全同一出力」の最終確認には外部LLM実行を含む live 比較が残るため、判定は条件付き完了とした。
