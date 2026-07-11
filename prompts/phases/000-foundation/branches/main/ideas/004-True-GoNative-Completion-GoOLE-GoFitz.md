# 004 True GoNative Completion GoOLE GoFitz

## 背景 (Background)

- `003-GoNative-ExcelPdfImage-with-GoOLE-GoFitz` に基づく実装を進めたが、`go-native` 経路の一部は暫定実装に留まっている。
- 現状の `go-native` は、`excel-to-pdf` 側で実シート解析を行わず固定的な `Sheet1` 情報を生成しており、`pdf-to-image` 側は `go-fitz` ではなく `magick` 実行経路を利用している。
- そのため、「主手法を `go-ole` / `go-fitz` にする」という中核要件が未達であり、sidecar の正確性（シート名・ページ数・失敗シート情報）も Python 実装同等には達していない。
- 本仕様では、暫定実装を完了形へ置換するための詳細要件を定義する。

## 要件 (Requirements)

### 必須要件

1. `excel-to-pdf --engine go-native` は `go-ole` による **直接 COM 制御**で実装すること。
   - PowerShell 経由文字列実行を主処理にしないこと。
2. `pdf-to-image --engine go-native` は `go-fitz` による **直接レンダリング**で実装すること。
   - `magick` / `pdftoppm` 実行を主処理にしないこと。
3. `excel-to-pdf` の go-native 実装で、シート単位処理を行うこと。
   - Workbook からシート名を取得する。
   - `--sheets` 指定がある場合は指定 index のみ処理する。
   - シートごとに PDF 出力（または同等に追跡可能な処理）を行い、ページ数を算出する。
4. sidecar (`*.sheet-map.json`) に実データを記録すること。
   - `sheet_entries[].sheet_name` は実シート名。
   - `sheet_entries[].page_count` は実ページ数。
   - 失敗シートがある場合は `export_status=failed` と `error` を記録。
   - `page_sheet_names` はページ単位のシート名展開を正確に格納。
5. `pdf-to-image` の命名規則を sidecar と厳密連携すること。
   - `01_<sanitized_sheet>.png` / `02_<sanitized_sheet>.png` 形式。
   - sidecar 不在/不足時は `NN_page.ext` フォールバック。
6. `--dpi` は go-native レンダリングに実際に反映されること。
   - 200 DPI と 300 DPI で画像寸法が変化すること。
7. `--format` は `png|jpeg` を受け付けること（内部表現 `jpg` でも可）。
8. 引数エラーは exit code `2` を返すこと。
   - `--sheets a,b`
   - 不正 `--engine`
   - 不正 `--format` など
9. `go-native` の制約を明確化すること。
   - 非Windowsの Excel 変換は `go-native` で失敗契約を固定。
   - `go-fitz` 導入に伴う CGO 要件を README に明記。
10. 公開 Go API (`entext` / `excelpdf` / `pdfimage`) から、上記 go-native 完了機能を利用できること。

### 任意要件

1. sidecar schema version を将来拡張しやすいよう、`version` と互換読込方針を明記する。
2. go-native 実行時の debug ログに、シートごとの処理結果（成功/失敗、ページ数）を記録する。

## 実現方針 (Implementation Approach)

### 1. Excel go-native 完了 (`go-ole`)

- `internal/exceltopdf/backend_go_native_windows.go` を実 COM 実装へ置換する。
- 処理フロー:
  1. COM 初期化
  2. Excel Application 起動
  3. Workbook Open
  4. 対象シート決定（`--sheets` 反映）
  5. シート単位で PDF 出力
  6. 出力 PDF のページ数計測
  7. sidecar 構築（`sheet_entries`, `page_sheet_names`）
  8. COM 解放・後処理
- 失敗したシートがあっても、処理継続可能な範囲は継続し、失敗情報を sidecar に残す。
  - 成功シートが1つ以上ある場合は PDF/sidecar を生成し、全体は成功終了とする。
  - 成功シートが0件の場合は runtime error として失敗終了する。

### 2. PDF go-native 完了 (`go-fitz`)

- `internal/pdfnative` パッケージを導入し、結合/レンダリングロジックを集約する。
- `internal/pdftoimage/backend_go_fitz.go` を新設し、`--engine go-native` で必ずこの backend を使用する。
- `dpi/72` スケール行列を適用し、ページごとのピクセルデータを出力する。
- 命名は sidecar 参照を最優先し、`sanitize` を適用する。
  - sidecar 読込優先順位は `--sheet-map` 明示指定 > 自動検出 `<input>.sheet-map.json`。

### 3. CLI/API 契約の確定

- `excel-to-pdf`:
  - `--engine go-native|legacy`
  - `--sheets "1,3,5"`
- `pdf-to-image`:
  - `--engine go-native|legacy`
  - `--dpi`
  - `--sheet-map`
- 公開 API:
  - `ConvertExcelToPDFWithOptions`
  - `ConvertPDFToImageWithOptions`
  - `FileArtifact.SheetMapPath`

### 4. 仕様未達対策

- 既存の暫定ロジック（固定 `Sheet1`、`go-native` で `magick` 使用）を削除する。
- `go-native` の成功条件を「実際に go-ole / go-fitz 経路を通ること」としてテストで担保する。

## 検証シナリオ (Verification Scenarios)

1. `excel-to-pdf` go-native 全シート
   1. `excel-to-pdf --engine go-native -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. `R06_09.pdf` と `R06_09.sheet-map.json` が生成される。
   3. sidecar の `page_sheet_names` 件数が PDF ページ数と一致する。
2. `excel-to-pdf` go-native 部分シート
   1. `excel-to-pdf --engine go-native --sheets "1,3,5" -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. sidecar の `sheet_entries` が index 1/3/5 のみを含む。
3. `pdf-to-image` go-native + sidecar 命名
   1. `pdf-to-image --engine go-native --sheet-map tmp/entext-test/pdf/R06_09.sheet-map.json -i tmp/entext-test/pdf/R06_09.pdf -o tmp/entext-test/images --format png --dpi 300`
   2. `01_<sheet>.png` 形式で出力される。
   3. 同一入力で `--dpi 200` と比較して画像寸法が変化する。
4. sidecar 不在フォールバック
   1. `pdf-to-image --engine go-native -i tmp/entext-test/pdf/R06_09.pdf -o tmp/entext-test/images --format png`
   2. `NN_page.png` 命名で出力される。
5. 引数エラー契約
   1. `excel-to-pdf --engine go-native --sheets a,b -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. exit code `2` で失敗する。
   3. stderr に validation error が出力される。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `scripts/process/build.sh`
2. 共通統合テスト（go-native 仕様）:
   - `scripts/process/integration_test.sh --specify "GoNative|SheetMap|Sanitize|InvalidSheets|DPI|Engine|Backend"`
3. 連鎖系統合テスト（回帰確認）:
   - `scripts/process/integration_test.sh --specify "pipeline_chain|image_to_markdown|session"`

### 要件対応表

- 要件1-3（go-ole 実装とシート処理）:
  - 単体: COMラッパ/シート解析/`--sheets` パーサ
  - 統合: `samples/R06_09.xlsx` を使った sidecar 内容検証
- 要件4-5（sidecar 正確性と命名）:
  - 単体: sidecar serialize/deserialize、sanitize、命名関数
  - 統合: 生成画像名と `page_sheet_names` の一致
- 要件6-7（dpi/format）:
  - 単体: format 正規化、dpi バリデーション
  - 統合: 200/300 DPI 出力比較
- 要件8（exit code 2 契約）:
  - E2E: 不正 `--sheets` / `--engine` / `--format`
- 要件9-10（制約明示/API公開）:
  - ドキュメント: README に Windows/CGO 制約を反映
  - 統合: `entext` 公開 API 経由の実行テスト

## 実装確定事項 (Finalized Contracts)

- `excel-to-pdf --engine go-native` は PowerShell を経由せず、`go-ole` で Excel COM を直接操作する。
- `pdf-to-image --engine go-native` は `go-fitz` の直接レンダリングを使用し、legacy backend (`pdftoppm` / `magick`) を経由しない。
- sidecar 読込は `ReadCompat` で後方互換し、`version` 欠落時は `version=1` として扱う。
- 引数エラーは CLI で `exit code 2`、実行時エラーは `exit code 1` を維持する。
