# 003 Go Native Excel PDF Image with GoOLE GoFitz

## 背景 (Background)

- 既存の Python 実装では、Excel COM（`pywin32`）と PDF ライブラリ（`PyMuPDF`）を組み合わせて、Excel -> PDF -> Image を高精度に実現している。
- 現行の `entext` は外部コマンド呼び出し（`libreoffice`, `pdftoppm`, `magick`）中心の設計であり、環境差分の影響を受けやすい。
- ユーザー要望として、「簡単かどうかは問わず、Go 系ライブラリを主手法として実装する」方針が明示された。
- そのため、Go ネイティブ寄りの実装（`go-ole` と `go-fitz` 主軸）へ移行するための要求仕様を定義する。

## 要件 (Requirements)

### 必須要件

1. Excel -> PDF 変換の主手法は `go-ole` を用いた Excel COM 制御とすること（Windows）。
2. PDF 操作（結合）および PDF -> Image 変換の主手法は `go-fitz` を用いること。
3. 外部コマンド（`libreoffice`, `pdftoppm`, `magick`）は主手法にしないこと。
   - 既存互換のために残す場合でも、明示的な非主系（fallback/legacy）として分離すること。
4. Python 実装の sidecar 方式を Go で再現すること。
   - `<pdf_basename>.sheet-map.json` を出力すること。
   - `page_sheet_names` と `sheet_entries` を保持すること。
5. シート単位の出力追跡を実装すること。
   - シートごとの `sheet_index`, `sheet_name`, `export_status`, `page_count`, `error`（必要時）を記録すること。
6. Excel の対象シート選択をサポートすること。
   - `--sheets "1,3,5"` の 1-based index 指定を受け付けること。
7. PDF -> Image の DPI とフォーマットを指定可能にすること。
   - `--dpi`（整数）
   - `--format`（`png|jpeg`）
8. 画像ファイル名にページ由来のシート名を反映すること。
   - 命名規則: `<page_index_2digit>_<sanitized_sheet_name>.<ext>`
   - sanitize 規則は Python 実装と同等（禁止文字除去、空白->`_`、連続`_`圧縮、前後`_`除去）とすること。
9. 既存 CLI の標準 I/O 契約（stdout は成果物、stderr はログ）および exit code 契約（0/1/2）を維持すること。
10. 公開 Go API からも同機能を利用可能にすること（CLI 専用実装にしない）。

### 任意要件

1. 非Windows環境向けに、主手法非対応時の legacy backend 利用可否を設定化する。
2. sidecar の schema versioning（`version` フィールド）を将来拡張しやすい構造にする。

## 実現方針 (Implementation Approach)

### 1. Excel COM 層（go-ole）

- `internal/exceltopdf` に COM 専用 backend を実装する。
- 処理フロー:
  1. Excel Application を起動
  2. Workbook を開く
  3. 対象シートごとに `PageSetup` を適用
  4. シート単位で一時 PDF 出力
  5. 結果を `sheet_entries` と `page_sheet_names` に集約
  6. COM オブジェクトを確実に解放

### 2. PDF 操作/画像化層（go-fitz）

- 一時 PDF 群を `go-fitz` で結合し、最終 PDF を生成する。
- 最終 PDF を `go-fitz` でページ単位レンダリングし、`--dpi` と `--format` を反映して画像出力する。
- 命名時は sidecar の `page_sheet_names` を参照し、sanitize を適用する。

### 3. sidecar 仕様

- 出力先: `<output_pdf_basename>.sheet-map.json`
- 代表フィールド:
  - `version`
  - `source_xlsx`
  - `pdf_path`
  - `page_sheet_names`
  - `sheet_entries[]`

### 4. CLI/API 仕様

- `excel-to-pdf` へ以下を追加:
  - `--sheets`
  - （必要なら）`--engine go-native|legacy` のような実行モード
- `pdf-to-image` へ以下を追加/整理:
  - `--dpi`
  - `--format png|jpeg`
  - `--sheet-map`（明示指定）または自動検出
- 公開 API には sidecar を返却できる Artifact 型を追加する。

## 検証シナリオ (Verification Scenarios)

1. 全シート変換（go-native）
   1. `excel-to-pdf -i samples/R06_09.xlsx -o tmp/entext-test/pdf --engine go-native`
   2. PDF と `*.sheet-map.json` が生成される。
   3. sidecar の `page_sheet_names` 件数が PDF ページ数と一致する。

2. シート限定変換（`--sheets`）
   1. `excel-to-pdf -i samples/R06_09.xlsx -o tmp/entext-test/pdf --engine go-native --sheets "1,3,5"`
   2. sidecar の `sheet_entries` が指定インデックスのみを含む。
   3. PDF は指定シート由来ページのみで構成される。

3. PDF -> Image（DPI/format 指定）
   1. `pdf-to-image -i tmp/entext-test/pdf/R06_09.pdf -o tmp/entext-test/images --dpi 300 --format png --engine go-native`
   2. 画像解像度が 200 DPI 時より高くなる。
   3. ファイル名が `01_<sheet>.png` 形式になる。

4. sidecar 連携
   1. `excel-to-pdf` で生成した sidecar を `pdf-to-image` が参照する。
   2. `page_sheet_names` と出力画像名の対応が崩れない。

5. エラー処理
   1. 不正 `--sheets`（例: `a,b`）で実行する。
   2. 引数エラーとして終了コード `2` で失敗する。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `scripts/process/build.sh`

2. 共通統合テスト（CLI契約/sidecar/命名）:
   - `scripts/process/integration_test.sh --categories "common" --specify "excel_to_pdf_go_native|pdf_to_image_go_native|sheet_map_sidecar|sheet_name_sanitize"`

3. LLM統合テスト（上流変換後の downstream 確認）:
   - `scripts/process/integration_test.sh --categories "llm" --specify "pipeline_chain|image_to_markdown"`

### 要件対応表

- 要件1-3（主手法の Go 化）:
  - 単体: COM backend / go-fitz renderer の振る舞いテスト
  - 統合: `--engine go-native` 実行で外部CLI非依存動作を確認
- 要件4-5（sidecar とシート追跡）:
  - 単体: sidecar serialize/deserialize テスト
  - 統合: ページ数・sheet_entries の整合性確認
- 要件6-8（sheets, dpi/format, 命名）:
  - 単体: `--sheets` parser・sanitize 関数テスト
  - 統合: 実ファイル出力名・画像サイズ検証
- 要件9-10（CLI/API互換）:
  - 単体: API artifact と validation
  - 統合: stdout/stderr/exit code 契約検証

## 実装確定メモ (Current Contract Snapshot)

- 追加 CLI オプション:
  - `excel-to-pdf --engine legacy|go-native`
  - `excel-to-pdf --sheets "1,3,5"`
  - `pdf-to-image --engine legacy|go-native`
  - `pdf-to-image --dpi <int>`
  - `pdf-to-image --sheet-map <path>`
- sidecar:
  - `excel-to-pdf` 実行時に `<pdf_basename>.sheet-map.json` を出力。
  - `pdf-to-image` は `--sheet-map` 指定があれば命名連携、未指定時は PDF と同名 sidecar を自動検出。
- 公開 API:
  - `ConvertExcelToPDFWithOptions(..., ExcelPDFOptions)`
  - `ConvertPDFToImageWithOptions(..., PDFImageOptions)`
  - `FileArtifact` は `SheetMapPath` を返却。
