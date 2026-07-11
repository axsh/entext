# 011 ImageToMarkdown Excel CSV Hint

> **運用上のスコープ制限**: 012 `ImageToMarkdown-CsvVisibleScope-ContextEfficiency` を参照。012 実装後は CSV 全文の classify 付与は廃止し、出力境界は画像可視スコープ、CSV は Phase 2 execute round 1 のスコープフィルタ抜粋のみに限定される。

## 背景 (Background)

- `image-to-markdown` は画像（PNG 等）を Vision で読み取り、Markdown へ変換する。セル内の長文・記号・表記ゆれは、画像解像度や書式の影響を受け、転記精度が低下しうる。
- 一方、元データが Excel（`.xlsx`）である場合、**セルに入力されている文字列**はファイルから機械的に取り出せる。書式（色・罫線・結合・図形）は無視し、**テキストとして読める部分だけ**を先に CSV として用意しておけば、Coding Agent へのプロンプトに「ヒント」として渡せる。
- 現行パイプラインは `excel-to-pdf` → `pdf-to-image` → `image-to-markdown` で、PDF 横の `*.sheet-map.json` は **画像ファイル命名** にのみ使われ、`image-to-markdown` のプロンプトには接続されていない。
- 参照 Markdown（`--ref` / `RefPatterns`）は `.md` 全文を `[Reference markdown context]` として付与する仕組みがあるが、**元 Excel のセル値 CSV** を渡す経路は存在しない。
- `010-ImageToMarkdown-ConversionScopedGapJudgment` により、ギャップ判定は**画像からの転記忠実度**に限定され、内容校正は対象外である。CSV も**内容の正しさを検証する正解データ**ではないが、**セル文字列の参照源**として積極利用してよい。
- CSV には、画像に含まれる**レイアウト・結合セル・図表・色付き書式・入れ子構造の視覚表現**は反映されない。一方、画像に**大量の表データ**が含まれる場合、Vision だけで全セルを再読すると漏れ・誤読が起きやすい。プロンプト上で「構造は画像、大量セル値は CSV 参照可」と明示し、データ量に応じた使い分けを Agent に許容する必要がある。

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
6. プロンプトには、**画像と CSV の役割分担**を明示する利用ガイド（disclaimer + usage policy）を必ず含めること。
   - CSV は元 Excel から抽出した**セル値テキストの参照源**である（単なる補助に限定しない）
   - CSV には**図表・色・結合レイアウト・入れ子の視覚構造**は含まれない
   - **表構造**（列見出し、行数、空欄行、セクション帯、結合・入れ子の有無）は**画像の Vision 読取を正**とする
   - **セル文字列**について:
     - 少量・判読容易な場合: Vision 転記を優先する
     - 画像に**大量の行・列データ**が含まれる場合: **CSV の該当セル値を参照して転記してよい**（Vision で全セルを逐一再読する必要はない）
   - CSV と画像の文字列が異なる場合: **画像で判読できる範囲を優先**し、判読不能・曖昧なセルのみ CSV を参照する
   - CSV および画像の内容について**校正・意味整合性・URL/正規表現の妥当性検証**は行わない（`010` の変換スコープと整合）
7. `010` のギャップ判定・Phase 2 スコープと矛盾しないこと。
   - ギャップ判定（AssessGapPrompt）入力に CSV を含めない方針は維持する
   - ただし Phase 2 **execute** および最終統合では、表構造が画像で把握済みかつ CSV が付与されている場合、**セル値の網羅読取に CSV 参照を許容**する
   - CSV との文字一致や内容整合を Phase 2 の不足理由にしてはならない（`010` 維持）
   - CSV にあっても、**画像上の表構造・空欄行・入れ子の視覚表現**が未取得なら不足とする
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

#### 3.2 プロンプト合成（画像 + CSV 併用ポリシー）

`analyzer.buildRefContext` と並列に `buildCsvHintContext` を追加する。
単なる disclaimer ではなく、**いつ Vision を使い、いつ CSV を参照してよいか**を Agent に明示する。

```text
[Reference csv hint]
Source: C:/path/to/01_変更履歴.csv

【CSV ヒントの位置づけ】
- この CSV は元 Excel から抽出したセル値テキストである。
- CSV には図表・色・結合レイアウト・入れ子の視覚構造は含まれない。

【画像と CSV の使い分け（必ず守ること）】
1. 表の構造（列見出し、行数、空欄行、セクション帯、結合・入れ子の有無）は、添付画像の Vision 読取を正とする。
2. セル内の文字列データについて:
   - 画像から判読可能で量が少ない場合: Vision 転記を優先する。
   - 画像に大量の行・列データが含まれる場合: この CSV の該当セル値を参照して転記してよい。
     Vision で全セルを逐一再読する必要はない。
3. CSV と画像の文字列が異なる場合: 画像で判読できる範囲を優先する。判読不能・曖昧なセルのみ CSV を参照する。
4. CSV および画像の内容について、校正・意味整合性・URL/正規表現の妥当性の検証は行わない。

--- CSV content ---
<csv body>
```

**Phase 2 execute 向け追記（CSV 付与時）:**

```text
Phase 2 追加指示:
- 原表の列構成・空欄行・入れ子の有無は画像で確認すること。
- データ行のセル文字列は、行数が多い場合は上記 CSV から転記してよい。
- 最終回答は Markdown テーブル形式とし、CSV をそのまま貼り付けるのではなく、
  画像の表構造に合わせて配置すること。
```

**最終統合向け追記:**

```text
- Phase 2 で CSV 参照により取得したセル値は、画像の原表構造に従って Markdown テーブルへ配置すること。
- CSV にしか存在しない列・行を追加してはならない（画像の表構造を超えない）。
```

付与タイミング（`refContext` と同じ）:
- 分類プロンプト
- Phase 各ラウンドの **execute** プロンプト（Phase 2 では上記追記を含む）
- 最終 Markdown 統合プロンプト

付与しない:
- `AssessGapPrompt`（充足判定は画像ベースの構造・未取得情報のみ）
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

| 観点 | 方針 |
| :--- | :--- |
| 表構造の正 | 画像（列・行・空欄・入れ子の視覚表現） |
| セル文字列（大量） | CSV 参照可（Vision 逐一読取は不要） |
| セル文字列（少量・矛盾時） | 画像優先、曖昧時のみ CSV |
| ギャップ判定 | CSV を入力に含めない。CSV 整合は不足理由にしない |
| 内容校正 | CSV/画像いずれも校正・妥当性検証の対象外 |
| Phase 2 execute | 構造は Vision、セル値は CSV 併用可 |

```text
         ┌─────────────────────────────────────┐
         │  Vision（画像）                      │
         │  ・列見出し / 行数 / 空欄行           │
         │  ・結合・入れ子・セクション帯         │
         │  ・少量セル / 矛盾時の優先ソース       │
         └──────────────┬──────────────────────┘
                        │ 構造を決定
                        ▼
         ┌─────────────────────────────────────┐
         │  CSV（セル値テキスト）                │
         │  ・大量行・列の文字列転記に参照可      │
         │  ・構造・図表の決定には使わない        │
         └─────────────────────────────────────┘
```

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
   2. analyzer モックで、execute プロンプトに `[Reference csv hint]` と**使い分けポリシー**（大量データは CSV 参照可、構造は Vision）が含まれることを確認する。
   3. Phase 2 execute プロンプトに Phase 2 追加指示（CSV 転記可）が含まれることを確認する。
   4. `AssessGapPrompt` 入力に CSV ヒントが**含まれない**ことを確認する。

3. **010 スコープとの非矛盾**
   1. CSV ヒント付きでも、Phase 2 ギャップ判定プロンプトに変換境界（校正禁止）が維持される。
   2. モック assess が INSUFFICIENT 理由に「CSV と画像の文字不一致」「CSV 整合」等を使わない。
   3. モック Phase 2 execute が CSV 付与時にセル値を CSV から転記する指示を含む。

4. **大量データシート（任意・手動）**
   1. 行数の多いシートの PNG + 対応 CSV で `image-to-markdown` を実行する。
   2. Phase 2 の回答が CSV セル値を参照した Markdown テーブルになっていることをセッションログで確認する。
   3. 列構成・空欄行は画像どおり維持されていることを確認する。

5. **パイプライン手動確認（任意）**
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
   - 使い分けポリシー必須フレーズ含有（「大量の行・列データ」「CSV の該当セル値を参照して転記してよい」「表の構造は Vision」）
   - Phase 2 追加指示・最終統合追記の契約（実装計画で関数分割）
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
| 6 画像+CSV 使い分けポリシー | `buildCsvHintContext` / Phase2 追記契約 |
| 7 010 非矛盾（構造=Vision、値=CSV可） | analyzer モック + ConversionScope 回帰 |
| 8 CLI/API 同一 | `ImageToMarkdown\|RootAPI` |
| 9 `--ref` 分離 | プロンプトラベル契約テスト |

## 実装計画への引き渡しメモ

- 新規 Go パッケージ `internal/exceltocsv/` と CLI `cmd/excel-to-csv/` が追加される。
- `ImageToMarkdownJob` / CLI フラグ拡張、`analyzer.Analyze` の引数または options 拡張が必要。
- 010 完了後の follow-up として実装する場合、010 のプロンプト境界（校正禁止）を維持しつつ、Phase 2 execute に CSV 参照許可を追記する。
