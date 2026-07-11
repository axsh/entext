# 002 Conversion Backend Selection And Fallback

## 背景 (Background)

- 直近の検証で、`libreoffice` や `pdftoppm` が未導入の環境ではツール本体の変換が成立せず、期待していた `xlsx -> pdf -> image -> markdown` の検証ができなかった。
- 一時的に Python で直接変換して Markdown を作成したが、これは `entext` ツールの価値検証にはならず、要件に対するテストとして不適切だった。
- 変換手段は `libreoffice` だけではなく、Windows では Excel COM を含め複数候補が存在する。利用可能な手段を選択できる設計が必要である。
- 失敗時に「どのバックエンドを試し、なぜ失敗したか」を明確に報告しないと、運用時の障害対応が困難になる。

## 要件 (Requirements)

### 必須要件

1. `excel-to-pdf` は複数バックエンドをサポートすること。
   - 最低サポート:
     - `libreoffice`
     - `excel-com`（Windows）
2. `pdf-to-image` は複数バックエンドをサポートすること。
   - 最低サポート:
     - `pdftoppm`
     - `magick`（ImageMagick/Ghostscript 経由）
3. 両コマンドにバックエンド指定オプションを追加すること。
   - 例:
     - `excel-to-pdf --backend libreoffice|excel-com|auto`
     - `pdf-to-image --backend pdftoppm|magick|auto`
4. バックエンド指定がある場合:
   - 指定された手段のみを使用する。
   - 利用不可/失敗時はフォールバックせず即エラーとする。
5. バックエンド指定が無い（`auto`）場合:
   - 事前定義した優先順で複数手段を順に試行する。
   - 全手段失敗時は最終的にエラーを返す。
6. 失敗時エラーメッセージに、試行したバックエンド一覧と各失敗理由を含めること。
7. `image-to-markdown` への入力は `pdf-to-image` の正規出力（画像ファイル）で行い、ツールチェーン全体で検証すること。
8. ツール外の代替処理（例: ad-hoc Python 変換）を検証成功として扱わないこと。

### 追加必須要件（運用性）

1. `--backend` の `--help` を整備し、必要な前提ソフトを明記すること。
2. ログ（DEBUG）で以下を追跡可能にすること。
   - 選択されたバックエンド
   - `auto` 時の試行順
   - 各試行の結果（success/failure）
3. Windows 環境で `excel-com` が使用可能な場合は、`auto` で優先的に選択できるようにすること（優先順は仕様で固定）。

## 実現方針 (Implementation Approach)

### 1. バックエンド抽象化

- `features/entext/internal/exceltopdf` と `features/entext/internal/pdftoimage` にバックエンドインターフェースを導入する。
- 例:
  - `type Backend interface { Name() string; Convert(ctx context.Context, ...) (...) }`
- `auto` は orchestrator が候補配列を順に実行し、最初の成功を採用する。

### 2. `excel-to-pdf` 実装方針

- `libreoffice` backend:
  - 既存ロジックを移行。
- `excel-com` backend（Windows）:
  - PowerShell or COM 呼び出しラッパーで Excel Application による PDF Export を実行。
- `auto` 優先順（Windows）:
  1. `excel-com`
  2. `libreoffice`
- `auto` 優先順（非Windows）:
  1. `libreoffice`

### 3. `pdf-to-image` 実装方針

- `pdftoppm` backend:
  - 既存ロジックを移行。
- `magick` backend:
  - `magick -density ... input.pdf output_%03d.png` 相当で生成。
  - 既存命名規約 `<basename>_<nnn>.<ext>` に正規化。
- `auto` 優先順:
  1. `pdftoppm`
  2. `magick`

### 4. エラー集約方針

- `auto` 失敗時のエラー型を追加:
  - `BackendAttemptError`（backend, reason）
  - `BackendAggregateError`（attempts[]）
- CLI は最終エラーとして以下を表示:
  - `all backends failed`
  - `tried: excel-com(...), libreoffice(...)`

### 5. CLI/API 方針

- CLI:
  - `--backend` 追加（default: `auto`）
- 公開 API（`github.com/axsh/entext`）:
  - `ConvertExcelToPDF` / `ConvertPDFToImage` に backend 指定を追加または option 化。
- 既存 `image-to-markdown` は変更不要。ただし、検証は必ず upstream 2段の出力を使って行う。

## 検証シナリオ (Verification Scenarios)

1. `excel-to-pdf` 指定バックエンド成功
   1. `excel-to-pdf --backend excel-com -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. PDF が生成される。
   3. stdout に生成PDFパスが出力される。

2. `excel-to-pdf` 指定バックエンド失敗
   1. `excel-to-pdf --backend libreoffice -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. `libreoffice` 未導入なら即失敗する。
   3. 他手段へフォールバックせずエラー終了する。

3. `excel-to-pdf` auto フォールバック
   1. `excel-to-pdf --backend auto -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. 優先順で手段を試行する。
   3. 利用可能手段で成功するか、全失敗なら集約エラーを返す。

4. `pdf-to-image` 指定バックエンド失敗
   1. `pdf-to-image --backend pdftoppm -i tmp/entext-test/pdf/R06_09.pdf -o tmp/entext-test/images --format png`
   2. `pdftoppm` 未導入なら即失敗する。
   3. 他手段へフォールバックしない。

5. `pdf-to-image` auto フォールバック
   1. `pdf-to-image --backend auto -i tmp/entext-test/pdf/R06_09.pdf -o tmp/entext-test/images --format png`
   2. `pdftoppm` -> `magick` の順で試行する。
   3. 成功時は `<basename>_<nnn>.png` が出力される。

6. ツールチェーン全体検証
   1. `excel-to-pdf --backend auto -i samples/R06_09.xlsx -o tmp/entext-test/pdf`
   2. `pdf-to-image --backend auto -i <generated.pdf> -o tmp/entext-test/images --format png`
   3. `image-to-markdown -i <generated_001.png> -o tmp/entext-test/md/R06_09.md`
   4. Markdown と session log が出力される。

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `scripts/process/build.sh`

2. 共通統合テスト（バックエンド選択/フォールバック）:
   - `scripts/process/integration_test.sh --categories "common" --specify "excel_to_pdf_backend|pdf_to_image_backend|backend_auto|backend_error_aggregate"`

3. LLM統合テスト（チェーン終端の確認）:
   - `scripts/process/integration_test.sh --categories "llm" --specify "image_to_markdown|pipeline_chain|session_log"`

## 未実施・未達項目 (Pending Items at Review Point)

1. ツールチェーン E2E（要件7）の未達:
   - `excel-to-pdf -> pdf-to-image -> image-to-markdown` のフルチェーン成功証跡が不足。
   - `samples/R06_09.xlsx` で `excel-to-pdf` は成功したが、`pdf-to-image` は `pdftoppm`/`magick` 未導入で停止し、最終段まで到達できていない。
2. 実行環境依存の未解消:
   - `pdf-to-image` の backend 実行に必要な外部ソフトが無い環境では、成功系E2Eが実施不能。
   - 現時点では失敗系（集約エラーの妥当性）中心の検証に留まっている。
3. 統合テストコマンド記述の不整合:
   - 本仕様書の `--categories` 付きコマンドは、現行 `scripts/process/integration_test.sh` の対応オプションと不一致。
   - 現行スクリプト前提では `--specify` のみで記載する必要がある。
4. 入力不正時の終了コード契約ギャップ:
   - backend 不正指定時に `exit code 2`（引数エラー）を期待したE2Eが未達。
   - 実行上は `exit code 1` で終了する挙動が確認されており、契約整理が必要。

### 要件対応表

- 要件1-5（複数 backend + 指定/auto 挙動）:
  - 単体テスト: backend resolver / orchestrator のテーブル駆動テスト
  - 統合テスト: 指定成功・指定失敗・auto成功・auto全失敗
- 要件6（エラー集約）:
  - 単体テスト: aggregate error の内容検証
  - 統合テスト: 実行ログ/エラー表示検証
- 要件7（チェーン検証）:
  - 統合テスト: excel->pdf->image->markdown の連結シナリオ
- 要件8（ツール外処理を成功扱いしない）:
  - 運用ルール: 検証結果報告時に「entext コマンド実行ログ」を必須証跡とする

## テストコマンド補足 (Current Script Constraints)

- 現行 `scripts/process/integration_test.sh` は `--categories` を受け付けず、`--specify` のみ対応。
- したがって、上記の統合テスト実行例は次のように読み替える。
  - `scripts/process/integration_test.sh --specify "excel_to_pdf_backend|pdf_to_image_backend|backend_auto|backend_error_aggregate|pipeline_chain|session_log"`
