# 000-Portable-ExcelPdfImageMarkdown

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/000-Portable-ExcelPdfImageMarkdown.md`

## Goal Description
Excel -> PDF -> Image -> Markdown を 3 つの独立CLIとして実装し、`--stdin` 明示方式で安全にパイプ連携できるツールチェーンを提供する。`image-to-markdown` は既存 `features/image-to-markdown` のアルゴリズムを再現しつつ、互換を壊さない改善モードを必須機能として実装する。

## User Review Required
1. Excel -> PDF 変換エンジンの標準実装（LibreOffice CLI 呼び出し）で問題ないか。  
2. PDF -> Image 変換エンジンの標準実装（Poppler `pdftoppm` 呼び出し）で問題ないか。  
3. `image-to-markdown` 改善オプションのデフォルト値:
   - `--save-question-log`: 有効
   - `--strict-gap-judge`: 無効（互換優先）
   - `--round-sleep-ms`, `--phase-sleep-ms`: 5000

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| 3ツール分離 (`excel-to-pdf`, `pdf-to-image`, `image-to-markdown`) | Proposed Changes > `cmd/excel-to-pdf`, `cmd/pdf-to-image`, `cmd/image-to-markdown` |
| Go + cobra + viper | Proposed Changes > `internal/common/config`, 各 `cmd/main.go` |
| `-i/-o` 基本オプション | Proposed Changes > 各 `cmd/main.go` の `cobra` 定義 |
| `--stdin` 明示入力、`-i` 排他、未指定エラー | Proposed Changes > `internal/common/io/input_resolver.go` |
| `stdout` と `stderr` 分離 | Proposed Changes > `internal/common/io/output_writer.go` |
| 終了コード `0/1/2` | Proposed Changes > `internal/common/exitcode/exitcode.go` と各 `main.go` |
| 標準出力は生成物パス（1行1件） | Proposed Changes > 各 service が `[]string` を返し `output_writer` で出力 |
| クロスプラットフォームのパス処理 | Proposed Changes > `internal/common/pathutil/pathutil.go` |
| `image-to-markdown` の既存アルゴリズム再現 | Proposed Changes > `internal/imagetomd/analyzer/*.go` |
| `Classify/Assess/Question/Markdown` プロンプト互換 | Proposed Changes > `internal/imagetomd/analyzer/prompts.go` |
| `CreateSession -> ... -> TerminateSession` の順序互換 | Proposed Changes > `internal/imagetomd/analyzer/analyzer.go` |
| SessionLog 形式互換 | Proposed Changes > `internal/imagetomd/analyzer/session.go` |
| `-r/--ref` 正規表現複数指定 | Proposed Changes > `internal/imagetomd/refresolver/resolver.go` |
| `--config`, `--verbose`, `--quiet`, `--output-mode`, `--print-json` 必須 | Proposed Changes > `internal/common/config`, `internal/common/io/output_writer.go` |
| 改善モード（厳密判定、question保存、sleep可変）必須 | Proposed Changes > `internal/imagetomd/analyzer/gap_judge.go`, `analyzer.go`, `session.go` |
| 検証シナリオ（単独/連結/互換/改善差分） | Verification Plan > Automated Verification + E2E Tests |

## Proposed Changes

### `features/doc-convert`

#### [NEW] `features/doc-convert/internal/common/io/input_resolver_test.go`
*   **Description**: `-i` / `--stdin` / 未指定 / 併用エラーの解決ロジックをテーブル駆動で先に固定する（TDD の Red）。
*   **Technical Design**:
    *   ```go
        type ResolveInputArgs struct {
            InputPath string
            UseStdin  bool
            Stdin     io.Reader
        }

        func ResolveInputPaths(args ResolveInputArgs) ([]string, error)
        ```
*   **Logic**:
    *   `-i` があれば単一入力として返す。
    *   `--stdin` は 1 行 1 パスで読み込み、空行除去、重複除去。
    *   `-i` と `--stdin` 併用は `ErrInvalidArgs`。
    *   両方未指定は `ErrInputRequired`。

#### [NEW] `features/doc-convert/internal/common/io/output_writer_test.go`
*   **Description**: `stdout` 機械可読 / `stderr` ログ分離、`path|json` 出力、`--quiet` の挙動を先に固定する。
*   **Technical Design**:
    *   ```go
        type OutputMode string

        func WriteResultPaths(w io.Writer, mode OutputMode, paths []string) error
        func NewLogger(stderr io.Writer, verbose bool, quiet bool) *slog.Logger
        ```
*   **Logic**:
    *   `path` は 1 行 1 パス。
    *   `json` は JSON 配列文字列。
    *   ログは常に stderr。quiet 時は info 抑制、warn/error は許可。

#### [NEW] `features/doc-convert/internal/common/exitcode/exitcode_test.go`
*   **Description**: エラー種別 -> 終了コード 0/1/2 の写像を固定する。
*   **Technical Design**:
    *   ```go
        const (
            CodeOK          = 0
            CodeRuntimeErr  = 1
            CodeInvalidArgs = 2
        )

        func FromError(err error) int
        ```
*   **Logic**:
    *   引数/入力検証エラーは `2`。
    *   変換失敗や外部コマンド失敗は `1`。

#### [NEW] `features/doc-convert/internal/imagetomd/analyzer/gap_judge_test.go`
*   **Description**: 互換判定と厳密判定の差分を固定する。
*   **Technical Design**:
    *   ```go
        type GapJudgeMode string // "compat" | "strict"
        func IsSufficient(resp string, mode GapJudgeMode) bool
        ```
*   **Logic**:
    *   compat: `strings.Contains(strings.ToUpper(resp), "SUFFICIENT")`
    *   strict: `SUFFICIENT` 単独行、または `Decision:SUFFICIENT` のみ真

#### [NEW] `features/doc-convert/internal/imagetomd/analyzer/session_test.go`
*   **Description**: `RoundLog.Question` 保存、simple_text 経路ログ、JSON 形式互換を検証する。
*   **Technical Design**:
    *   ```go
        type RoundLog struct {
            KnownInfo     string `json:"known_info"`
            GapAssessment string `json:"gap_assessment"`
            Sufficient    bool   `json:"sufficient"`
            Question      string `json:"question"`
            Answer        string `json:"answer"`
        }
        ```
*   **Logic**:
    *   質問生成後に必ず `Question` 格納。
    *   simple_text は分類情報とショートパス理由を最低限保持。

#### [NEW] `features/doc-convert/tests/common_doc_convert_cli_test.go`
*   **Description**: 3コマンドの引数契約、`stdout/stderr` 分離、`--stdin` バリデーションを統合テストで確認。
*   **Technical Design**:
    *   `//go:build integration`
    *   `TestDocConvertCLI_InputContract`, `TestDocConvertCLI_OutputContract`
*   **Logic**:
    *   `--stdin` 未接続時に即 `exit 2`。
    *   `-i` + `--stdin` 併用時 `exit 2`。
    *   stdout にログ混入なし。

#### [NEW] `features/doc-convert/tests/llm_image_to_markdown_compat_test.go`
*   **Description**: `image-to-markdown` の既存互換（コール順、プロンプト構成、セッション出力）を統合テストで担保。
*   **Technical Design**:
    *   `//go:build integration`
    *   `TestImageToMarkdown_CompatFlow`, `TestImageToMarkdown_SimpleTextShortPath`
*   **Logic**:
    *   モック可能な Tern API サーバで呼び出し順を検証。
    *   `[Attached image: <absPath>]` の付与を検証。

#### [NEW] `features/doc-convert/tests/llm_image_to_markdown_improved_test.go`
*   **Description**: 改善モード（厳密判定、sleep可変、question保存）の統合テスト。
*   **Technical Design**:
    *   `//go:build integration`
    *   `TestImageToMarkdown_StrictGapJudge`, `TestImageToMarkdown_SleepOverride`
*   **Logic**:
    *   `NOT SUFFICIENT` で継続することを確認。
    *   sleep=0 設定で短時間完了、機能差分なしを確認。

#### [NEW] `features/doc-convert/internal/common/io/input_resolver.go`
*   **Description**: 入力元統一（`-i` or `--stdin`）を担う共通実装。
*   **Technical Design**:
    *   ```go
        func ResolveInputPaths(args ResolveInputArgs) ([]string, error) { /* ... */ }
        ```
*   **Logic**:
    *   stdin は `bufio.Scanner` で読み込み。
    *   パス正規化は `filepath.Clean` を通す。

#### [NEW] `features/doc-convert/internal/common/io/output_writer.go`
*   **Description**: 結果出力フォーマットとログ出力を標準化。
*   **Technical Design**:
    *   ```go
        func WriteResultPaths(w io.Writer, mode OutputMode, paths []string) error
        func ResolveOutputMode(outputMode string, printJSON bool) (OutputMode, error)
        ```
*   **Logic**:
    *   `--print-json` は `json` モードに正規化（互換エイリアス）。
    *   出力順は入力順を保持。

#### [NEW] `features/doc-convert/internal/common/exitcode/exitcode.go`
*   **Description**: すべてのコマンドで同一終了コード運用を保証。
*   **Technical Design**:
    *   ```go
        func ExitWithError(err error) {
            os.Exit(FromError(err))
        }
        ```
*   **Logic**:
    *   バリデーションエラーか実行時エラーかをラップ型で判定。

#### [NEW] `features/doc-convert/internal/common/config/config.go`
*   **Description**: `viper` 共通初期化（`--config` + ENV + defaults）。
*   **Technical Design**:
    *   ```go
        type CommonConfig struct {
            Verbose    bool
            Quiet      bool
            OutputMode string
            PrintJSON  bool
        }
        ```
*   **Logic**:
    *   優先順位 `CLI > ENV > config > default` を明示実装。

#### [NEW] `features/doc-convert/internal/exceltopdf/service.go`
*   **Description**: Excel 単体変換ロジック（1入力 -> 1PDF）と複数入力ループ。
*   **Technical Design**:
    *   ```go
        type Service interface {
            Convert(ctx context.Context, input string, outputDir string) (string, error)
        }
        ```
*   **Logic**:
    *   既定実装は LibreOffice CLI を利用し `<basename>.pdf` を生成。
    *   複数入力時は順次処理、失敗時は即エラー終了。

#### [NEW] `features/doc-convert/internal/pdftoimage/service.go`
*   **Description**: PDF 全ページ画像化（`png`/`jpg`）。
*   **Technical Design**:
    *   ```go
        type Service interface {
            Convert(ctx context.Context, inputPDF string, outputDir string, format string) ([]string, error)
        }
        ```
*   **Logic**:
    *   `<basename>_<nnn>.<ext>` で3桁ゼロ埋め。
    *   戻り値 `[]string` はページ順。

#### [NEW] `features/doc-convert/internal/imagetomd/refresolver/resolver.go`
*   **Description**: `-r/--ref` 正規表現の解決、重複除外、読み込み順制御。
*   **Technical Design**:
    *   ```go
        func ResolveRefs(patterns []string, root string) ([]RefDocument, error)
        ```
*   **Logic**:
    *   パターン順で評価し、同一パスは最初の一致を優先。

#### [NEW] `features/doc-convert/internal/imagetomd/analyzer/prompts.go`
*   **Description**: 既存仕様互換の固定文字列とテンプレートを実装。
*   **Technical Design**:
    *   ```go
        const ClassifyPrompt = `...`
        func AssessGapPrompt(phase Phase, known string) string
        func GenerateQuestionPrompt(phase Phase, gap string) string
        func GenerateMarkdownPrompt(phases []PhaseLog) string
        ```
*   **Logic**:
    *   固定 suffix と `[Attached image: <absPath>]` の付与を共通化。

#### [NEW] `features/doc-convert/internal/imagetomd/analyzer/session.go`
*   **Description**: 互換 JSON スキーマでセッション記録。
*   **Technical Design**:
    *   `SessionLog`, `PhaseLog`, `RoundLog` を仕様互換で定義。
*   **Logic**:
    *   `<output-dir>/_sessions/<basename>_session.json` へ保存。

#### [NEW] `features/doc-convert/internal/imagetomd/analyzer/analyzer.go`
*   **Description**: 既存フロー再現 + 改善オプション対応のコア実装。
*   **Technical Design**:
    *   ```go
        type AnalyzeOptions struct {
            StrictGapJudge bool
            SaveQuestionLog bool
            RoundSleepMS    int
            PhaseSleepMS    int
        }

        func (a *Analyzer) Analyze(ctx context.Context, imagePath string, workDir string, refs []RefDocument) (string, *SessionLog, error)
        ```
*   **Logic**:
    *   `CreateSession -> classify -> short path or phase loops -> final markdown -> terminate` を再現。
    *   `SaveQuestionLog` 有効時に必ず `RoundLog.Question` 保存。
    *   sleep は可変（既定5000ms）。

#### [NEW] `features/doc-convert/cmd/excel-to-pdf/main.go`
*   **Description**: `excel-to-pdf` CLI エントリ。
*   **Technical Design**:
    *   `--input/-i`, `--stdin`, `--output-dir/-o`, `--config`, `--verbose`, `--quiet`, `--output-mode`, `--print-json`
*   **Logic**:
    *   入力解決 -> サービス変換 -> stdout 出力 -> エラー時終了コード変換。

#### [NEW] `features/doc-convert/cmd/pdf-to-image/main.go`
*   **Description**: `pdf-to-image` CLI エントリ。
*   **Technical Design**:
    *   上記共通 + `--format (png|jpg)`。
*   **Logic**:
    *   入力1件ごとに画像群を生成し、すべての生成パスを stdout へ出力。

#### [NEW] `features/doc-convert/cmd/image-to-markdown/main.go`
*   **Description**: `image-to-markdown` CLI エントリ（再現 + 改善）。
*   **Technical Design**:
    *   共通フラグ + `--server`, `--agent`, `--model`, `--ref`, `--strict-gap-judge`, `--save-question-log`, `--round-sleep-ms`, `--phase-sleep-ms`
*   **Logic**:
    *   単一入力時は `--output` 基本、`--output-dir` も許容。
    *   `--stdin` 時は `--output-dir` 必須。

## Step-by-Step Implementation Guide

1.  **テスト基盤先行（TDD Red）**:
    *   Edit `features/doc-convert/internal/common/io/input_resolver_test.go` to 期待I/O契約テストを追加。
    *   Edit `features/doc-convert/internal/common/io/output_writer_test.go` to `stdout/stderr` 分離テストを追加。
    *   Edit `features/doc-convert/internal/imagetomd/analyzer/gap_judge_test.go` と `session_test.go` を追加。
2.  **共通基盤実装（TDD Green）**:
    *   Edit `features/doc-convert/internal/common/io/input_resolver.go` to `-i/--stdin` 解決実装。
    *   Edit `features/doc-convert/internal/common/io/output_writer.go` と `exitcode.go` を実装。
    *   Edit `features/doc-convert/internal/common/config/config.go` to `viper` 優先順位を実装。
3.  **`excel-to-pdf` 実装**:
    *   Edit `features/doc-convert/internal/exceltopdf/service.go` を実装。
    *   Edit `features/doc-convert/cmd/excel-to-pdf/main.go` を実装。
4.  **`pdf-to-image` 実装**:
    *   Edit `features/doc-convert/internal/pdftoimage/service.go` を実装。
    *   Edit `features/doc-convert/cmd/pdf-to-image/main.go` を実装。
5.  **`image-to-markdown` 再現実装**:
    *   Edit `features/doc-convert/internal/imagetomd/analyzer/prompts.go`, `session.go`, `analyzer.go` を実装。
    *   Edit `features/doc-convert/internal/imagetomd/refresolver/resolver.go` を実装。
    *   Edit `features/doc-convert/cmd/image-to-markdown/main.go` を実装。
6.  **統合テスト追加**:
    *   Edit `features/doc-convert/tests/common_doc_convert_cli_test.go` で CLI 契約テストを追加。
    *   Edit `features/doc-convert/tests/llm_image_to_markdown_compat_test.go` と `llm_image_to_markdown_improved_test.go` を追加。
7.  **リファクタと整形**:
    *   Edit 各実装ファイル to 重複排除（共通オプション定義、共通エラー整形）。
8.  **Verification Plan 実行**:
    *   下記 Automated Verification を順に実行し、結果を記録する。

## Verification Plan

### Automated Verification

1.  **Build & Unit Tests**:
    実装と単体テストの整合を確認する。
    ```bash
    ./scripts/process/build.sh
    ```
    *   **Log Verification**: `features/doc-convert/internal/...` のテストが失敗なく通過し、`stdout/stderr` 契約と `gap judge` テストが成功していること。

2.  **Integration Tests (common)**:
    CLI 契約とパイプ連携の回帰を確認する。
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "doc_convert|excel-to-pdf|pdf-to-image|stdin|output-mode"
    ```
    *   **Log Verification**: `--stdin` 未接続時 `exit 2`、`-i` 併用エラー、`stdout` にログ混入なしを確認。

3.  **Integration Tests (llm)**:
    `image-to-markdown` の既存互換と改善差分を確認する。
    ```bash
    ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "llm" --specify "image_to_markdown|compat|strict_gap|session_log"
    ```
    *   **Log Verification**: コールシーケンス、`[Attached image: ...]` 付与、`RoundLog.Question` 保存、`strict` 判定差分を確認。

4.  **E2E Tests (新規/追加)**:
    CLI 連鎖を実行する統合E2E相当テストを `tests/` 配下に追加する（手動コマンド確認は代替不可）。

    #### [NEW] `features/doc-convert/tests/e2e_doc_convert_pipeline_test.go`(file://features/doc-convert/tests/e2e_doc_convert_pipeline_test.go)
    *   **テストケース**:
        *   `TestPipeline_ExcelToMarkdown_WithStdin`
        *   `TestPipeline_StopsOnInvalidInputWithExitCode2`
    *   **検証ポイント**:
        *   3段パイプで最終Markdownまで到達する。
        *   中間生成物パスが stdout で受け渡される。
        *   失敗時に適切な終了コードで停止する。

### Test Item Design (Bottom-Up)

1. **末端（C）**: `input_resolver`, `output_writer`, `gap_judge` の純ロジック単体テスト。  
2. **中間（B）**: 各 service (`exceltopdf`, `pdftoimage`, `imagetomd analyzer`) の依存込み単体/準統合テスト。  
3. **上位（A）**: CLI エントリと3段パイプ E2E テスト。

#### 観点チェックリスト適用
- 正常系: 単一入力、複数入力、パイプ実行。  
- 異常系: 不正拡張子、入力未指定、`-i` と `--stdin` 併用。  
- 外部連携: LibreOffice / Poppler / Tern API 呼び出し。  
- データ一貫性: 生成パス順序、セッションJSON整合。  
- 状態遷移: Phase0 -> Phase1-4 -> Phase5、short path 分岐。  
- 設定反映: `--config`, ENV, CLI 優先順位。  
- 副作用: 一時ファイル残存、セッションログ出力漏れ。

#### テスト項目セルフレビュー（§11.4）
- **網羅性**: I/O契約、変換結果、アルゴリズム互換、改善差分を網羅。  
- **証拠十分性**: 戻り値だけでなく出力ファイル・ログ構造・送信プロンプトを検証。  
- **迂回排除**: 互換モード/改善モードを明示フラグで切替し、意図経路を検証。  
- **依存整合**: C->B->A の順で固定し、上位テストの妥当性を担保。

### Post-Test Comprehensive Verdict Plan

全テスト完了後、`Testing Rules` §12 のチェック項目（スキップ有無、部分エラー、迂回成功、設定誤適用、順序依存、カバレッジ、外部状態）を実施し、以下のフォーマットで判定記録する。

- 判定: `✅ 動作確認完了 / ⚠️ 条件付き確認完了 / ❌ 追加確認必要`
- 件数サマリ（成功/失敗/事実上スキップ）
- 7項目チェック表
- 判定理由と未解決リスク

## Documentation

`prompts/specifications` 配下の既存ドキュメントを確認し、本計画で直接更新が必要な対象は現時点でなし。

*   **更新内容**: None.（本変更は新規ツール追加であり、既存 `prompts/specifications` 文書への直接追記対象を伴わないため）

## Execution Progress

- [x] Step 1: テスト基盤（`input_resolver`, `output_writer`, `gap_judge`, `session`）を追加
- [x] Step 2: 共通基盤（I/O契約、終了コード、設定、logger）を実装
- [x] Step 3: `excel-to-pdf` を実装
- [x] Step 4: `pdf-to-image` を実装
- [x] Step 5: `image-to-markdown` 再現実装と改善オプションを実装
- [x] Step 6: 統合テスト/E2E相当テストファイルを追加
- [x] Step 7: ビルドスクリプトを複数パッケージ対応に修正
- [x] Step 8: `scripts/process/build.sh` 実行で成功を確認
- [/ ] Step 9: `integration_test.sh` の現行仕様に合わせた統合テスト運用の整備（このリポジトリでは `tests/go.mod` が未存在のため未実行）
