# 008-ImageToMarkdown-TableFaithfulOutput

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/008-ImageToMarkdown-TableFaithfulOutput.md`

## Goal Description

`image-to-markdown` の最終成果物を「解析レポート」ではなく「原表中心の Markdown 写し」に寄せる。`IsSufficient` の誤判定修正、Phase 2 実行保証、最終統合プロンプトの出力制約強化、説明寄りセクション検出による再合成を実装し、`01_変更履歴.png` で 9 列原表・SUB 展開・禁止セクション非含有を自動テストで固定する。

## User Review Required

1. `GapJudgeCompat` の既定挙動を変更し、`NOT SUFFICIENT` / `SUFFICIENT ではありません` を**非充足**とみなすことで確定してよいか（現行 `TestIsSufficientCompat` は legacy 誤動作を期待しているため、テストも更新する）。
2. 説明寄り出力の検出に `looksLikeExplanatoryReport` を追加し、検出時は `looksLikePhaseReport` と同様に**再合成 1 回**を行う方針で確定してよいか。
3. `tests/testdata/reference_parity/01_変更履歴.md` を、No.43/44 の実セル値（`秋葉達也`, `2025/7/7`, `28X-REQ-A0220` 等）を含む原表中心ゴールデンへ更新してよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. 最終成果物の中心は原表 Markdown | `prompts.go` 最終統合制約、`analyzer.go` 品質チェック、`tests/testdata/reference_parity/01_変更履歴.md` |
| 2. `01_変更履歴` 原表セクション必須内容 | `tests/image_to_markdown_table_faithful_test.go`, `tests/testdata/reference_parity/01_変更履歴.md` |
| 3. 解析レポート専用セクション禁止 | `quality.go` + `prompts.go` + `analyzer.go` 再合成、`tests/image_to_markdown_table_faithful_test.go` |
| 4. SUB_TABLE 展開維持（原文再掲のみ） | `prompts.go` 最終統合・再試行プロンプト |
| 5. `IsSufficient` 否定文を誤充足しない | `gap_judge.go`, `gap_judge_test.go` |
| 6. Phase 2 が空 answer のまま終了しない | `gap_judge.go` 修正 + `analyzer.go` Phase2 ガード |
| 7. 中間表の最終転載禁止 | `prompts.go` 禁止事項、`quality.go` 検出 |
| 8. 最終統合プロンプトへ制約明文化 | `prompts.go`, `prompts_test.go` |
| 9. 007 契約維持（CLI/API/inproc/OnText） | 変更範囲を analyzer のみに限定。`tern/client.go` は触らない |
| 10. CLI/API 同一品質 | ロジックは `analyzer` 共有。`entext.go` / `cmd/image-to-markdown` は回帰テストのみ |
| 任意1. 既定 gap judge を否定安全化 | `gap_judge.go` compat 分岐（`--strict-gap-judge` は strict 維持） |
| 任意2. Phase 2 行データ抽出前処理 | 本計画では**先送り**（最終統合プロンプト強化で代替。効果不足時に follow-up） |
| 任意3. 説明寄り度ヒューリスティック | `quality.go` の `looksLikeExplanatoryReport` で対応 |

## Proposed Changes

### `internal/imagetomd/analyzer`

#### [NEW] `internal/imagetomd/analyzer/quality_test.go`(file://internal/imagetomd/analyzer/quality_test.go)
*   **Description**: 説明寄り出力検出と gap 判定の退行防止テストを先に追加（TDD-RED）。
*   **Technical Design**:
    *   テーブル駆動で `IsSufficient` と `looksLikeExplanatoryReport` を検証。
    *   ```go
        func TestIsSufficientCompatRejectsJapaneseNegation(t *testing.T) {
            cases := []struct {
                name string
                in   string
                want bool
            }{
                {name: "plain sufficient", in: "SUFFICIENT", want: true},
                {name: "decision sufficient", in: "判定: SUFFICIENT\n補足あり", want: true},
                {name: "japanese negation", in: "SUFFICIENT ではありません", want: false},
                {name: "english negation", in: "NOT SUFFICIENT", want: false},
                {name: "insufficient keyword", in: "INSUFFICIENT", want: false},
            }
            for _, tc := range cases {
                t.Run(tc.name, func(t *testing.T) {
                    if got := IsSufficient(tc.in, GapJudgeCompat); got != tc.want {
                        t.Fatalf("got %v want %v", got, tc.want)
                    }
                })
            }
        }

        func TestLooksLikeExplanatoryReport(t *testing.T) {
            samples := []struct {
                name string
                in   string
                want bool
            }{
                {name: "phase meta heading", in: "## 要素一覧（Phase 1）\n| 要素ID |", want: true},
                {name: "format interpretation table", in: "## 書式・注記・セル結合・突合", want: true},
                {name: "meaning column", in: "| 意味対応・解釈 |", want: true},
                {name: "faithful table", in: "# 変更履歴\n\n| No. | 変更箇所 |", want: false},
                {name: "nested expansion ok", in: "## 入れ子構造の展開\n### SUB_TABLE_P01", want: false},
            }
            for _, tc := range samples {
                t.Run(tc.name, func(t *testing.T) {
                    if got := looksLikeExplanatoryReport(tc.in); got != tc.want {
                        t.Fatalf("got %v want %v", got, tc.want)
                    }
                })
            }
        }
        ```
*   **Logic**:
    *   実装前に RED を確認してから本体実装へ進む。

#### [MODIFY] `internal/imagetomd/analyzer/gap_judge_test.go`(file://internal/imagetomd/analyzer/gap_judge_test.go)
*   **Description**: legacy 誤動作を期待していた `TestIsSufficientCompat` を否定安全仕様へ更新。
*   **Technical Design**:
    *   `TestIsSufficientCompat` を削除または置換し、`TestIsSufficientCompatRejectsNegation`（`quality_test.go`）へ統合。
*   **Logic**:
    *   `NOT SUFFICIENT` が true になるテストは**削除**する。

#### [MODIFY] `internal/imagetomd/analyzer/gap_judge.go`(file://internal/imagetomd/analyzer/gap_judge.go)
*   **Description**: compat モードでも否定パターンを先に除外する否定安全 `IsSufficient` へ変更。
*   **Technical Design**:
    *   ```go
        var compatNegativePatterns = []*regexp.Regexp{
            regexp.MustCompile(`(?i)NOT\s+SUFFICIENT`),
            regexp.MustCompile(`SUFFICIENT\s*ではありません`),
            regexp.MustCompile(`(?i)INSUFFICIENT`),
            regexp.MustCompile(`(?i)NOT_SUFFICIENT`),
        }

        func isCompatNegativeSufficient(resp string) bool {
            for _, p := range compatNegativePatterns {
                if p.MatchString(resp) {
                    return true
                }
            }
            return false
        }

        func IsSufficient(resp string, mode GapJudgeMode) bool {
            switch mode {
            case GapJudgeStrict:
                return strictSufficientLine.MatchString(resp) || strictDecisionLine.MatchString(resp)
            default:
                if isCompatNegativeSufficient(resp) {
                    return false
                }
                return strings.Contains(strings.ToUpper(resp), "SUFFICIENT")
            }
        }
        ```
*   **Logic**:
    *   `SUFFICIENT ではありません` → false。
    *   `判定: SUFFICIENT`（否定語なし）→ true。
    *   strict モードは現行の行マッチを維持。

#### [NEW] `internal/imagetomd/analyzer/quality.go`(file://internal/imagetomd/analyzer/quality.go)
*   **Description**: 説明寄り最終出力を機械検出する品質ヘルパーを追加。
*   **Technical Design**:
    *   ```go
        var explanatoryHeadingPatterns = []*regexp.Regexp{
            regexp.MustCompile(`(?i)要素一覧（Phase`),
            regexp.MustCompile(`書式・注記・セル結合`),
            regexp.MustCompile(`意味対応・解釈`),
            regexp.MustCompile(`図解概要`),
            regexp.MustCompile(`(?m)^##\s+図解要素`),
        }

        func looksLikeExplanatoryReport(text string) bool {
            trimmed := strings.TrimSpace(text)
            if trimmed == "" {
                return false
            }
            for _, p := range explanatoryHeadingPatterns {
                if p.MatchString(trimmed) {
                    return true
                }
            }
            // メタ解析表: 要素ID 列 + 解釈系列名の同時出現
            lower := strings.ToLower(trimmed)
            if strings.Contains(lower, "要素id") && strings.Contains(lower, "意味対応") {
                return true
            }
            return false
        }

        func needsFinalSynthesisRetry(markdown string) (bool, string) {
            if strings.TrimSpace(markdown) == "" {
                return true, "empty"
            }
            if looksLikePhaseReport(markdown) {
                return true, "phase_report"
            }
            if looksLikeExplanatoryReport(markdown) {
                return true, "explanatory_report"
            }
            return false, ""
        }
        ```
*   **Logic**:
    *   `looksLikePhaseReport` は既存維持。説明寄り検出は別関数で分離。

#### [MODIFY] `internal/imagetomd/analyzer/prompts_test.go`(file://internal/imagetomd/analyzer/prompts_test.go)（新規）
*   **Description**: 最終統合プロンプトに原表中心・禁止セクション文言が含まれることを固定。
*   **Technical Design**:
    *   ```go
        func TestGenerateMarkdownPromptContainsTableFaithfulConstraints(t *testing.T) {
            prompt := GenerateMarkdownPrompt(nil)
            must := []string{
                "原表",
                "解析メタ表",
                "要素一覧（Phase",
                "書式・注記・セル結合",
                "意味対応・解釈",
                "入れ子構造の展開",
            }
            for _, token := range must {
                if !strings.Contains(prompt, token) {
                    t.Fatalf("missing constraint token %q", token)
                }
            }
        }
        ```
*   **Logic**:
    *   プロンプト文字列の退行を単体テストで検出。

#### [MODIFY] `internal/imagetomd/analyzer/prompts.go`(file://internal/imagetomd/analyzer/prompts.go)
*   **Description**: 最終統合・再試行プロンプトへ原表中心制約と禁止セクションを追記。Phase 2 向け質問の開始行ヒントを追加。
*   **Technical Design**:
    *   `GenerateMarkdownPrompt` 末尾に以下制約ブロックを追加:
        ```text
        **最終成果物の必須構造（complex_table）**
        1. 文書先頭は画像タイトル（例: `# 変更履歴`）と、画像の原表を表す Markdown テーブル。
        2. 原表は画像の全列・全行を省略せず再現する。No.43/No.44 等の実データ行を含める。
        3. 入れ子セルは親表セルに `[詳細](#sub_table_p01)` 等のリンクを置き、`## 入れ子構造の展開` 以下で原文のみ再掲する。
        4. 禁止: `要素一覧（Phase` / `書式・注記・セル結合・突合` / `意味対応・解釈` / `図解概要` / 要素ID・隣接遷移・親子関係の解析専用表。
        5. Phase 1/3/4 の中間解析表は素材として使うが、そのまま転載しない。原表セルへマージする。
        6. diagram/mixed 以外では Mermaid と図解説明セクションを出力しない。
        ```
    *   `GenerateMarkdownRetryPrompt` にも同趣旨の禁止リストを同期。
    *   `GenerateQuestionPrompt` の rule 2（SUB_TABLE）は**削除しない**。
    *   Phase 2 専用の execute 質問テンプレート関数を追加（任意だが推奨）:
        ```go
        func Phase2ExecuteHint() string {
            return "必ず画像の原表列名（`| No. | 変更箇所 | 変更内容(変更理由) | Ver | 作成・変更者 | 作成・変更日 | 承認者 | 承認日 | 備考 |`）から始める Markdown テーブルのみを即時出力してください。"
        }
        ```
*   **Logic**:
    *   SUB_TABLE 方針は維持しつつ、最終出力の情報設計だけを原表中心に矯正。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: 説明寄り最終出力の再試行契約と Phase 2 ガードをモックで検証。
*   **Technical Design**:
    *   追加ケース:
        * `TestAnalyzeRetriesWhenFinalLooksLikeExplanatoryReport`
        * `TestAnalyzePhase2RequiresNonEmptyAnswerBeforeSoftLimit`（gap が `SUFFICIENT ではありません` のとき Phase 2 が継続）
    *   ```go
        // Phase2 guard: queueClient に assess="SUFFICIENT ではありません", execute="| No. |..." を投入し
        // phase2 round が 2 回以上進むことを検証
        ```
*   **Logic**:
    *   既存 `queueClient` を再利用。`Progress` ログに `phase=2 round=2` が出ることを確認。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer.go`(file://internal/imagetomd/analyzer/analyzer.go)
*   **Description**: 最終合成の品質判定を `needsFinalSynthesisRetry` に統合。Phase 2 ソフト終了ガードを追加。
*   **Technical Design**:
    *   最終合成部を以下へ置換:
        ```go
        markdown, err := a.client.SendText(ctx, sessionID, finalPrompt)
        if retry, reason := needsFinalSynthesisRetry(markdown); retry {
            a.progressf("step=final_synthesis_retry reason=%s", reason)
            retryPrompt := GenerateMarkdownRetryPrompt(buildAnswerCorpus(log.Phases))
            markdown, err = a.client.SendText(ctx, sessionID, retryPrompt)
        }
        if retry, _ := needsFinalSynthesisRetry(markdown); retry {
            return "", nil, ErrEmptyMarkdown
        }
        ```
    *   Phase ループ内、phase 終了前ガード:
        ```go
        if phase.Num == 2 && phaseLog.ExitReason == "soft_limit" && !phaseLog.HasNonEmptyAnswer() {
            // sufficient 誤判定の安全網: 最低1ラウンドは画像付き execute を要求
            a.progressf("step=phase2_guard reason=no_answer_before_soft_limit")
            // 既知情報が空でなければ継続、空なら hard_limit 相当で再実行ラウンドを1回追加
        }
        ```
    *   `PhaseLog` にヘルパーを追加:
        ```go
        func (p PhaseLog) HasNonEmptyAnswer() bool {
            for _, r := range p.Rounds {
                if strings.TrimSpace(r.Answer) != "" {
                    return true
                }
            }
            return false
        }
        ```
*   **Logic**:
    *   `looksLikePhaseReport` 単独判定から、説明寄り検出を含む統合判定へ拡張。
    *   Phase 2 ガードは `IsSufficient` 修正後も残す（二重の安全網）。

#### [MODIFY] `internal/imagetomd/analyzer/fallback_test.go`(file://internal/imagetomd/analyzer/fallback_test.go)
*   **Description**: `needsFinalSynthesisRetry` の reason 分岐テストを追加。
*   **Logic**:
    *   `explanatory_report` reason が説明寄り見出しで返ることを固定。

### `tests`（統合・ゴールデン）

#### [MODIFY] `tests/testdata/reference_parity/01_変更履歴.md`(file://tests/testdata/reference_parity/01_変更履歴.md)
*   **Description**: 原表中心ゴールデンへ更新（実セル値・空行・SUB 展開）。
*   **Technical Design**:
    *   必須内容:
        * 9 列ヘッダ
        * `| 43 |` 行に `秋葉達也`, `2025/7/7`, `28X-REQ-A0220`, `^/search/event.html(.*)`
        * `| 44 |` 行に `藤本華子`, `2025/12/9`, `28P-ITb1-0243`
        * 下部空行 8 行程度
        * `## 入れ子構造の展開` + `SUB_TABLE` 原文リスト
*   **Logic**:
    *   「不明」プレースホルダーは除去し、画像と一致する文字列へ更新。

#### [NEW] `tests/image_to_markdown_table_faithful_test.go`(file://tests/image_to_markdown_table_faithful_test.go)
*   **Description**: 原表忠実度・禁止セクション非含有の統合契約テスト。
*   **Technical Design**:
    *   ```go
        func TestTableFaithful_ChangeHistoryGoldenContract(t *testing.T) {
            assertReferenceMarkdownContract(t, "01_変更履歴.md",
                []string{
                    "# 変更履歴",
                    "| No. | 変更箇所 | 変更内容(変更理由) | Ver | 作成・変更者 | 作成・変更日 | 承認者 | 承認日 | 備考 |",
                    "秋葉達也",
                    "2025/7/7",
                    "28X-REQ-A0220",
                    "藤本華子",
                    "2025/12/9",
                    "28P-ITb1-0243",
                    "入れ子構造の展開",
                },
                []string{
                    "要素一覧（Phase",
                    "書式・注記・セル結合",
                    "意味対応・解釈",
                    "図解概要",
                    "Q:",
                    "A:",
                    "### Phase",
                },
            )
        }
        ```
*   **Logic**:
    *   既存 `assertReferenceMarkdownContract` ヘルパーを `image_to_markdown_reference_parity_test.go` から共有可能なら `tests/helper_reference_parity.go` へ抽出。

#### [MODIFY] `tests/image_to_markdown_reference_parity_test.go`(file://tests/image_to_markdown_reference_parity_test.go)
*   **Description**: 007 契約テストを維持しつつ、原表必須トークンを `TableFaithful` テストへ委譲。
*   **Logic**:
    *   重複アサーションは整理し、007=ログ非混入、008=原表忠実度と責務分離。

## Implementation Status

- [x] Step 1-2: gap judge tests + `isCompatNegativeSufficient`
- [x] Step 3: `quality.go` + unit tests
- [x] Step 4: `prompts_test.go` + table-faithful prompt constraints
- [x] Step 5: `analyzer.go` flow + analyzer/fallback tests
- [x] Step 6: golden + `image_to_markdown_table_faithful_test.go` + README
- [x] Step 7: Verification (`build.sh` + `integration_test.sh`) — ✅ PASS 2026-07-11

### CLI / Public API（回帰のみ）

#### [MODIFY] `cmd/image-to-markdown/main_test.go`(file://cmd/image-to-markdown/main_test.go)
*   **Description**: CLI 契約の回帰確認（フラグ追加なし）。
*   **Logic**:
    *   既存テストが通ることを Verification で確認。

#### [MODIFY] `entext.go` / `tests/root_api_validation_test.go`
*   **Description**: 公開 API 経路の回帰確認のみ。ロジック変更なし。

## Step-by-Step Implementation Guide

1.  **TDD-RED: 品質・gap 判定テスト追加**:
    *   Add `internal/imagetomd/analyzer/quality_test.go` with failing `IsSufficient` / `looksLikeExplanatoryReport` cases.
    *   Update `gap_judge_test.go` to expect `NOT SUFFICIENT` => false.
2.  **GREEN: gap 判定修正**:
    *   Edit `internal/imagetomd/analyzer/gap_judge.go` to implement `isCompatNegativeSufficient`.
3.  **TDD-RED: 品質ヘルパー**:
    *   Add `internal/imagetomd/analyzer/quality.go` with `looksLikeExplanatoryReport` and `needsFinalSynthesisRetry`.
    *   Run `./scripts/process/build.sh` and confirm new unit tests pass.
4.  **TDD-RED: プロンプト契約テスト**:
    *   Add `internal/imagetomd/analyzer/prompts_test.go`.
    *   Edit `internal/imagetomd/analyzer/prompts.go` to append table-faithful constraint blocks.
5.  **Analyzer フロー修正**:
    *   Edit `internal/imagetomd/analyzer/analyzer.go`:
        * Replace final synthesis retry condition with `needsFinalSynthesisRetry`.
        * Add `PhaseLog.HasNonEmptyAnswer` and Phase 2 guard.
    *   Extend `internal/imagetomd/analyzer/analyzer_test.go` and `fallback_test.go`.
6.  **ゴールデン・統合テスト更新**:
    *   Update `tests/testdata/reference_parity/01_変更履歴.md` to table-faithful golden.
    *   Add `tests/image_to_markdown_table_faithful_test.go`.
    *   Adjust `tests/image_to_markdown_reference_parity_test.go` if needed.
7.  **Verification Plan 実行**:
    *   Run full automated verification below.
    *   Record comprehensive verdict per testing-rules §12.

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```

2.  **Integration Tests**:
    ```bash
    ./scripts/process/integration_test.sh --specify "TableFaithful|ReferenceParity|NoPhaseReport|ImageToMarkdown|RootAPI"
    ```
    *   **Log Verification**:
        * `stderr` に `step=phase2_guard` が誤充足時のみ出ること（正常時は不要）。
        * `step=final_synthesis_retry reason=explanatory_report` が説明寄り出力時に出ること。
        * `stdout` は結果パスのみ（CLI 契約維持）。

3.  **E2E Tests (新規/追加)**:

    #### [NEW] `image_to_markdown_table_faithful_test.go`(file://tests/image_to_markdown_table_faithful_test.go)
    *   **テストケース**: `TestTableFaithful_ChangeHistoryGoldenContract`, `TestTableFaithful_ForbiddenExplanatoryHeadings`
    *   **検証ポイント**:
        * 原表 9 列ヘッダ存在
        * No.43/44 の実データ文字列存在
        * `要素一覧（Phase` / `書式・注記` / `意味対応・解釈` 不在
        * `入れ子構造の展開` または `SUB_TABLE` 存在

    E2E として live LLM 変換は本計画の必須範囲外（外部依存・非決定性）。固定ゴールデン契約テストで代替する理由を明記。

4.  **Post-Test Comprehensive Verdict (testing-rules §12)**:
    *   スキップ 0 / 失敗 0 / フォールバック偽成功なしを確認。
    *   判定 ✅/⚠️/❌ を実装メモに記録。

### テスト項目設計セルフレビュー（testing-rules §11）

| 観点 | 対応 |
| :--- | :--- |
| 単体: gap 判定 | `gap_judge_test.go`, `quality_test.go` |
| 単体: 説明寄り検出 | `quality_test.go`, `fallback_test.go` |
| 単体: プロンプト制約 | `prompts_test.go` |
| 単体: analyzer フロー | `analyzer_test.go`（再試行・Phase2 ガード） |
| 統合: ゴールデン契約 | `image_to_markdown_table_faithful_test.go` |
| 統合: 007 回帰 | `image_to_markdown_reference_parity_test.go` |
| 迂回排除 | 禁止見出し・必須セル値を文字列固定で検証 |
| 依存関係 | analyzer 変更 → tests ゴールデン更新の順で実施 |

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**: `image-to-markdown` の期待出力が「原表中心 Markdown」であること、解析レポートセクションが成果物に含まれないことを追記。

#### [MODIFY] `prompts/phases/000-foundation/branches/main/ideas/008-ImageToMarkdown-TableFaithfulOutput.md`(file://prompts/phases/000-foundation/branches/main/ideas/008-ImageToMarkdown-TableFaithfulOutput.md)
*   **更新内容**: 実装完了後に「実装結果メモ」セクションを追記。
