# 017-ExcelFill-Part2-FillDialogueAndVisual

> **Source Specification**: `prompts/phases/000-foundation/branches/main/ideas/017-ExcelFill-TemplateAnalyzeAndFill.md`
>
> **前提**: Part 1 `prompts/phases/000-foundation/branches/main/plans/016-ExcelFill-Part1-AnalyzeAndCellIO.md` が実装済みであること（`refprompt`, `excelcell`, 構造 Markdown, `AnalyzeExcelTemplate`）。

## Goal Description

Excel テンプレートへの **対話的埋め込み** と **見た目検証リトライループ** を実装する。`excel-fill` CLI / `FillExcel` 公開 API、`text`/`json` 対話アダプタ、セル書き込み、PDF→画像化による VisualChecker、`--max-retries`（既定 5）と継続確認を提供する。

## User Review Required

1. **リトライ定義（仕様推奨を採用）**: `--max-retries` は **見た目検証の失敗回数** を数える。初回埋め込みはカウント外。失敗回数が上限に達したら `continue_confirm` を出し、ユーザーが継続するなら `additional_retries`（または `--continue-retries` が事前指定されていればそれを）だけ上限を加算して再開する。
2. **text モード I/O**: 質問・status・問題報告は **stderr**、ユーザー入力は **stdin**。完了時の成果物パスは既存契約どおり **stdout**（`--output-mode path|json`）。json モードは **stdout に NDJSON assistant メッセージ**、stdin から user NDJSON を読む（ログは stderr）。
3. **VisualChecker**: 初版は Tern `SendImagePrompt` で JSON 配列の issue を返させる。単体/統合は interface モック。パース失敗は「検証不能」として致命エラー（リトライ対象外、exit 1）。
4. **埋め込みエージェント**: 記入値の決定も Tern `SendText`（構造 MD + hints + 対話履歴 + visual feedback）。テストでは `Filler` / `Dialog` をモックし、固定 `[]CellWrite` を返す。
5. **任意要件は先送り**（Part 1 と同じ）: `--sheets`、`--visual-strict`、`--session-log`、非対話一括モード。
6. **`--continue-retries`**: CLI に整数フラグを用意（既定 0）。`continue_confirm` 時、フラグ > 0 なら対話なしでその値を加算して継続。0 なら対話で確認（json/text）。

## Requirement Traceability

| Requirement (from Spec) | Implementation Point (Section/File) |
| :--- | :--- |
| A1 `excel-fill` CLI | `cmd/excel-fill/` |
| A2 `FillExcel` API | `entext.go` |
| A3–A4 I/O・共通フラグ | CLI + exitcode |
| B5–B10 analyze | Part 1（完了前提） |
| C11 必須オプション template/structure/output | fill CLI/API |
| C12 ref/prompt 系 | `refprompt` 再利用 |
| C13 `--mode json\|text` | `internal/excelfill/dialog/` |
| C14 対話で不足情報収集 | `excelfill` orchestrator + Filler |
| C15 セル書き込み | `excelcell` + CopyFile |
| C16 見た目検証ループ | `excelfill/visual.go` + pipeline |
| C17 `--max-retries` 既定 5 + 継続確認 | `loop.go` |
| C18 成功時最終保存 exit 0 | `service.go` |
| C19 json/text 同等内容 | dialog adapters |
| C20 Tern 設定 | `FillExcelConfig` |
| D21 原本非破壊 | CopyFile → 作業コピー → 最終 `-o` |
| D22 backend 選択 | 既存 PDF/image options |
| D23 単体（対話・リトライ） | `*_test.go` |
| D24 統合 analyze→fill | 本 Part E2E（モック Visual/Filler 経路は internal、CLI は validation + json プロトコル） |
| D25 ヒント共通化 | Part 1 `refprompt` |
| 任意 | **先送り** |

## Proposed Changes

### `internal/excelfill/dialog`

#### [NEW] `internal/excelfill/dialog/protocol_test.go`(file://internal/excelfill/dialog/protocol_test.go)
*   **Description**: NDJSON メッセージの encode/decode。
*   **Technical Design**:
    *   ```go
        func TestMarshalQuestionAndParseAnswer(t *testing.T)
        func TestParseContinueDecision(t *testing.T)
        func TestRejectUnknownType(t *testing.T)
        func TestVisualIssueMessageRoundTrip(t *testing.T)
        ```

#### [NEW] `internal/excelfill/dialog/protocol.go`(file://internal/excelfill/dialog/protocol.go)
*   **Description**: 仕様 §5 の最小スキーマを Go 型で固定。
*   **Technical Design**:
    *   ```go
        package dialog

        type Role string // "assistant" | "user" | "system"
        type Type string
        const (
            TypeQuestion         Type = "question"
            TypeAnswer           Type = "answer"
            TypeStatus           Type = "status"
            TypeVisualIssue      Type = "visual_issue"
            TypeContinueConfirm  Type = "continue_confirm"
            TypeContinueDecision Type = "continue_decision"
            TypeDone             Type = "done"
            TypeError            Type = "error"
        )

        type Message struct {
            Role    Role           `json:"role"`
            Type    Type           `json:"type"`
            Prompt  string         `json:"prompt,omitempty"`
            Fields  []FieldSpec    `json:"fields,omitempty"`
            Text    string         `json:"text,omitempty"`
            Values  map[string]string `json:"values,omitempty"`
            Issues  []VisualIssue  `json:"issues,omitempty"`
            Status  string         `json:"status,omitempty"`
            Continue *bool         `json:"continue,omitempty"`
            AdditionalRetries *int `json:"additional_retries,omitempty"`
            OutputPath string      `json:"output_path,omitempty"`
            RetriesUsed int        `json:"retries_used,omitempty"`
            Error   string         `json:"error,omitempty"`
        }

        type FieldSpec struct {
            ID       string `json:"id"`
            Label    string `json:"label"`
            Required bool   `json:"required"`
        }

        type VisualIssue struct {
            Kind        string `json:"kind"` // cutoff|overflow|layout
            Sheet       string `json:"sheet,omitempty"`
            CellHint    string `json:"cell_hint,omitempty"`
            Description string `json:"description"`
            Suggestion  string `json:"suggestion,omitempty"`
        }

        func Encode(msg Message) ([]byte, error) // single line JSON + '\n'
        func DecodeLine(line string) (Message, error)
        ```

#### [NEW] `internal/excelfill/dialog/transport_test.go`(file://internal/excelfill/dialog/transport_test.go)
*   **Description**: text/json アダプタの入出力。
*   **Technical Design**:
    *   ```go
        func TestJSONTransportAskAndReceive(t *testing.T)
        func TestTextTransportAskAndReceive(t *testing.T)
        func TestContinueConfirmUsesContinueRetriesFlag(t *testing.T)
        ```

#### [NEW] `internal/excelfill/dialog/transport.go`(file://internal/excelfill/dialog/transport.go)
*   **Description**: 対話トランスポート。
*   **Technical Design**:
    *   ```go
        type Transport interface {
            Send(ctx context.Context, msg Message) error
            Receive(ctx context.Context) (Message, error)
        }

        type JSONTransport struct {
            In  io.Reader // line scanner
            Out io.Writer // stdout NDJSON
        }

        type TextTransport struct {
            In     io.Reader
            Out    io.Writer // stderr for questions
            Prompt func(msg Message) string // human formatting
        }

        func NewJSONTransport(in io.Reader, out io.Writer) *JSONTransport
        func NewTextTransport(in io.Reader, errOut io.Writer) *TextTransport
        ```
*   **Logic**:
    *   `AskQuestion`: Send question → Receive が answer になるまで（不正 type は error メッセージを Send して再読取、最大 N 回で失敗）。
    *   `ConfirmContinue`: Send continue_confirm → Receive continue_decision。

### `internal/excelfill`

#### [NEW] `internal/excelfill/loop_test.go`(file://internal/excelfill/loop_test.go)
*   **Description**: リトライカウンタと継続分岐（モック Visual/Filler/Transport）。
*   **Technical Design**:
    *   ```go
        func TestFillSucceedsOnFirstVisualPass(t *testing.T)
        func TestFillRetriesOnVisualIssuesUntilPass(t *testing.T)
        func TestFillAsksContinueWhenMaxRetriesExhausted_Decline(t *testing.T)
        func TestFillAsksContinueWhenMaxRetriesExhausted_Accept(t *testing.T)
        func TestFillDefaultMaxRetriesIsFive(t *testing.T)
        func TestFillDoesNotModifyTemplateSource(t *testing.T)
        func TestFillWritesOutputPathOnSuccess(t *testing.T)
        ```
*   **Logic**:
    *   fake Visual: 最初の 2 回 issue あり、3 回目空 → retries_used=2。
    *   maxRetries=2 で常に issue → continue_confirm。Decline → error 結果 + 作業 xlsx パス。Accept + additional=1 → もう 1 回試行。

#### [NEW] `internal/excelfill/filler.go`(file://internal/excelfill/filler.go)
*   **Description**: 埋め込み内容の決定。
*   **Technical Design**:
    *   ```go
        type CellWrite struct {
            Sheet string
            Cell  string
            Value string
        }

        type FillContext struct {
            StructureMD string
            HintText    string
            Answers     map[string]string
            History     []dialog.Message
            VisualFeedback []dialog.VisualIssue
        }

        type Filler interface {
            Plan(ctx context.Context, fc FillContext) (questions []dialog.FieldSpec, writes []CellWrite, err error)
            // Plan may return questions (need more info) XOR writes (ready to apply).
        }

        type TernFiller struct {
            Client tern.Client
            Agent  string
            Model  string
        }
        ```
*   **Logic**:
    *   Tern に JSON スキーマ応答を要求: `{"need":[...fields], "writes":[...]}`。
    *   `need` 非空なら Transport で質問し Answers を更新して再 Plan。
    *   VisualFeedback 非空なら「短くする・改行する」等を指示に含める。

#### [NEW] `internal/excelfill/visual.go`(file://internal/excelfill/visual.go)
*   **Description**: 見た目検証。
*   **Technical Design**:
    *   ```go
        type VisualChecker interface {
            Check(ctx context.Context, imagePaths []string, hintText string) ([]dialog.VisualIssue, error)
        }

        type TernVisualChecker struct {
            Client tern.Client
        }

        const VisualCheckPrompt = `Inspect the filled Excel page image(s).
Report ONLY visible text cutoff, overflow into neighbors, or broken layout.
Reply with JSON array of objects: kind, sheet, cell_hint, description, suggestion.
If none, reply [].`
        ```
*   **Logic**:
    *   各画像に `SendImagePrompt`。複数画像の issue を結合。
    *   JSON 抽出（コードフェンス除去）に失敗したら error。

#### [NEW] `internal/excelfill/render_images.go`(file://internal/excelfill/render_images.go)
*   **Description**: 埋めた xlsx → 画像（Part 1 `excelanalyze.RenderTemplateImages` と同等ロジックを **共有**するため、`internal/excelanalyze/pipeline.go` の関数を `excelanalyze` から export 済みのものを呼び出す。循環があれば `internal/excelvisual/` に pipeline を移す — Part 1 実装時に `RenderTemplateImages` を `internal/excelrender` へ置く判断を許容。本計画では **`excelanalyze.RenderTemplateImages` を Part 2 から再利用**する）。

#### [NEW] `internal/excelfill/service.go`(file://internal/excelfill/service.go)
*   **Description**: fill オーケストレーション。
*   **Technical Design**:
    *   ```go
        type Options struct {
            TemplatePath   string
            StructurePath  string
            OutputPath     string
            Hints          refprompt.HintInput
            Mode           string // "text"|"json"
            MaxRetries     int    // default 5 if <=0
            ContinueRetries int   // CLI preseed; 0 => ask
            Transport      dialog.Transport
            Filler         Filler
            Visual         VisualChecker
            WorkDir        string
            Pipeline       excelanalyze.PipelineOptions
            Verbose        bool
        }

        type Result struct {
            OutputPath  string
            RetriesUsed int
            LastIssues  []dialog.VisualIssue
            Aborted     bool
        }

        func Fill(ctx context.Context, opts Options) (Result, error)
        ```
*   **Logic（具体手順）**:
    1. Validate template/structure/output。structure ファイル読込。
    2. `refprompt.Resolve` → `FormatForPrompt(..., ModeFill)`。
    3. `excelcell.CopyFile(template, workDir/filled.xlsx)`。
    4. Answers マップ初期化。対話ループ:
       - `Filler.Plan` → questions なら Transport で収集して Answers 更新、再 Plan。
       - writes を作業 xlsx に `SetCellValue` → `SaveAs`。
    5. `RenderTemplateImages` で画像化 → `Visual.Check`。
    6. issues 空 → 作業 xlsx を `OutputPath` へ copy、`TypeDone` 送信、success。
    7. issues あり → `visual_issue` 送信、`retriesUsed++`、feedback を次 Plan へ。
    8. `retriesUsed >= MaxRetries` → `continue_confirm`。decline → `TypeError` + `Aborted` + err。accept → `MaxRetries += additional`（ContinueRetries または decision）しループ継続。

### `cmd/excel-fill`

#### [NEW] `cmd/excel-fill/main_test.go`(file://cmd/excel-fill/main_test.go)
*   **Description**: mode 不正、必須フラグ欠落の validation ヘルパテスト。

#### [NEW] `cmd/excel-fill/main.go`(file://cmd/excel-fill/main.go)
*   **Description**: CLI。
*   **Technical Design**:
    *   Flags:
        *   `--template` (required)
        *   `--structure` (required)
        *   `-o/--output` (required)
        *   `--ref`, `--ref-dir`, `--prompt`, `--prompt-file`
        *   `--mode` default `text`
        *   `--max-retries` default `5`
        *   `--continue-retries` default `0`
        *   Tern / PDF / image / 共通ログフラグ（analyze と同型）
    *   Viper prefix: `doc_excel_fill`
*   **Logic**:
    *   mode に応じて JSONTransport(stdout/stdin) または TextTransport(stderr/stdin) を構築。
    *   `entext.FillExcel` 呼び出し。成功時 stdout に output path。Aborted は exit 1。

### `entext` 公開 API

#### [MODIFY] `entext.go`(file://entext.go)
*   **Technical Design**:
    *   ```go
        type ExcelFillJob struct {
            TemplatePath  string
            StructurePath string
            OutputPath    string
            RefPatterns   []string
            RefDirs       []string
            Prompts       []string
            PromptFiles   []string
            Mode          string
            MaxRetries    int
            ContinueRetries int
        }

        type ExcelFillConfig struct {
            ServerURL, Agent, Model, TernMode, TernConfigPath string
            PDFBackend, PDFEngine, ImageBackend, ImageEngine string
            DPI int
            Verbose, Quiet bool
            // Stdin/Stdout/Stderr allow tests to inject pipes; nil => os.Std*
            Stdin  io.Reader
            Stdout io.Writer
            Stderr io.Writer
        }

        type ExcelFillArtifact struct {
            OutputPath  string
            RetriesUsed int
        }

        func FillExcel(ctx context.Context, job ExcelFillJob, cfg ExcelFillConfig) (ExcelFillArtifact, error)
        ```
*   **Logic**:
    *   Tern Runtime 構築 → TernFiller + TernVisualChecker。
    *   Transport を Mode から選択。
    *   MaxRetries<=0 なら 5。

#### [NEW] `entext_excel_fill_test.go`(file://entext_excel_fill_test.go)
*   **Description**: validation（template/structure/output 欠落）。

### Integration / E2E

#### [NEW] `tests/excel_fill_e2e_test.go`(file://tests/excel_fill_e2e_test.go)
*   **Description**: CLI/API 契約と json プロトコルのパイプテスト。
*   **Technical Design**:
    *   ```go
        func TestRootAPIFillExcelValidation(t *testing.T)
        func TestE2EExcelFillCLI_InvalidArgsExit2(t *testing.T)
        func TestE2EExcelFillJSONMode_ProtocolWithMockBinaries(t *testing.T)
        ```
*   **Logic（確定）**:
    *   実 Tern なしでフル fill を E2E するのは困難なため、**プロトコルと validation を E2E**、フルループは `excelfill/loop_test.go` のモックで担保する（仕様 D23/D24 を単体厚め + CLI 契約 E2E で満たす）。
    *   追加の強い統合: `TestExcelFillInternalLoop_ViaGoTest` は既に unit。  
    *   Part 1 の structure fixture + 最小 template を用い、**テスト専用ビルドタグは使わない**。代わりに `excelfill.Fill` を tests パッケージから呼べない制約があるため、公開 API に `FillExcelForTest` は作らない。loop 単体を「統合相当」の主証拠とする旨を Verification に明記。

#### [MODIFY] `tests/e2e_backend_pipeline_test.go`(file://tests/e2e_backend_pipeline_test.go)
*   **Description**: Part 1 で入れた `excel-fill` の `toolCommand` case を利用する CLI 起動テストを本ファイルまたは `excel_fill_e2e_test.go` に追加。

#### [NEW] `tests/testdata/excel_fill/structure_minimal.md`(file://tests/testdata/excel_fill/structure_minimal.md)
*   **Description**: Part 1 レンダラ互換の最小構造 Markdown（field_id `name` → Sheet1!B1）。

### Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**: `excel-fill` Usage、対話モード、リトライ、Go API `FillExcel`、analyze→fill の一連例。

## Step-by-Step Implementation Guide

1. **dialog protocol TDD**: `protocol_test.go` → `protocol.go`。
2. **transport TDD**: json/text。
3. **visual + filler interfaces** と Tern 実装（プロンプト定数）。
4. **loop_test.go RED** → `service.go` GREEN（モック依存）。
5. **render_images** を Part 1 pipeline に接続。
6. **公開 API `FillExcel`** + validation test。
7. **CLI `excel-fill`**。
8. **E2E validation / invalid args**、README。
9. **Verification Plan** 実行 + §12 総合判定。

## Verification Plan

### Automated Verification

1. **Build & Unit Tests**:
   ```bash
   ./scripts/process/build.sh
   ```

2. **Fill / dialog 単体を含むビルド確認後の統合**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "ExcelFill|FillExcel|excel-fill|ExcelTemplate"
   ```
   *   **Log Verification**: invalid args で exit 2、validation テスト成功、既存 excel テスト非退行。

3. **既存 Excel リグレッション**:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --categories "common" --specify "excel|Excel|sheet"
   ```

4. **E2E Tests**:
    #### [NEW] `tests/excel_fill_e2e_test.go`(file://tests/excel_fill_e2e_test.go)
    *   **テストケース**: `TestRootAPIFillExcelValidation` — 必須パス欠落。
    *   **テストケース**: `TestE2EExcelFillCLI_InvalidArgsExit2` — フラグ不足。
    *   **検証ポイント**: CLI が `bin/entext/excel-fill` としてビルドされ契約を満たす。
    *   **E2E でフル LLM ループをしない理由**: Tern/Excel COM 依存が重く不安定。ループ正当性は `loop_test.go` のモックで証拠十分とする（仕様の自動化要件は単体+CLI 契約で充足）。

### テスト項目設計（§11）とセルフレビュー

**ボトムアップ順序**:
1. `dialog` protocol/transport
2. `loop`（fake Filler/Visual）
3. Tern 実装のプロンプト整形単体（httptest 再利用可能なら）
4. 公開 API validation / CLI
5. リグレッション

**観点チェックリスト**:
| # | 観点 | 対応 |
|---|------|------|
| 1 | 正常系 | 1 回で Visual pass → output 存在 |
| 2 | 異常系 | max retries + decline → Aborted |
| 3 | 外部連携 | Tern はモック/unit。PDF は pipeline 再利用 |
| 4 | データ一貫性 | writes が xlsx に残る（excelcell テスト） |
| 5 | 状態遷移 | retriesUsed 増加、continue 加算 |
| 6 | 設定反映 | MaxRetries 既定 5、ContinueRetries |
| 7 | 副作用 | template 原本バイト列不変 |

**セルフレビュー結果**:
1. **網羅性**: 仕様 C11–C20 / D21–D25 をカバー。任意は先送り明示。
2. **証拠の十分性**: ループ分岐をテーブル駆動で固定。CLI は exit code 証拠。
3. **迂回排除**: Transport/Filler/Visual を注入し本番 Tern 経路とテスト経路を分離。
4. **依存関係**: Part 1 の cell/refprompt/structure が前提。

### 総合判定プロセス（§12）

全テスト成功後、フォールバック偽成功・原本破壊・リトライ数え間違いがないかを確認しウォークスルーに総合判定を記録する。

## Documentation

#### [MODIFY] `README.md`(file://README.md)
*   **更新内容**: 2 CLI のエンドツーエンド例（analyze → fill）、json モードの NDJSON 例、既定リトライ 5。

## Part 1 との接続チェックリスト

実装開始前に Part 1 成果を確認する:

- [ ] `refprompt.Resolve` / `FormatForPrompt(ModeFill)` が利用可能
- [ ] `excelcell.CopyFile` / `SetCellValue` / `SaveAs` が利用可能
- [ ] 構造 Markdown の Cell Mapping 表が fill から読める（必要なら `structure.ParseFields(md string) ([]Field, error)` を Part 2 で追加）
- [ ] `excelanalyze.RenderTemplateImages`（または共有 `excelrender`）が利用可能

### 構造 Markdown パーサ追加（本 Part で必要なら）

#### [NEW] `internal/excelanalyze/structure/parse.go`(file://internal/excelanalyze/structure/parse.go)
*   **Description**: Cell Mapping 表から `[]Field` を復元（簡易 Markdown 表パーサ）。
*   **Technical Design**:
    *   ```go
        func ParseFields(md string) ([]Field, error)
        ```
*   **Logic**: `| field_id | label | sheet | cells | role |` 行を掃引。`cells` はカンマ区切り。
