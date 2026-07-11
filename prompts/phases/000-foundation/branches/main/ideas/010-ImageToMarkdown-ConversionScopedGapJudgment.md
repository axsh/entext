# 010 ImageToMarkdown Conversion-Scoped Gap Judgment

## 背景 (Background)

- `image-to-markdown` の目的は、画像に記載された情報を Markdown へ忠実に変換することである。
- `008-ImageToMarkdown-TableFaithfulOutput` の対応後、`tmp/output/pc/images/01_変更履歴.png` は、9 列の原表、No.43/44、空欄行、入れ子セルの内容を `tmp/output/pc/md/01_変更履歴.md` に以前より正確に再現できるようになった。この改善は維持する必要がある。
- 一方、`tmp/output/pc/md/_sessions/01_変更履歴_session.json` の Phase 2 では、画像の主要な表データをすでに読み取った後も、複数ラウンドの転記結果に含まれる `[詳細設計]` / `【詳細設計】`、`〇` / `○`、`＜＞` / `〈〉`、`-` / `−` などの差異を理由に `NOT SUFFICIENT` と判定し続けた。
- さらに、記号の確定根拠、複数転記結果間の統一、行・列番号付きの「最終確定版」などを追加要求したため、Phase 2 は 5 ラウンドを使い切って `hard_limit` 終了した。
- 画像に対する転記の正確性は重要である。しかし、画像内に元から存在する記載内容の意味的整合性、表記ゆれ、URL・正規表現の妥当性、文章としての正しさを校正することは、画像変換とは別の処理要求である。
- 現状は、次の二つの「正確性」がギャップ判定内で混同されている。
  1. **変換正確性**: 画像に見える文字、行、列、空欄、構造を Markdown に忠実に写すこと。
  2. **内容正確性**: 元の記載内容が相互に整合しているか、業務的・文法的・技術的に正しいかを検証・校正すること。
- 本仕様では、`image-to-markdown` のギャップ判定を変換正確性の範囲に限定し、内容正確性の検証を明示的にスコープ外とする。
- 加えて、現行の `AssessGapPrompt` は「充足時に `SUFFICIENT` を含める」ことだけを規定しており、不充足時の出力形式は未定義である。その結果、セッションログでは `不足しています`、`不十分です`、`NOT SUFFICIENT`、`SUFFICIENT ではありません` など表記揺れが発生し、Go 側の `IsSufficient` が否定パターンの列挙と部分文字列マッチで後追い対応している。判定の二値（充足 / 不充足）を LLM 出力とパーサーの双方で固定する必要がある。

## 要件 (Requirements)

### 必須要件

1. `image-to-markdown` の Phase 2 ギャップ判定は、**画像から Markdown への変換に必要な情報が揃ったか**だけを判定すること。
2. Phase 2 の充足条件は、対象画像の種類に応じて次を満たすこと。
   - テーブル:
     - 見えている列構成と見出しを取得済み
     - 見えているデータ行と各セル値を取得済み
     - 空欄セル・空欄行を含む、Markdown で再現すべき表構造を取得済み
     - 入れ子リストや複合セルがある場合、その原文データを取得済み
   - 図解:
     - 見えているテキスト、ラベル、ノード、接続関係を Markdown または Mermaid で再現できる情報を取得済み
3. 次の事項を、Phase 2 の不足理由にしてはならない。
   - 元画像内の表記ゆれ、文章上の矛盾、業務上の不整合
   - URL、正規表現、識別子、日付、氏名などの意味的・技術的な妥当性
   - 元画像に記載された内容の校正、修正、正規化
   - 複数の中間転記結果を一つの「正しい内容」に統一すること自体
   - `SymbolEvidence`、校正根拠、内容整合性注記など、最終 Markdown に不要な証明情報
4. 画像との照合により実際の転記誤りが疑われる場合は再読取してよい。ただし、不足理由は「画像の特定セルが未読」「画像と転記が一致しない可能性がある」など、**画像と出力候補の対応関係**として表現すること。
5. 複数ラウンドの回答間に字形差があっても、それだけで不足と判定してはならない。画像から判読できる最も忠実な候補を採用し、変換に必要な全体構造と主要データが揃っていれば `SUFFICIENT` とすること。
6. 判読不能または曖昧な箇所が残る場合は、無期限に校正を繰り返さず、次のいずれかで変換結果に保持すること。
   - 判読可能な範囲をそのまま転記する
   - 最小限の判読不能マーカーを付ける
   - 代替候補を最小限の注記として残す
7. `AssessGapPrompt` は、次の境界を明示すること。
   - 判定対象は「画像情報を Markdown で再現できるか」
   - 内容の正しさ、整合性、妥当性、校正は判定対象外
   - 元画像の表記は、誤りや不整合に見えても原則そのまま保持
8. Phase 2 の Goal は、「読み取りの正確性」が**画像との転記忠実度**を意味し、内容校正を意味しないことを明文化すること。
9. Phase 3 / Phase 4 の分析も変換目的に従属させること。Markdown で構造や視覚表現を再現するために必要な分析は許可するが、元データの意味的整合性の評価や修正提案へ拡張してはならない。
10. `tmp/output/pc/images/01_変更履歴.png` に対する現在の改善結果を維持すること。少なくとも次を最終 Markdown に含めること。
    - 9 列の変更履歴表
    - No.43 / No.44 のデータ
    - `秋葉達也`、`藤本華子`、`2025/7/7`、`2025/12/9`
    - URL・正規表現を含む変更内容
    - 画像下部の空欄行
    - 入れ子内容の展開
11. 最終 Markdown に、元画像には存在しない校正表、内容整合性評価、`SymbolEvidence` 列、転記比較レポートを出力しないこと。
12. CLI `image-to-markdown` と公開 API `ConvertImageToMarkdown` で同じギャップ判定を使用すること。
13. 全 Phase のギャップ判定出力は、**充足 / 不充足の二値**とし、次のいずれか一方のみを判定語として用いること。
    - 充足: `SUFFICIENT`
    - 不充足: `INSUFFICIENT`
    不足理由や補足説明は、この判定語の前後に記述してよいが、`SUFFICIENT` と `INSUFFICIENT` を同一回答内で混在させてはならない。
14. `AssessGapPrompt` は、上記二値出力を義務付けること。現行の「充足時に `SUFFICIENT` を含める」だけの規定を置き換え、不充足時は必ず `INSUFFICIENT` を出力するよう明示すること。`不足しています`、`不十分`、`NOT SUFFICIENT`、`SUFFICIENT ではありません` などの代替表現は禁止する。
15. `IsSufficient`（および将来の gap 判定パーサー）は、文字列マッチングを次の順序で行うこと。
    1. **`INSUFFICIENT` を先に判定**する（`INSUFFICIENT` は `SUFFICIENT` を部分文字列として含むため）。
    2. 次に **`SUFFICIENT` を判定**する。
    3. どちらも検出できない場合は **不充足（`false`）としてフォールバック**し、追加読取ラウンドを継続する。判定不能を充足と誤認してはならない。
    - `compat` モード: 大文字小文字を無視した部分一致でよいが、判定順序 1→2→3 は厳守する。
    - `strict` モード: `^SUFFICIENT$` / `^INSUFFICIENT$`、または `Decision: SUFFICIENT` / `Decision: INSUFFICIENT` の行単位一致のみを真とする。
    - 移行期間中、`NOT SUFFICIENT`、`NOT_SUFFICIENT`、`SUFFICIENT ではありません` 等のレガシー否定表現は `INSUFFICIENT` と同等に不充足として扱ってよい。
16. セッションログの `sufficient` フィールドは、上記パーサー結果と一致すること。`gap_assessment` に `INSUFFICIENT` が含まれるのに `sufficient: true` となる誤判定を再発させないこと。

### 任意要件

1. Phase 2 の最新回答が原表全体を含む場合、過去回答を累積した `known_info` ではなく、最新の原表候補を優先してギャップ判定へ渡すこと。
2. テーブル向けに、列見出し・データ行・空欄行・入れ子内容の取得有無を軽量に確認する決定的チェックを追加すること。
3. Phase 2 が同じ表を繰り返し出力している場合、内容校正を追加要求せず早期終了すること。
4. 内容校正が必要な場合は、将来の独立したコマンドまたは明示オプションとして設計し、通常の画像変換から分離すること。

## 実現方針 (Implementation Approach)

### 1. 変換正確性と内容正確性の境界

| 観点 | `image-to-markdown` で扱う | `image-to-markdown` では扱わない |
| :--- | :--- | :--- |
| 文字 | 画像に見える文字を転記 | 誤字・表記ゆれの修正 |
| 表構造 | 行、列、空欄、結合、入れ子を再現 | 表の業務ルール上の整合性評価 |
| URL・正規表現 | 見える文字列をそのまま保持 | 構文・動作・意味の検証 |
| 日付・氏名・ID | 見える値をそのまま保持 | 値の妥当性・相互整合性の検証 |
| 曖昧な字形 | 画像との再照合、最小限の不明表示 | 複数案の校正レポート作成 |
| ギャップ判定 | Markdown 再現に必要な情報の有無 | 元データの内容品質の判定 |

### 2. Phase 2 Goal の限定

`internal/imagetomd/analyzer/prompts.go` の Phase 2 Goal を、次の趣旨へ変更する。

```text
テーブルの場合は、画像に見える全行・全列・セル値・空欄・入れ子内容を、
内容の校正や意味的整合性の評価をせず、画像どおりに Markdown へ再現できる情報として記録する。
```

### 3. ギャップ判定プロンプトの境界追加と二値出力

`AssessGapPrompt` に変換境界と二値出力形式を追加する。現行の「充足時に `SUFFICIENT` を含める」規定は削除する。

```text
判定結果は、必ず次のいずれか一方を回答に含めてください（混在禁止）:
- 充足: SUFFICIENT
- 不充足: INSUFFICIENT

不足理由や補足説明は、この判定語の前後に簡潔に記述してよいです。
「不足しています」「不十分」「NOT SUFFICIENT」等の代替表現は使わないでください。

この判定は画像から Markdown への変換に必要な情報の充足だけを対象とします。
元画像の記載内容について、誤字、表記ゆれ、意味的矛盾、URL・正規表現の妥当性を
校正または評価してはなりません。画像に記載されているなら、そのまま転記できていることを
充足条件としてください。
```

### 4. Phase 2 専用充足ガイド

- Phase 2 では、一般的な「不足情報」探索ではなく、変換可能性の確認項目を提示する。
- 既存回答間の差異ではなく、元画像に対して未取得の行・列・セル・入れ子情報があるかを問う。
- すでに原表全体が一つ以上の回答に含まれている場合は、追加校正を求めず `SUFFICIENT` を返せるようにする。

### 5. ギャップ判定パーサーの二値化

`internal/imagetomd/analyzer/gap_judge.go` の `IsSufficient` を、否定パターン列挙＋`SUFFICIENT` 部分一致依存から、明示的な二値判定へ変更する。

判定順序（`compat` / `strict` 共通の原則）:

```
1. INSUFFICIENT を検出 → false（不充足）
2. SUFFICIENT を検出   → true（充足）
3. どちらも未検出     → false（不充足フォールバック、ラウンド継続）
```

`compat` モードの検出例:

- `INSUFFICIENT` → false（`SUFFICIENT` 部分一致より先に評価）
- `判定: INSUFFICIENT\n不足: 列見出し` → false
- `SUFFICIENT` → true
- `判定: SUFFICIENT\n補足あり` → true
- `不足しています`（レガシー、移行期のみ）→ レガシー否定パターンで false
- 判定語なし → false（フォールバック）

`strict` モードの検出例:

- 行単位 `SUFFICIENT` / `INSUFFICIENT`、または `Decision: SUFFICIENT` / `Decision: INSUFFICIENT` のみ真
- `NOT SUFFICIENT` は strict では不充足（行完全一致しないため）

### 6. 現行品質の維持

- `008-ImageToMarkdown-TableFaithfulOutput` の原表中心制約、説明レポート検出、Phase 2 実行保証は維持する。
- 本仕様は読み取り精度を下げるものではない。
- 画像と転記の不一致、未読セル、欠落行は引き続き不足として扱う。
- 除外するのは、転記完了後に元データそのものを校正・検証する処理である。

## 検証シナリオ (Verification Scenarios)

1. **変更履歴画像の Phase 2 正常終了**
   1. ビルド済み `bin/entext/image-to-markdown.exe` を使用する。
   2. `tmp/output/pc/images/01_変更履歴.png` を `tmp/output/pc/md/01_変更履歴.md` に変換する。
   3. Phase 2 で列見出し、No.43/44、各セル値、空欄行、入れ子内容を取得する。
   4. 元画像の内容に校正上の疑問があっても、画像どおり転記できていれば `SUFFICIENT` と判定する。
   5. `SymbolEvidence`、内容整合性、URL・正規表現の妥当性を追加要求しない。
   6. Phase 2 が同一表の校正だけを理由に `hard_limit` へ到達しない。

2. **画像との転記不一致は引き続き不足**
   1. モック回答から明示的にデータ行または列見出しを欠落させる。
   2. ギャップ判定が、内容整合性ではなく「未取得の行・列・セル」を具体的に不足として返す。
   3. 不足箇所を取得後に `SUFFICIENT` となる。

3. **元データの不整合をそのまま保持**
   1. 同一画像内に表記ゆれ、矛盾して見える値、構文上疑わしい URL または正規表現があるケースを用意する。
   2. それらを画像どおり Markdown に転記する。
   3. ギャップ判定が内容修正・正規化・妥当性検証を要求しない。
   4. 最終 Markdown に校正コメントを追加しない。

4. **現在の原表忠実度を回帰させない**
   1. `tmp/output/pc/images/01_変更履歴.png` の固定ゴールデン契約を実行する。
   2. 9 列表、No.43/44、氏名、日付、変更内容、空欄行、入れ子展開が保持される。
   3. Phase レポート、校正表、内容整合性評価が含まれない。

5. **ギャップ判定の二値出力とパース**
   1. モック `gap_assessment` が `INSUFFICIENT` のみのとき `sufficient: false` となる。
   2. `INSUFFICIENT` を含む文字列が `SUFFICIENT` 部分一致で `sufficient: true` にならない。
   3. `SUFFICIENT` のみのとき `sufficient: true` となる。
   4. 判定語がない曖昧な回答は `sufficient: false`（フォールバック）となる。
   5. `AssessGapPrompt` に `SUFFICIENT` / `INSUFFICIENT` の二値義務と代替表現禁止が含まれる。

## テスト項目 (Testing for the Requirements)

### 単体テスト

1. `AssessGapPrompt` の変換境界と二値出力:
   - 「内容の校正・意味的整合性・妥当性は対象外」
   - 「画像どおり転記できれば充足」
   - 「未取得の行・列・セルは不足」
   - 「充足: SUFFICIENT / 不充足: INSUFFICIENT（混在禁止）」
   - 「不足しています」「NOT SUFFICIENT」等の代替表現禁止
   を示す文言が含まれること。

2. `IsSufficient` 二値パース（`gap_judge_test.go` / `quality_test.go`）:
   - `INSUFFICIENT` → false
   - `INSUFFICIENT\n補足` → false（`SUFFICIENT` 部分一致より優先）
   - `SUFFICIENT` → true
   - 判定語なし → false（フォールバック）
   - strict: `NOT SUFFICIENT` → false、`SUFFICIENT` 行 → true
   - compat 移行期: `NOT SUFFICIENT` / `SUFFICIENT ではありません` → false

3. Phase 2 Goal:
   - 画像との転記忠実度を要求すること。
   - 内容校正を禁止すること。

4. モックギャップ判定:
   - 原表が揃った回答に、元データ上の表記ゆれや疑わしい URL が含まれても追加校正を要求しないこと。
   - 行またはセルが欠落した回答は不足と判定すること。

5. 質問生成:
   - `SymbolEvidence`、内容整合性、URL 妥当性、正規表現検証を要求しないこと。
   - 不足時は画像上の未取得データのみを質問すること。

### 統合テスト

1. 既存の原表忠実度契約:
   ```bash
   ./scripts/process/integration_test.sh --specify "TableFaithful|ReferenceParity|NoPhaseReport"
   ```

2. CLI / 公開 API 回帰:
   ```bash
   ./scripts/process/integration_test.sh --specify "ImageToMarkdown|RootAPI"
   ```

3. Phase 2 スコープ契約:
   ```bash
   go test ./internal/imagetomd/analyzer/... -run "Phase2|GapPrompt|ConversionScope|GapJudge|IsSufficient" -count=1
   ```

### ビルド・全体検証

```bash
./scripts/process/build.sh
```

### 要件対応表

| 要件 | 自動検証 |
| :--- | :--- |
| 1-2 変換情報だけで充足判定 | Phase 2 モックテスト |
| 3 内容校正を不足理由にしない | `ConversionScope` プロンプト契約テスト |
| 4 画像との不一致は再読取 | 欠落行・欠落セルテスト |
| 5 中間回答間の差異だけで継続しない | 表記ゆれを含む複数回答テスト |
| 6 判読不能箇所の有限処理 | 曖昧文字ケースの質問生成テスト |
| 7-9 全 Phase の変換境界 | `GapPrompt` / Phase Goal テスト |
| 10 現行原表品質の維持 | `TableFaithful|ReferenceParity` |
| 11 校正レポート非出力 | 禁止トークン統合テスト |
| 12 CLI/API 同一挙動 | `ImageToMarkdown|RootAPI` |
| 13-14 二値出力（SUFFICIENT / INSUFFICIENT） | `AssessGapPrompt` 契約テスト |
| 15 パーサー判定順序とフォールバック | `GapJudge|IsSufficient` 単体テスト |
| 16 `sufficient` フィールド整合 | analyzer モック統合テスト |

## 非目標 (Non-Goals)

- 元画像内の文章校正
- 表記ゆれの統一
- URL や正規表現の構文・動作検証
- 日付、氏名、ID、版番号の業務的妥当性確認
- 元データ内の矛盾検出・修正提案
- 画像変換と同時に行う品質監査

これらが必要な場合は、`image-to-markdown` とは独立した明示的な処理要求として扱う。
