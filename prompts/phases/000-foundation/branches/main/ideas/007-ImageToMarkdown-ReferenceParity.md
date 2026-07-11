# 007 ImageToMarkdown Reference Parity

## 背景 (Background)

- `tmp/entext-test/images/pc/01_変更履歴.png` から生成された `tmp/entext-test/md/pc/01_変更履歴.md` が、画像内容と等価ではない。
- ユーザー提供の参照実装 `tmp/reference/image-to-markdown` は同種画像で期待どおり動作している。
- 現行 `internal/imagetomd` は、参照実装に対して制御フロー・応答取り込み・失敗時挙動が乖離しており、Markdown再現性を損なっている。
- 本仕様では、参照実装を「正解」とし、現行実装を参照実装準拠へ修正する。

## 要件 (Requirements)

### 必須要件

1. `internal/imagetomd/analyzer` の主要アルゴリズム（分類→Phase 1-4適応ループ→最終統合）を、`tmp/reference/image-to-markdown/internal/analyzer` と同等の処理順序・責務に揃えること。
2. 最終成果物は「画像内容をMarkdown化した本文」であり、`Phase`/`Q:`/`A:`/`round` などの解析ログ表現を含まないこと。
3. `tmp/entext-test/images/pc/01_変更履歴.png` の変換結果に、少なくとも次を含むこと:
   - 見出し行: `No.`, `変更箇所`, `変更内容(変更理由)`, `Ver`, `作成・変更者`, `作成・変更日`, `承認者`, `承認日`, `備考`
   - No.43 と No.44 の行データ
   - `28X-REQ-A0220` および `28P-ITb1-0243` の文字列
4. `tmp/entext-test/images/pc/02_書き換えルール.png` の変換結果が空でないこと、かつメイン表を表すMarkdownテーブルを含むこと。
5. ストリーム応答の採用優先は参照実装に合わせ、本文チャンク（`OnText`）を優先して連結すること。
6. 応答の機械判定（例: plan-only 判定）で本文候補を過剰に破棄しないこと。破棄条件は明示的かつ最小化すること。
7. Phase 3/4 の追加ラウンド拡張は無制限増加を禁止し、上限と発火回数を明示すること。
8. `--tern-mode inproc` の起動失敗（`agent service port was not assigned`）を低減し、少なくとも再現頻度を大幅に下げる実装（待機条件・エラーハンドリング改善）を行うこと。
9. 既存CLI契約（`--stdin`、`stdout`/`stderr`分離、exit code方針）を壊さないこと。
10. 公開API `ConvertImageToMarkdown` でも同じ品質で動作すること（CLI専用の分岐にしない）。
11. 参照実装の `adaptiveLoop` 互換の詳細ログ（Round開始/終了、sufficient判定、拡張理由）を `--verbose` 時に統一的に出す。
12. 変換品質の回帰を防ぐため、固定入力画像に対するゴールデンテストを追加する。

## 実現方針 (Implementation Approach)

### 1. 差分分析（現行 vs 参照）と採用方針

1. **ストリーム応答集約**
   - 参照: `collectStreamText()` で `OnText` のみを連結採用。
   - 現行: `OnResult` 優先の経路があり、本文が脱落しうる。
   - 方針: 参照準拠で `OnText` 優先を正式仕様化。

2. **最終統合フェーズの責務**
   - 参照: Phase結果を素材に最終Markdownを1回生成。失敗時は失敗として扱う。
   - 現行: 空応答/Phaseレポート判定、再試行、独自フォールバックが混在。
   - 方針: 参照の責務分離に戻し、必要なら「再試行は1回まで」「ログ出力のみ」など単純化。

3. **ヒューリスティック判定の導入範囲**
   - 参照: plan-only の本文破棄ロジックなし。
   - 現行: plan-only 判定で回答除外が発生。
   - 方針: 本文除外は最小化し、除外時は必ず理由を記録。既定では除外しない。

4. **ラウンド拡張ロジック**
   - 参照: Phase 3/4 の動的拡張意図あり（実装は保守要）。
   - 現行: 拡張条件次第で長時間化しやすい。
   - 方針: 拡張は最大1回、追加回数固定、全体上限を導入。

5. **Tern in-process 起動安定性**
   - 参照: 外部サーバ接続中心で起動問題の影響が小さい。
   - 現行: inproc で `agent service port was not assigned` が顕在化。
   - 方針: `waitForAgentPort` / `waitForHealth` の待機戦略、`Launch` 早期失敗の検出、失敗時ログを改善し、再試行戦略を明確化。

### 2. 実装単位

1. `internal/imagetomd/analyzer/analyzer.go`
   - 参照実装の `Analyze()` / `adaptiveLoop()` をベースに現行APIへ移植。
   - 余分なフォールバック経路を整理。

2. `internal/imagetomd/analyzer/prompts.go`
   - 参照実装の `PhaseDefinition`/`Phases` とプロンプト文面を正本として整合化。

3. `internal/imagetomd/tern/client.go`
   - ストリーム集約の採用順序を参照準拠に固定化。

4. `internal/imagetomd/tern/runtime.go`
   - inproc起動の待機・失敗診断を改善。

5. `cmd/image-to-markdown/main.go` / `entext.go`
   - 上記修正をCLI/API経由で利用可能にし、契約維持を確認。

## 検証シナリオ (Verification Scenarios)

1. **差分の再現確認（現状失敗の固定化）**
   1. 現行実装で `tmp/entext-test/images/pc/01_変更履歴.png` を変換。
   2. `01_変更履歴.md` が画像と不整合（行欠落・不明値多発・Phaseログ混入）であることを確認。

2. **参照準拠修正後の変換確認（01_変更履歴）**
   1. 修正後実装で同画像を変換。
   2. 先頭が正規のMarkdown表であることを確認。
   3. `28X-REQ-A0220` / `28P-ITb1-0243` / No.43 / No.44 が本文に含まれることを確認。
   4. `Phase` / `Q:` / `A:` / `round` が本文に含まれないことを確認。

3. **参照準拠修正後の変換確認（02_書き換えルール）**
   1. `tmp/entext-test/images/pc/02_書き換えルール.png` を変換。
   2. 出力が空でないことを確認。
   3. メイン表を表すMarkdownテーブルが含まれることを確認。

4. **inproc起動安定性確認**
   1. `--tern-mode inproc --tern-config settings/tern/tern-config.yaml` で連続実行（複数回）。
   2. `agent service port was not assigned` の発生率が修正前より改善することを確認。

5. **CLI/API後方互換確認**
   1. CLI: `--stdin`, `--output-mode`, `--verbose` を使った実行で既存契約が維持されることを確認。
   2. API: `ConvertImageToMarkdown` でも同じ品質で変換できることを確認。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `./scripts/process/build.sh`

2. 参照準拠差分テスト（analyzer/tern）:
   - `go test ./internal/imagetomd/analyzer ./internal/imagetomd/tern ./cmd/image-to-markdown`

3. 統合テスト（image-to-markdown関連）:
   - `./scripts/process/integration_test.sh --specify "ImageToMarkdown|Tern|Config|RootAPIValidation"`

4. 固定入力画像の回帰テスト（新規）:
   - `./scripts/process/integration_test.sh --specify "ChangeHistory|RewriteRules|ReferenceParity|NoPhaseReport"`

### 要件対応表

- 要件1-4,10（参照準拠・品質）:
  - `go test ./internal/imagetomd/analyzer ./cmd/image-to-markdown`
  - `integration_test.sh --specify "ChangeHistory|RewriteRules|ReferenceParity"`
- 要件5-7（応答採用/判定/ラウンド制御）:
  - `go test ./internal/imagetomd/tern ./internal/imagetomd/analyzer`
- 要件8（inproc安定性）:
  - `integration_test.sh --specify "Tern|InProcess|Config"`
- 要件9（CLI契約維持）:
  - `go test ./cmd/image-to-markdown`
  - `integration_test.sh --specify "stdin|output-mode|exit code"`

## 実装結果メモ（2026-07-11）

- `plan-only` 判定は制御分岐から外し、ログ注記用途に限定した。
- 最終合成が `Phase/Q/A` 形式または空文字のときは再合成を1回実施し、それでも不正なら `ErrEmptyMarkdown` を返す契約を固定した。
- `--verbose` 時の追跡ログを `step=...` ベースへ統一（phase/round/sufficient/answer_chars/retry_reason/elapsed）。
- 参照パリティの固定データを `tests/testdata/reference_parity/` に追加し、禁止トークン混入の回帰を防止した。
