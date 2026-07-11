# 009-ImageToMarkdown-ConversionScopedGapJudgment

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/010-ImageToMarkdown-ConversionScopedGapJudgment.md`

## Goal Description

`image-to-markdown` のギャップ判定を**変換正確性**（画像→Markdown 転記）の範囲に限定し、内容校正・意味整合性・URL 妥当性検証を Phase 2 の不足理由から除外する。あわせて gap 判定出力を `SUFFICIENT` / `INSUFFICIENT` の**二値**に固定し、`IsSufficient` パーサーを **INSUFFICIENT 優先 → SUFFICIENT → 未検出は false フォールバック** の順序へ変更する。008 で得た原表忠実度（9 列・No.43/44・空欄行・入れ子展開）は回帰させない。

## User Review Required

1. **判定語なし回答のフォールバック**: 仕様どおり `SUFFICIENT` / `INSUFFICIENT` のどちらも含まない `gap_assessment` は **不充足（`false`）** とする。LLM が旧来の `不足しています` のみを返す移行期は追加ラウンドが発生しうるが、プロンプト二値化で収束させる方針で確定してよいか。
2. **任意要件 1〜3 は先送り**: 最新回答優先、`known_info` 軽量チェック、同一表の早期終了は本計画では実装しない。プロンプト境界＋二値パーサーで Phase 2 `hard_limit` 問題を解消し、効果不足時に follow-up とする方針でよいか。
3. **Phase 3/4 Goal の追記範囲**: 仕様要件 9 に従い、Phase 3/4 の Goal 末尾へ「変換目的に従属・内容整合性評価禁止」の 1 文を追加する。Phase 専用の `AssessGapPrompt` 分岐は Phase 2 のみ（変換充足ガイド）とする方針でよいか。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 1. Phase 2 は変換情報の充足のみ判定 | `prompts.go` `AssessGapPrompt` 変換境界、`phase2ConversionGapGuide` |
| 2. Phase 2 充足条件（表/図解） | `prompts.go` Phase 2 Goal + Phase 2 専用ガイド |
| 3. 内容校正を不足理由にしない | `AssessGapPrompt` 禁止事項、`GenerateQuestionPrompt` 禁止ルール |
| 4. 画像との転記不一致は再読取可 | `AssessGapPrompt` 不足理由の表現ガイド |
| 5. 中間回答間の字形差だけで継続しない | `AssessGapPrompt` Phase 2 ガイド |
| 6. 判読不能箇所の有限処理 | `AssessGapPrompt` + `GenerateQuestionPrompt`（校正ループ禁止） |
| 7. `AssessGapPrompt` 変換境界 | `prompts.go` `conversionScopeBoundary` |
| 8. Phase 2 Goal の転記忠実度明文化 | `prompts.go` `DefaultPhases[1].Goal` |
| 9. Phase 3/4 も変換目的に従属 | `prompts.go` Phase 3/4 Goal 追記 |
| 10. `01_変更履歴` 改善結果維持 | 既存 `tests/image_to_markdown_table_faithful_test.go` 回帰 |
| 11. 校正レポート非出力 | 008 実装維持（変更なし、回帰テストのみ） |
| 12. CLI/API 同一ギャップ判定 | `analyzer` 共有ロジック、回帰 `ImageToMarkdown\|RootAPI` |
| 13. 二値出力 SUFFICIENT / INSUFFICIENT | `AssessGapPrompt` + `gap_judge.go` |
| 14. 不充足時 INSUFFICIENT 義務・代替表現禁止 | `AssessGapPrompt` |
| 15. パーサー INSUFFICIENT 優先・フォールバック | `gap_judge.go`, `gap_judge_test.go`, `quality_test.go` |
| 16. `sufficient` フィールド整合 | `analyzer.go`（既存フロー）+ `analyzer_test.go` |
| 任意1. 最新原表候補優先 | **先送り** — プロンプト改善で代替 |
| 任意2. 決定的チェック | **先送り** — follow-up |
| 任意3. 同一表早期終了 | **先送り** — follow-up |
| 任意4. 内容校正の独立コマンド | **非目標** — 本計画対象外 |

## Proposed Changes

### `internal/imagetomd/analyzer`

#### [MODIFY] `internal/imagetomd/analyzer/gap_judge_test.go`(file://internal/imagetomd/analyzer/gap_judge_test.go)
*   **Description**: 二値パーサー仕様の RED テストを先に追加・拡張（TDD）。
*   **Technical Design**:
    *   ```go
        func TestIsSufficientStrictBinary(t *testing.T) {
            cases := []struct {
                name string
                in   string
                want bool
            }{
                {name: "insufficient line", in: "INSUFFICIENT", want: false},
                {name: "decision insufficient", in: "Decision: INSUFFICIENT", want: false},
                {name: "sufficient line", in: "SUFFICIENT", want: true},
                {name: "decision sufficient", in: "Decision:SUFFICIENT", want: true},
                {name: "not sufficient legacy", in: "NOT SUFFICIENT", want: false},
                {name: "ambiguous no token", in: "不足しています", want: false},
            }
            for _, tc := range cases {
                t.Run(tc.name, func(t *testing.T) {
                    if got := IsSufficient(tc.in, GapJudgeStrict); got != tc.want {
                        t.Fatalf("got %v want %v", got, tc.want)
                    }
                })
            }
        }

        func TestIsSufficientCompatInsufficientBeforeSufficientSubstring(t *testing.T) {
            in := "INSUFFICIENT\n補足: 列見出し未取得"
            if IsSufficient(in, GapJudgeCompat) {
                t.Fatalf("INSUFFICIENT must win over embedded SUFFICIENT substring")
            }
        }
        ```
*   **Logic**:
    *   実装前に RED を確認。既存 `TestIsSufficientStrict` は上記へ統合または拡張。

#### [MODIFY] `internal/imagetomd/analyzer/quality_test.go`(file://internal/imagetomd/analyzer/quality_test.go)
*   **Description**: compat 二値パースとフォールバックのテーブル駆動テストを拡張。
*   **Technical Design**:
    *   `TestIsSufficientCompatRejectsJapaneseNegation` に以下ケースを追加:
        *   `{name: "insufficient with note", in: "判定: INSUFFICIENT\n不足: 行", want: false}`
        *   `{name: "fallback ambiguous", in: "不足しています。列見出しが未取得", want: false}`
        *   `{name: "plain sufficient unchanged", in: "SUFFICIENT", want: true}`
    *   既存 `insufficient keyword` / `NOT SUFFICIENT` / `SUFFICIENT ではありません` ケースは維持（レガシー移行期）。
*   **Logic**:
    *   仕様 15 の判定順序 1→2→3 を compat で固定する。

#### [MODIFY] `internal/imagetomd/analyzer/gap_judge.go`(file://internal/imagetomd/analyzer/gap_judge.go)
*   **Description**: 否定パターン列挙＋部分一致依存から、明示的二値パーサーへ変更。
*   **Technical Design**:
    *   ```go
        var (
            strictSufficientLine     = regexp.MustCompile(`(?m)^\s*SUFFICIENT\s*$`)
            strictInsufficientLine   = regexp.MustCompile(`(?m)^\s*INSUFFICIENT\s*$`)
            strictDecisionSufficient = regexp.MustCompile(`(?mi)^\s*Decision\s*:\s*SUFFICIENT\s*$`)
            strictDecisionInsufficient = regexp.MustCompile(`(?mi)^\s*Decision\s*:\s*INSUFFICIENT\s*$`)

            compatInsufficientToken = regexp.MustCompile(`(?i)INSUFFICIENT`)
            compatLegacyNegativePatterns = []*regexp.Regexp{
                regexp.MustCompile(`(?i)NOT\s+SUFFICIENT`),
                regexp.MustCompile(`SUFFICIENT\s*ではありません`),
                regexp.MustCompile(`(?i)NOT_SUFFICIENT`),
            }
        )

        func isCompatInsufficient(resp string) bool {
            if compatInsufficientToken.MatchString(resp) {
                return true
            }
            for _, p := range compatLegacyNegativePatterns {
                if p.MatchString(resp) {
                    return true
                }
            }
            return false
        }

        func isCompatSufficient(resp string) bool {
            return strings.Contains(strings.ToUpper(resp), "SUFFICIENT")
        }

        func isStrictInsufficient(resp string) bool {
            return strictInsufficientLine.MatchString(resp) ||
                strictDecisionInsufficient.MatchString(resp)
        }

        func isStrictSufficient(resp string) bool {
            return strictSufficientLine.MatchString(resp) ||
                strictDecisionSufficient.MatchString(resp)
        }

        func IsSufficient(resp string, mode GapJudgeMode) bool {
            switch mode {
            case GapJudgeStrict:
                if isStrictInsufficient(resp) {
                    return false
                }
                return isStrictSufficient(resp)
            default:
                if isCompatInsufficient(resp) {
                    return false
                }
                if isCompatSufficient(resp) {
                    return true
                }
                return false
            }
        }
        ```
*   **Logic**:
    *   **Step 1**: `INSUFFICIENT`（およびレガシー否定）を検出 → `false`。
    *   **Step 2**: `SUFFICIENT` を検出 → `true`（Step 1 通過後なので `INSUFFICIENT` 部分一致問題なし）。
    *   **Step 3**: 未検出 → `false`（追加ラウンド継続）。
    *   strict も INSUFFICIENT 行を先に評価し、混在時は不充足側を優先。

#### [MODIFY] `internal/imagetomd/analyzer/prompts_test.go`(file://internal/imagetomd/analyzer/prompts_test.go)
*   **Description**: プロンプト契約テストを RED 先行で追加。
*   **Technical Design**:
    *   ```go
        func TestAssessGapPromptContainsBinaryGapJudgment(t *testing.T) {
            p := DefaultPhases[1]
            got := AssessGapPrompt(p, "known")
            for _, want := range []string{
                "SUFFICIENT",
                "INSUFFICIENT",
                "混在禁止",
                "不足しています",
                "NOT SUFFICIENT",
            } {
                if !strings.Contains(got, want) {
                    t.Fatalf("missing %q in AssessGapPrompt", want)
                }
            }
        }

        func TestAssessGapPromptContainsConversionScopeBoundary(t *testing.T) {
            got := AssessGapPrompt(DefaultPhases[1], "")
            for _, want := range []string{
                "画像から Markdown への変換",
                "校正または評価してはなりません",
                "画像どおり転記",
            } {
                if !strings.Contains(got, want) {
                    t.Fatalf("missing conversion scope %q", want)
                }
            }
        }

        func TestAssessGapPromptPhase2ContainsConversionGuide(t *testing.T) {
            got := AssessGapPrompt(DefaultPhases[1], "| No. | ... |")
            if !strings.Contains(got, "表記ゆれ") || !strings.Contains(got, "未取得") {
                t.Fatalf("phase2 conversion guide missing: %s", got)
            }
        }

        func TestPhase2GoalRequiresTranscriptionFidelityNotProofreading(t *testing.T) {
            goal := DefaultPhases[1].Goal
            if !strings.Contains(goal, "校正") || !strings.Contains(goal, "意味的整合性") {
                t.Fatalf("phase2 goal missing conversion-scope wording: %s", goal)
            }
        }

        func TestGenerateQuestionPromptForbidsContentValidation(t *testing.T) {
            got := GenerateQuestionPrompt(DefaultPhases[1], "INSUFFICIENT\n未取得: 行3")
            for _, forbidden := range []string{
                "SymbolEvidence",
                "内容整合性",
                "URL",
                "正規表現",
                "妥当性",
            } {
                if strings.Contains(got, forbidden) {
                    t.Fatalf("question prompt must not encourage %q", forbidden)
                }
            }
        }
        ```
*   **Logic**:
    *   仕様 7, 13, 14, 8 のプロンプト文言を固定する。

#### [MODIFY] `internal/imagetomd/analyzer/prompts.go`(file://internal/imagetomd/analyzer/prompts.go)
*   **Description**: Phase Goal、AssessGapPrompt、GenerateQuestionPrompt を変換スコープ＋二値判定対応へ更新。
*   **Technical Design**:
    *   Phase 2 Goal を仕様どおり置換:
        ```text
        テーブルの場合は、画像に見える全行・全列・セル値・空欄・入れ子内容を、
        内容の校正や意味的整合性の評価をせず、画像どおりに Markdown へ再現できる情報として記録する。
        図解の場合は、含まれるテキスト・ラベル・接続関係をすべて抽出する。
        読み取りの正確性は画像との転記忠実度を意味し、要約や解釈は行わない。
        ```
    *   Phase 3/4 Goal 末尾に追記:
        ```text
        本 Phase の分析は画像から Markdown への変換再現に従属する。
        元データの意味的整合性の評価や修正提案は行わない。
        ```
    *   定数追加:
        ```go
        const gapJudgmentBinaryOutput = `
        判定結果は、必ず次のいずれか一方を回答に含めてください（混在禁止）:
        - 充足: SUFFICIENT
        - 不充足: INSUFFICIENT

        不足理由や補足説明は、この判定語の前後に簡潔に記述してよいです。
        「不足しています」「不十分」「NOT SUFFICIENT」等の代替表現は使わないでください。`

        const conversionScopeBoundary = `
        この判定は画像から Markdown への変換に必要な情報の充足だけを対象とします。
        元画像の記載内容について、誤字、表記ゆれ、意味的矛盾、URL・正規表現の妥当性を
        校正または評価してはなりません。画像に記載されているなら、そのまま転記できていることを
        充足条件としてください。
        不足理由は「画像上の未取得の行・列・セル・入れ子情報」など、画像と出力候補の対応関係として述べてください。`

        const phase2ConversionGapGuide = `
        Phase 2 追加ガイド:
        - 複数ラウンドの回答間の字形差（[ ]/【】、〇/○、＜＞/〈〉、-/− 等）だけを理由に INSUFFICIENT にしない。
        - 原表全体（列見出し・データ行・空欄行・入れ子内容）が取得済みなら SUFFICIENT とする。
        - SymbolEvidence、内容整合性、URL・正規表現の妥当性検証を不足理由にしない。`
        ```
    *   `AssessGapPrompt` 改修:
        ```go
        func AssessGapPrompt(phase Phase, knownInfo string) string {
            var b strings.Builder
            b.WriteString(fmt.Sprintf(`現在は画像解析の Phase %d: [%s] を実施しています。
        この Phase の目的は「%s」です。
        ...`, phase.Num, phase.Name, phase.Goal, knownInfo))
            b.WriteString(gapJudgmentBinaryOutput)
            b.WriteString(conversionScopeBoundary)
            if phase.Num == 2 {
                b.WriteString(phase2ConversionGapGuide)
            }
            b.WriteString("\n回答は簡潔に行い、前置き（「はい、承知しました」等）は不要です。")
            return b.String()
        }
        ```
        ※ 旧行「十分な情報が集まった場合は…SUFFICIENT…を含めてください」は**削除**。
    *   `GenerateQuestionPrompt` に禁止ルール追加:
        ```text
        7. 次を質問に含めてはならない: SymbolEvidence、内容整合性評価、URL/正規表現の妥当性検証、
           表記ゆれの統一、複数転記結果の校正統合、行・列番号付き「最終確定版」の作成要求。
        8. 不足時は画像上の未取得データ（行・列・セル・入れ子）のみを対象とする。
        ```
*   **Logic**:
    *   LLM 出力を二値化し、Phase 2 が内容校正ループに入らないようプロンプト境界を固定する。

#### [MODIFY] `internal/imagetomd/analyzer/analyzer_test.go`(file://internal/imagetomd/analyzer/analyzer_test.go)
*   **Description**: `sufficient` フィールドと INSUFFICIENT 判定の整合、Phase 2 ガードとの相互作用を検証。
*   **Technical Design**:
    *   ```go
        func TestAnalyzeMapsInsufficientAssessmentToSufficientFalse(t *testing.T) {
            // classify + phase1 assess/execute + phase2 assess=INSUFFICIENT → sufficient=false in session
            // logs contain assess_end ... sufficient=false
        }

        func TestAnalyzePhase2GuardStillRunsWhenAssessIsSufficientButAnswerEmpty(t *testing.T) {
            // 既存 TestAnalyzePhase2RequiresNonEmptyAnswerBeforeSoftLimit を維持
        }

        func TestAnalyzePhase2ContinuesWhenAssessIsInsufficientToken(t *testing.T) {
            // assess="INSUFFICIENT\n未取得" → phase2 round continues (no premature soft_limit)
        }
        ```
*   **Logic**:
    *   仕様 16: `gap_assessment` に `INSUFFICIENT` があれば `sufficient: false`。
    *   008 の Phase 2 ガード（SUFFICIENT でも answer 空なら継続）は維持。

### `tests`

#### [NEW] `tests/image_to_markdown_conversion_scope_test.go`(file://tests/image_to_markdown_conversion_scope_test.go)
*   **Description**: 008 原表忠実度契約に加え、校正レポート禁止トークンがゴールデンに含まれないことを固定（統合契約）。
*   **Technical Design**:
    *   ```go
        func TestConversionScope_ChangeHistoryGoldenForbidsProofreadingArtifacts(t *testing.T) {
            t.Parallel()
            assertReferenceMarkdownContract(t, "01_変更履歴.md", nil, []string{
                "SymbolEvidence",
                "文字差異注記",
                "内容整合性",
                "意味対応・解釈",
            })
        }
        ```
*   **Logic**:
    *   仕様 10, 11 の回帰。LLM 実変換は行わずゴールデン契約で固定（008 と同パターン）。
    *   Phase 2 スコープの**実行時**検証は analyzer モック単体テストで担保。本ファイルは最終 Markdown 成果物の回帰。

## Step-by-Step Implementation Guide

1.  **RED: gap パーサー二値化テスト**:
    *   Edit `internal/imagetomd/analyzer/gap_judge_test.go` and `quality_test.go` to add INSUFFICIENT-first, fallback-false, strict INSUFFICIENT line cases.
    *   Run `./scripts/process/build.sh --skip-frontend --skip-etc` and confirm new tests fail.

2.  **GREEN: gap_judge.go 二値パーサー**:
    *   Edit `internal/imagetomd/analyzer/gap_judge.go` per Technical Design (`isCompatInsufficient` → `isCompatSufficient` → `return false`).
    *   Remove obsolete `isCompatNegativeSufficient` name; keep legacy patterns inside `isCompatInsufficient`.
    *   Re-run build until gap judge tests pass.

3.  **RED: プロンプト契約テスト**:
    *   Edit `internal/imagetomd/analyzer/prompts_test.go` with AssessGapPrompt / Phase2 Goal / GenerateQuestionPrompt tests.
    *   Confirm RED.

4.  **GREEN: prompts.go 更新**:
    *   Update `DefaultPhases[1].Goal`, Phase 3/4 Goal suffix.
    *   Add `gapJudgmentBinaryOutput`, `conversionScopeBoundary`, `phase2ConversionGapGuide`.
    *   Rewrite `AssessGapPrompt`; extend `GenerateQuestionPrompt` rules 7-8.
    *   Confirm prompts tests pass.

5.  **RED → GREEN: analyzer 統合モック**:
    *   Add `TestAnalyzeMapsInsufficientAssessmentToSufficientFalse` and `TestAnalyzePhase2ContinuesWhenAssessIsInsufficientToken` in `analyzer_test.go`.
    *   Adjust `appendDefaultPhaseResponses` helpers if needed to inject `INSUFFICIENT` / `SUFFICIENT` assess strings explicitly.
    *   Keep existing Phase 2 guard tests green.

6.  **統合契約テスト追加**:
    *   Create `tests/image_to_markdown_conversion_scope_test.go` with forbidden proofreading token checks on `01_変更履歴.md` golden.

7.  **Documentation**:
    *   Edit `README.md` image-to-markdown 節に 1 文追加: gap 判定は変換スコープ（転記忠実度）に限定し、判定語は `SUFFICIENT` / `INSUFFICIENT` の二値。

8.  **Verification Plan を実行**（下記）。

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    ```bash
    ./scripts/process/build.sh
    ```
    *   **Log Verification**: `internal/imagetomd/analyzer` の `GapJudge|IsSufficient|AssessGap|ConversionScope|Phase2` 関連テストがすべて PASS。

2.  **Integration Tests (selective)**:
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "TableFaithful|ReferenceParity|NoPhaseReport|ConversionScope|ImageToMarkdown|RootAPI"
    ```
    *   **Log Verification**:
        *   `TestTableFaithful_*` PASS（008 原表忠実度回帰）
        *   `TestConversionScope_ChangeHistoryGoldenForbidsProofreadingArtifacts` PASS
        *   `ImageToMarkdown|RootAPI` PASS（CLI/API 同一 analyzer 経路）

3.  **E2E Tests (新規/追加)**:
    *   本変更は LLM プロンプトと文字列パーサーの内部ロジックが中心であり、CLI 引数・HTTP 契約・ファイル I/O パスは変更しない。
    *   実 LLM を使った E2E（`01_変更履歴.png` 実変換）は CI 時間・外部依存のため本計画では**自動 E2E に含めない**。
    *   代替: analyzer モックで Phase 2 `INSUFFICIENT`/`SUFFICIENT` フローと、ゴールデン Markdown 契約で成果物品質を固定。
    *   **理由**: 仕様の検証シナリオ 1（実バイナリ変換）は手動確認対象とし、自動化はモック＋ゴールデンで十分と判断。

### Test Item Design Self-Review (§11.3 / §11.4)

| # | 観点 | 本計画での対応 |
|---|------|----------------|
| 1 | 正常系 | `SUFFICIENT` → true、`INSUFFICIENT` → false、プロンプト二値文言含有 |
| 2 | 異常系・境界 | 判定語なし → false、`INSUFFICIENT` 部分一致優先、レガシー否定 |
| 3 | 外部連携 | CLI/API は既存統合テストで analyzer 共有経路を回帰 |
| 4 | データ一貫性 | `RoundLog.Sufficient` が `IsSufficient(gap_assessment)` と一致 |
| 5 | 状態遷移 | Phase 2: INSUFFICIENT 継続 → 原表取得後 SUFFICIENT で soft_limit |
| 6 | 設定反映 | `--strict-gap-judge` で strict 行マッチ、`compat` 既定 |
| 7 | 副作用 | 008 原表出力・禁止セクション検出を回帰で確認 |

**§11.4 総合**: パーサー（末端 C）→ プロンプト契約（B）→ analyzer モック（A）のボトムアップ順で設計。ゴールデン契約により「校正アーティファクト非含有」も固定。実 LLM 変換は手動だが、Phase 2 hard_limit の主因（内容校正ループ＋判定揺れ）はプロンプト＋パーサーで除去可能と判断。

### Comprehensive Verdict (§12)

全自動テスト PASS 後、以下を確認してから完了とする:

1.  `AssessGapPrompt` 出力に `SUFFICIENT` / `INSUFFICIENT` 二値義務と変換境界が含まれる（単体テストで確認済み）。
2.  `IsSufficient("INSUFFICIENT...")` が false かつ `IsSufficient("SUFFICIENT")` が true（単体テストで確認済み）。
3.  `01_変更履歴.md` ゴールデンが 008 契約を満たし、校正アーティファクトトークンを含まない。
4.  （任意・手動）ビルド済み `bin/entext/image-to-markdown.exe` で `01_変更履歴.png` を再変換し、Phase 2 が内容校正のみで `hard_limit` しないことをセッションログで確認。

## Documentation

#### [MODIFY] [README.md](file://README.md)
*   **更新内容**: `image-to-markdown` の gap 判定が変換スコープ（転記忠実度）に限定され、判定出力が `SUFFICIENT` / `INSUFFICIENT` 二値である旨を 1〜2 文追加。
