# 011 ImageToMarkdown Excel CSV Hint

## 背景 (Background)

- `image-to-markdown` は画像（PNG 等）を Vision で読み取り、Markdown へ変換する。セル内の長文・記号・表記ゆれは、画像解像度や書式の影響を受け、転記精度が低下しうる。
- 一方、元データが Excel（`.xlsx`）である場合、**セルに入力されている文字列**はファイルから機械的に取り出せる。書式（色・罫線・結合・図形）は無視し、**テキストとして読める部分だけ**を先に CSV として用意しておけば、Coding Agent へのプロンプトに「ヒント」として渡せる。
- 現行パイプラインは `excel-to-pdf` → `pdf-to-image` → `image-to-markdown` で、PDF 横の `*.sheet-map.json` は **画像ファイル命名** にのみ使われ、`image-to-markdown` のプロンプトには接続されていない。
- 参照 Markdown（`--ref` / `RefPatterns`）は `.md` 全文を `[Reference markdown context]` として付与する仕組みがあるが、**元 Excel のセル値 CSV** を渡す経路は存在しない。
- `010-ImageToMarkdown-ConversionScopedGapJudgment` により、ギャップ判定は**画像からの転記忠実度**に限定され、内容校正は対象外である。CSV ヒントも同様に、**正解データの検証源**ではなく、Vision 読取を補助する参考情報として扱う必要がある。
- CSV には、画像に含まれる**レイアウト・結合セル・図表・色付き書式・入れ子構造の視覚表現**は反映されない。プロンプト上でこの限界を明示しないと、Agent が CSV を唯一の正として画像を無視したり、存在しない構造を CSV から推測したりするリスクがある。

## 要件 (Requirements)

### 必須要件

1. **Excel → CSV 変換**を新設すること。入力は `.xlsx`（将来 `.xls` は任意）。出力は UTF-8 の CSV ファイルとする。
2. CSV 変換は**形式的な書式を無視**し、**テキストとして読めるセル値**の抽出に限定すること。次は変換対象外または平坦化のみとする。
   - セル背景色・文字色・罫線スタイル
   - セル結合のレイアウト（結合範囲は展開して各セルに同一値を置く、または代表セルのみに値を置く等、実装で一貫した規則を定める）
   - 図形・チャート・画像オブジェクト
   - 数式の**表示値**は抽出してよいが、数式文字列そのものは原則不要（表示値が空の場合は空欄）
3. `excel-to-pdf` と同様に、**CLI サブコマンド**と**公開 Go API** の両方を提供すること。
   - CLI 例: `bin/entext/excel-to-csv -i ./book.xlsx -o ./tmp/csv/`
   - シート指定: `--sheets "1,3,5"` または全シート（`excel-to-pdf` と同じ意味）
   - 1 シート 1 CSV ファイル（シート名またはインデックスをファイル名に反映）
4. `image-to-markdown` が、**任意の CSV ヒント**をプロンプトに含められること。
   - 明示指定: `--csv-hint <path>`（複数可、または 1 パスに限定 — 実装計画で決定）
   - 自動解決（推奨）: 入力画像と**同一 basename** の `<basename>.csv` が画像と同じディレクトリまたは隣接 `csv/` ディレクトリに存在する場合、自動読込
5. CSV ヒントは **Vision 実行プロンプト**（分類・Phase 各ラウンドのデータ取得・最終統合）に付与すること。現行の `--ref` Markdown と同様、**ギャップ判定（AssessGapPrompt）には付与しない**（ヒントが充足判定を歪めないため）。
6. プロンプトには、次の**注意書き（disclaimer）**を必ず含めること。
   - CSV は元 Excel から抽出した**参考ヒント**であり、**最終出力の正は画像**である
   - CSV には**図表・色・結合レイアウト・入れ子の視覚構造**は含まれない
   - CSV と画像が矛盾する場合は**画像を優先**し、CSV は文字列の読取補助にのみ使う
   - CSV の内容について**校正・整合性評価・妥当性検証**を行わない（`010` の変換スコープと整合）
7. `010` のギャップ判定・Phase 2 スコープと矛盾しないこと。CSV ヒントの有無を理由に、画像上の未取得データの読取要求を省略してはならない。
8. CLI `image-to-markdown` と公開 API `ConvertImageToMarkdown` で**同一のヒント注入ロジック**を使用すること。
9. 既存の `--ref`（Markdown 参照）との役割分担を維持すること。CSV ヒントは `--ref` とは独立した経路とし、混同しないラベル（例: `[Reference csv hint]`）で付与する。

### 任意要件

1. `*.sheet-map.json` と連携し、PDF/画像 basename から**対応シートの CSV** を自動選択する。
2. `excel-to-pdf` 実行時に **オプションで CSV も同時出力**（`--with-csv`）し、sidecar 命名規則を統一する。
3. CSV ヒントの最大文字数制限（超過時は truncate + 注記）を設け、プロンプト溢れを防ぐ。
4. パイプライン用 wrapper script（`excel → pdf → image + csv → md`）のサンプルを `scripts/` に追加する。

## 実現方針 (Implementation Approach)

### 1. パイプライン上の位置づけ

```text
[Excel .xlsx]
    ├─ excel-to-csv ──► hints/*.csv          ─┐
    ├─ excel-to-pdf ──► *.pdf + sheet-map    │
    └─ pdf-to-image ──► images/*.png         │
                                              ▼
                              image-to-markdown (--csv-hint / auto)
                                              ▼
                                        output/*.md
```

CSV は **画像と並行の sidecar データ** とし、PDF 変換の必須前提にはしない。

### 2. 新規コンポーネント: `excel-to-csv`

| 項目 | 方針 |
| :--- | :--- |
| パッケージ | `internal/exceltocsv/`（新規） |
| CLI | `cmd/excel-to-csv/main.go`（新規） |
| 公開 API | `entext.go` に `ConvertExcelToCSV(job, opts)` を追加 |
| バックエンド | `excel-to-pdf` と同型の `--backend auto\|libreoffice\|excel-com` を踏襲。Windows では `excel-com` 優先、非 Windows では `libreoffice` |
| エンジン | 初版は **legacy バックエンド**（COM / LibreOffice の CSV エクスポート）を優先。go-native は follow-up |
| 出力命名 | `<workbook_basename>_<sanitized_sheet_name>.csv` または `<workbook_basename>.sheet-<index>.csv`（sheet-map と対応可能な規則） |
| 文字コード | UTF-8（BOM 有無は実装計画で決定。日本語 Excel との互換を優先） |

**平坦化ルール（例）:**
- 結合セル: 左上セルの表示値を結合範囲全体に複写、または CSV 上は 1 セルのみ非空（どちらか一方に統一しテストで固定）
- 空セル: 空フィールド
- 改行を含むセル: CSV エスケープ（引用符）でそのまま保持

### 3. ヒント注入: `internal/imagetomd` 拡張

#### 3.1 ヒント読込

```go
type CsvHint struct {
    Path    string
    Content string
}

func ResolveCsvHints(explicitPaths []string, imagePath string) ([]CsvHint, error)
```

- `explicitPaths`: CLI `--csv-hint` / API `CsvHintPaths`
- 自動解決: `filepath.Dir(imagePath)` 内の `<imageBasename>.csv`、および `<imageDir>/../csv/<imageBasename>.csv` 等、実装計画で列挙

#### 3.2 プロンプト合成

`analyzer.buildRefContext` と並列に `buildCsvHintContext` を追加:

```text
[Reference csv hint]
Source: C:/path/to/01_変更履歴.csv

IMPORTANT (read before using this hint):
- This CSV is supplementary text extracted from the source Excel workbook.
- It does NOT include diagrams, charts, cell colors, merged-cell layout, or nested visual structure.
- The attached image is authoritative. If CSV and image disagree, follow the image.
- Use CSV only to assist reading ambiguous cell text. Do not proofread or validate content against CSV.

--- CSV content ---
<csv body>
```

付与タイミング（`refContext` と同じ）:
- 分類プロンプト
- Phase 各ラウンドの **execute** プロンプト（`ExecutionQuestionSuffix` 前後）
- 最終 Markdown 統合プロンプト

付与しない:
- `AssessGapPrompt`
- `GenerateQuestionPrompt`（不足理由は画像ベースのまま）

#### 3.3 公開 API / CLI 拡張

```go
type ImageToMarkdownJob struct {
    InputPath    string
    OutputPath   string
    OutputDir    string
    RefPatterns  []string
    CsvHintPaths []string // 新規。空なら自動解決を試行
}
```

CLI: `--csv-hint path`（repeatable）、`--no-csv-hint-auto`（自動解決オフ）

### 4. `010` との整合

| 観点 | CSV ヒント |
| :--- | :--- |
| 転記の正 | 画像が正。CSV は補助 |
| ギャップ判定 | CSV との文字一致を充足条件にしない |
| 内容校正 | CSV を「正しい版」として統一要求しない |
| Phase 2 | 画像上の行・列・空欄・入れ子は引き続き Vision で確認 |

### 5. 非目標（本仕様のスコープ外）

- CSV を唯一入力として Markdown を生成するモード（画像なし変換）
- Excel 書式・図表の CSV への完全再現
- CSV ヒントと画像の自動 diff 校正レポート
- 010 で禁止した `SymbolEvidence` 列の CSV 版生成

## 検証シナリオ (Verification Scenarios)

1. **Excel → CSV 単体変換**
   1. テスト用 `.xlsx`（複数シート、結合セル、日本語・記号を含むセル）を用意する。
   2. `bin/entext/excel-to-csv -i book.xlsx -o ./tmp/csv/` を実行する。
   3. 指定シート数と同数の CSV が出力される。
   4. 既知セルの文字列（例: `秋葉達也`, `2025/7/7`, URL 文字列）が CSV に含まれる。
   5. 図形・チャートは CSV に現れない。

2. **CSV ヒント付き image-to-markdown（モック）**
   1. 単体テストで `ResolveCsvHints` が basename 自動解決できることを確認する。
   2. analyzer モックで、execute プロンプトに `[Reference csv hint]` と disclaimer が含まれることを確認する。
   3. `AssessGapPrompt` 入力に CSV ヒントが**含まれない**ことを確認する。

3. **010 スコープとの非矛盾**
   1. CSV ヒント付きでも、Phase 2 ギャップ判定プロンプトに変換境界（校正禁止）が維持される。
   2. CSV にあって画像にない情報を「不足」としないテスト（モック assess が INSUFFICIENT 理由に CSV 整合を使わない）。

4. **パイプライン手動確認（任意）**
   1. `tmp/input` の Excel から CSV を生成する。
   2. 対応 PNG に `--csv-hint` を付けて `image-to-markdown` を実行する。
   3. セッションログで Phase 2 が CSV 整合のみで `hard_limit` にならないことを目視確認する。

5. **回帰**
   1. `--csv-hint` / ヒント未指定時、既存 `TableFaithful|ReferenceParity` 契約が変わらない。

## テスト項目 (Testing for the Requirements)

### 単体テスト

1. `excel-to-csv`:
   - 既知 xlsx fixture から期待セル文字列が CSV に含まれる
   - `--sheets` 指定で出力ファイル数が一致
   - 結合セル平坦化規則が固定されている

2. `ResolveCsvHints`:
   - 明示パス指定
   - `<imageBasename>.csv` 自動解決
   - ファイル不存在時は空（エラーにしない）

3. `buildCsvHintContext`:
   - disclaimer 必須フレーズ含有
   - CSV 本文が含まれる

4. analyzer プロンプト契約:
   - classify / execute / final synthesis に hint 付与
   - assess / generate_question に hint 非付与

### 統合テスト

1. 公開 API:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "ExcelToCsv|CsvHint|ImageToMarkdown|RootAPI"
   ```

2. 008/010 回帰:
   ```bash
   ./scripts/process/build.sh && ./scripts/process/integration_test.sh --specify "TableFaithful|ReferenceParity|ConversionScope|NoPhaseReport"
   ```

### ビルド・全体検証

```bash
./scripts/process/build.sh
```

### 要件対応表

| 要件 | 自動検証 |
| :--- | :--- |
| 1-3 Excel→CSV CLI/API | `ExcelToCsv` 統合テスト + 単体 |
| 4-5 ヒント読込・付与タイミング | `CsvHint` / analyzer 契約テスト |
| 6 disclaimer | `buildCsvHintContext` 単体 |
| 7 010 非矛盾 | analyzer モック + ConversionScope 回帰 |
| 8 CLI/API 同一 | `ImageToMarkdown\|RootAPI` |
| 9 `--ref` 分離 | プロンプトラベル契約テスト |

## 実装計画への引き渡しメモ

- 新規 Go パッケージ `internal/exceltocsv/` と CLI `cmd/excel-to-csv/` が追加される。
- `ImageToMarkdownJob` / CLI フラグ拡張、`analyzer.Analyze` の引数または options 拡張が必要。
- 010 完了後の follow-up として実装する場合、010 のプロンプト境界を上書きしないよう disclaimer を共通化すること。
