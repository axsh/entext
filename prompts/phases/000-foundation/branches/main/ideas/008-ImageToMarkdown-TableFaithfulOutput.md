# 008 ImageToMarkdown Table-Faithful Output

## 背景 (Background)

- `tmp/output/pc/images/01_変更履歴.png` を `image-to-markdown` で変換した `tmp/output/pc/md/01_変更履歴.md` が、画像内テーブルの忠実な写しではなく、表の説明・解析レポートに寄った内容になっている。
- 出力には `## 要素一覧（Phase 1）`、`## 書式・注記・セル結合・突合`、`## 図解要素` など、解析フェーズ由来のメタ情報セクションが含まれ、ユーザー期待の「9列変更履歴表をそのまま Markdown 化した本文」と乖離している。
- `tmp/reference/image-to-markdown/internal/analyzer/prompts.go` と `internal/imagetomd/analyzer/prompts.go` を比較した結果、**SUB_TABLE 指示（`[SUB_TABLE_P01]` プレースホルダーと別セクション展開）は参照実装と同一**であり、説明寄り出力の主因は SUB_TABLE 指示そのものではない。
- セッションログ `tmp/output/pc/md/_sessions/01_変更履歴_session.json` では、Phase 2（データの網羅的な読み取り）が 1 ラウンドで終了している。gap 判定文に `SUFFICIENT ではありません` が含まれるにもかかわらず `sufficient: true` となっており、`IsSufficient` の部分文字列マッチ（`compat` モード）が誤判定を起こしている疑いが強い。
- 現行の 4 フェーズ設計は Phase 1（全体概要）、Phase 3（構造関係）、Phase 4（暗黙知の言語化）で**解釈・意味付け・メタ表**の収集を明示的に要求しており、最終統合プロンプトがそれらを「構造化 Markdown」として統合するため、成果物が説明書化しやすい。
- `007-ImageToMarkdown-ReferenceParity` はストリーム集約・inproc 安定性・Phase ログ混入防止を扱ったが、**最終成果物の情報設計（原表中心 vs 解析レポート中心）**までは十分に制約していない。本仕様でそのギャップを埋める。

## 要件 (Requirements)

### 必須要件

1. 最終成果物の**中心**は、画像内の原表を表す Markdown テーブルであること。説明・解釈・Phase 名・要素 ID 体系は補助情報に留め、本文の主役にしないこと。
2. `tmp/output/pc/images/01_変更履歴.png` の変換結果に、少なくとも次を**原表セクション**として含むこと:
   - 見出し行: `No.`, `変更箇所`, `変更内容(変更理由)`, `Ver`, `作成・変更者`, `作成・変更日`, `承認者`, `承認日`, `備考`
   - No.43 行: `秋葉達也`, `2025/7/7`, `28X-REQ-A0220`, `^/search/event.html(.*)` など画像と一致するセル値
   - No.44 行: `藤本華子`, `2025/12/9`, `28P-ITb1-0243`, 修正前後の URL 条件
   - 画像下部の空行（罫線のみの行）を省略しないこと
3. 最終成果物に、次の**解析レポート専用セクションを出力しない**こと（禁止例）:
   - `要素一覧（Phase` / `Phase 1` / `Phase 2` など Phase 番号付き見出し
   - `書式・注記・セル結合・突合` のような書式解釈専用表（凡例が必要な場合は原表セル内または最小限の脚注に限定）
   - `意味対応・解釈` / `図解概要` / `要素ID` を列に持つメタ解析表
4. 入れ子セル（変更内容欄の多段リスト・修正前後ブロック）は、**親テーブルの該当セルにリンクを置き、別セクションで原文を展開**する既存方針（`[SUB_TABLE_P01]` / `[詳細](#sub_table_p01)`）は維持すること。ただし展開セクションは「原文データの再掲」に限定し、構造解析・意味解釈の文章化は含めないこと。
5. `IsSufficient` の既定判定は、`SUFFICIENT ではありません` のような否定文を**充足と誤判定しない**こと。`compat` モードでも否定パターンを除外するか、既定を `strict` 相当に変更すること。
6. Phase 2（データの網羅的な読み取り）は、gap 判定が充足でない限り**実データ読み取りラウンドを実行**すること。Phase 2 が 0 件の `answer` のまま `soft_limit` 終了してはならない（画像未提示など通信失敗を除く）。
7. Phase 1 / Phase 3 / Phase 4 で生成された中間表（要素 ID 表、隣接遷移表、書式解釈表など）は、最終統合の**入力素材**としては利用してよいが、最終 Markdown への**そのままの転載を禁止**すること。
8. 最終統合プロンプト（`GenerateMarkdownPrompt` / 再試行プロンプト）に、「原表再現を最上位とし、解析メタ表・凡例・意味説明セクションの出力禁止」を明文化すること。
9. `007` で導入した契約（CLI/API 後方互換、`OnText` 優先、inproc 起動、Phase ログ文字列の本文混入禁止）は維持すること。
10. 公開 API `ConvertImageToMarkdown` と CLI `image-to-markdown` で同一品質を保証すること。

### 任意要件

1. `--strict-gap-judge` 未指定時も誤判定が起きにくい既定値へ変更すること（後方互換フラグは維持）。
2. `complex_table` 向けに、最終統合前に Phase 2 の行データだけを抽出して合成素材とする前処理を導入すること。
3. 変換品質の説明寄り度合いをスコア化する軽量ヒューリスティック（例: 禁止見出しの有無）をテストに追加すること。

## 実現方針 (Implementation Approach)

### 1. 根本原因への対処方針

| 原因 | 方針 |
| :--- | :--- |
| `IsSufficient` の部分文字列マッチ | `gap_judge.go` で否定パターン（`NOT SUFFICIENT`, `ではありません`, `不足` 等）を先に判定。既定は否定安全側。`--strict-gap-judge` は厳格行マッチを維持。 |
| Phase 2 スキップ | 誤充足修正により Phase 2 が最低 1 回は画像付き execute を実行。Phase 2 の question テンプレートは**原表の列名と一致する Markdown 表**を要求するよう固定化。 |
| 最終統合が解析表を丸ごと出力 | `prompts.go` の `GenerateMarkdownPrompt` / `GenerateMarkdownRetryPrompt` に禁止セクション・必須セクション構造を追加。 |
| Phase 1/3/4 の中間成果が説明的 | 中間フェーズの question 生成は現状維持可。ただし最終統合で「中間表の転載禁止・原表へのマージのみ」と明示。 |

### 2. 最終成果物の推奨構造

```markdown
# 変更履歴

| No. | 変更箇所 | 変更内容(変更理由) | ... |
| ... 原表（全列・全行） ... |

## 入れ子構造の展開

### SUB_TABLE_P01
- （セル原文のリスト再掲のみ）

## （任意・最小）書式メモ
- 画像から読み取った記号の原文列挙のみ。意味解釈文は禁止。
```

- `## 要素一覧（Phase N）` 型の見出しは禁止。
- Mermaid / 図解説明は `diagram` / `mixed` 分類時のみ許可（`complex_table` 単独画像では不要）。

### 3. 変更対象ファイル

1. `internal/imagetomd/analyzer/gap_judge.go`
   - 否定安全な `IsSufficient` 実装。
   - 単体テストで `SUFFICIENT ではありません` が false になることを固定。

2. `internal/imagetomd/analyzer/prompts.go`
   - `GenerateMarkdownPrompt`: 原表中心・禁止セクション・SUB_TABLE 展開の用途限定を追記。
   - `GenerateMarkdownRetryPrompt`: 同上の禁止事項を同期。
   - （必要なら）Phase 2 向け execute question の開始行指定を原表列名に固定。

3. `internal/imagetomd/analyzer/analyzer.go`
   - Phase 2 が answer 空のまま充足終了しないガード（ソフトチェック + verbose ログ）。
   - 最終合成後の品質チェックに「禁止見出しパターン」検出を追加し、検出時は再試行 1 回。

4. `tests/image_to_markdown_reference_parity_test.go` / `tests/testdata/reference_parity/`
   - `01_変更履歴.md` の期待データを「原表中心」に更新。
   - 禁止トークン（`要素一覧（Phase`, `意味対応`, `図解概要` 等）の非含有アサーションを追加。

5. `cmd/image-to-markdown/main.go` / `entext.go`
   - 挙動変更の契約維持確認のみ（ロジック分岐は analyzer 側に集約）。

### 4. 007 との関係

- `007-ImageToMarkdown-ReferenceParity` の成果（`client/v1` 移動、`OnText` 優先、inproc 設定、`model_profiles` スキーマ更新）は維持する。
- 本仕様は 007 の「参照実装との制御フロー一致」の上に、**出力情報設計の追加制約**を載せる。
- SUB_TABLE プロンプト自体は参照実装と同一のため、削除ではなく**最終統合での使い方を限定**する。

## 検証シナリオ (Verification Scenarios)

1. **誤充足の再現と修正確認（Phase 2）**
   1. 修正前実装（または現行）で `01_変更履歴.png` を変換し、セッションログで Phase 2 の `answer` が空のまま `sufficient: true` になることを確認する。
   2. 修正後に同画像を再変換する。
   3. Phase 2 で少なくとも 1 件の非空 `answer` が記録されることを確認する。
   4. gap 判定文が `SUFFICIENT ではありません` のとき、ラウンドが継続することを確認する。

2. **原表中心出力の確認（01_変更履歴）**
   1. `go run ./cmd/image-to-markdown -i tmp/output/pc/images/01_変更履歴.png -o tmp/output/pc/md/01_変更履歴.md --tern-mode inproc --tern-config settings/tern/tern-config.yaml --agent codex --model gpt-5.3-codex` を実行する。
   2. 出力先頭付近に 9 列の変更履歴 Markdown テーブルがあることを確認する。
   3. No.43 / No.44 の作成者・日付・変更内容の主要文字列が原表セルに含まれることを確認する。
   4. `## 要素一覧（Phase` / `書式・注記・セル結合・突合` / `意味対応・解釈` が**含まれない**ことを確認する。
   5. `28X-REQ-A0220` と `28P-ITb1-0243` が本文（原表または SUB 展開）に含まれることを確認する。

3. **SUB_TABLE 展開の維持確認**
   1. 上記出力に `SUB_TABLE` または `入れ子構造の展開` 相当のセクションがあることを確認する。
   2. 展開セクションに修正前後の URL 条件の原文が含まれることを確認する。
   3. 展開セクションが「要素ID / 隣接遷移 / 意味対応」型の解析表だけで構成されていないことを確認する。

4. **回帰確認（02_書き換えルール）**
   1. `tmp/output/pc/images/02_書き換えルール.png`（または同等の固定入力）を変換する。
   2. 出力が空でないこと、メイン表の Markdown テーブルが含まれることを確認する。
   3. Phase 名付き解析セクションが混入していないことを確認する。

5. **CLI/API 後方互換**
   1. `--stdin`, `--output-mode`, `--verbose`, `--tern-mode` の既存オプションで実行し、exit code と stdout/stderr 契約が維持されることを確認する。
   2. `entext.ConvertImageToMarkdown` から同画像を変換し、CLI と同等の禁止セクション非含有を確認する。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   ```bash
   ./scripts/process/build.sh
   ```

2. gap 判定・プロンプト・analyzer 単体テスト:
   ```bash
   go test ./internal/imagetomd/analyzer/... -count=1
   ```

3. image-to-markdown CLI 契約テスト:
   ```bash
   go test ./cmd/image-to-markdown/... -count=1
   ```

4. 参照パリティ・原表忠実度の統合テスト:
   ```bash
   ./scripts/process/integration_test.sh --specify "ReferenceParity|ChangeHistory|TableFaithful|NoPhaseReport"
   ```

### 要件対応表

| 要件 | 検証 |
| :--- | :--- |
| 1-4, 8（原表中心・SUB_TABLE 維持・禁止セクション） | `go test ./internal/imagetomd/analyzer/...` + `integration_test.sh --specify "ReferenceParity|TableFaithful"` |
| 5-6（IsSufficient 修正・Phase 2 実行保証） | `go test ./internal/imagetomd/analyzer/... -run GapJudge|Phase2` |
| 7（中間表の最終転載禁止） | `integration_test.sh --specify "NoPhaseReport|TableFaithful"` |
| 9-10（007 契約維持・API 同等品質） | `go test ./cmd/image-to-markdown/... ./tests/... -run "RootAPI|ImageToMarkdown"` |

## 実装結果メモ

- **判定**: ✅ 完了（2026-07-11）
- **変更概要**:
  - `gap_judge.go`: compat モードで否定パターン（`NOT SUFFICIENT`, `SUFFICIENT ではありません` 等）を先に除外
  - `quality.go`: `looksLikeExplanatoryReport` / `needsFinalSynthesisRetry` で説明寄り出力を検出し再合成をトリガー
  - `prompts.go`: 最終統合・再試行プロンプトに原表中心制約、`Phase2ExecuteHint()` 追加
  - `analyzer.go`: Phase 2 ソフト終了ガード、最終合成リトライを `needsFinalSynthesisRetry` に統合
  - `tests/testdata/reference_parity/01_変更履歴.md`: 実セル値入り原表中心ゴールデンへ更新
  - `tests/image_to_markdown_table_faithful_test.go`: 原表忠実度・禁止セクション契約テスト追加
- **検証結果**:
  - `./scripts/process/build.sh` → PASS
  - `./scripts/process/integration_test.sh --specify "TableFaithful|ReferenceParity|NoPhaseReport|ImageToMarkdown|RootAPI"` → PASS（スキップ 0 / 失敗 0）
- **未実施**: live LLM E2E（計画どおりゴールデン契約テストで代替）
