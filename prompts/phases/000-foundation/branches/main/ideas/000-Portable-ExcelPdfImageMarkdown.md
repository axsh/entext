# 000 Portable Excel-PDF-Image-Markdown Toolchain

## 背景 (Background)

- ドキュメント変換処理を「Excel -> PDF -> 画像 -> Markdown」の段階に分離し、用途に応じて個別実行できる構成が必要である。
- 一括変換は単一コマンドに閉じるより、単機能CLIをスクリプトで連結する方が運用しやすい。
- 変換結果を他ツールへ受け渡す時に、OS依存を減らしながら再利用できるポータブルなCLI設計が求められている。
- 画像解析からMarkdown化する処理は既存実装 (`features/image-to-markdown`) の知見を活用しつつ、再利用可能な部品として整理したい。

## 要件 (Requirements)

### 必須要件

1. 3つの独立したCLIツールを提供すること。
   - `excel-to-pdf`
   - `pdf-to-image`
   - `image-to-markdown`
2. すべてのツールは Go + `cobra` + `viper` をベースに実装すること。
3. 各ツールは `-i` (入力) と `-o` (出力) を基本オプションとして受け付けること。
4. すべてのツールで `--stdin` をサポートし、標準入力から入力ファイルパス（1行1パス）を受け取れること。
   - `-i` と `--stdin` の同時指定はエラーにする。
   - `-i` と `--stdin` のどちらも未指定の場合はエラーにする（待機しない）。
5. 入出力はファイルシステム上で完結し、単体で実行可能であること。
6. 終了コードを厳密化すること。
   - 成功: `0`
   - 引数不正/入力不正: `2`
   - 変換失敗/実行時エラー: `1`
7. 標準出力と標準エラー出力の責務を分離すること。
   - `stdout`: 機械可読データ（生成物パス、またはJSON出力）
   - `stderr`: 人間向けログ、警告、エラー
8. パイプ連携しやすいように、各コマンドは標準出力へ「生成物パス」を1行1ファイルで出力すること。
9. クロスプラットフォーム実行を考慮し、パス操作は `filepath` を使い、OS固有の区切り文字を内部吸収すること。

### ツール別必須要件

#### 1) `excel-to-pdf`

- `-i, --input` で単一Excelファイルを指定する。
- `--stdin` で標準入力からExcelファイルパス群を受け取り、複数件を順次処理可能にする。
- `-o, --output-dir` で出力先ディレクトリを指定する。
- 入力1ファイルからPDF 1ファイルを生成する。
- 生成ファイル名は入力ファイルのベース名を継承する（例: `report.xlsx -> report.pdf`）。

#### 2) `pdf-to-image`

- `-i, --input` で単一PDFを指定する。
- `--stdin` で標準入力からPDFファイルパス群を受け取り、複数件を順次処理可能にする。
- `-o, --output-dir` で出力先ディレクトリを指定する。
- `--format` で画像形式を `png` または `jpg` から選択できること（デフォルト `png`）。
- PDFの全ページを連番で出力すること。
  - 形式: `<basename>_<nnn>.<ext>` (`nnn` は3桁ゼロ埋め)

#### 3) `image-to-markdown`

- `-i, --input` で単一画像を指定する。
- `--stdin` で標準入力から画像ファイルパス群を受け取り、複数件を順次処理可能にする。
- `-o, --output` または `--output-dir` を受け付ける。
  - 単一入力（`-i`）では `--output` 指定を基本とする。
  - 複数入力（`--stdin`）では `--output-dir` 指定を必須とする。
- `-r, --ref` を複数指定可能にすること。
  - 指定値は正規表現として扱い、マッチしたMarkdownファイル群を参照コンテキストとして読み込む。
  - `-r` は複数回指定可能（例: `-r "glossary.*\\.md" -r "results/.*\\.md"`）。
- 既存 `features/image-to-markdown` の多段解析アルゴリズムを再現すること。
  - Phase 0: 分類（`simple_text` / `complex_table` / `diagram` / `mixed`）
  - `simple_text` は直接変換ショートパスを実行し、Phase 1-5 をスキップする。
  - Phase 1-4: `AssessGap -> GenerateQuestion -> 画像付き回答` の適応ループ（最大5ラウンド）
  - Phase 5: 収集結果を統合して最終Markdown生成
- 既存アルゴリズムの入出力契約を再現すること。
  - Markdown: `<output-dir>/<basename>.md`
  - SessionLog: `<output-dir>/_sessions/<basename>_session.json`
  - `SessionLog / PhaseLog / RoundLog` の構造を互換維持する。

### 追加必須要件

以下は任意ではなく、すべて必須要件として扱う。

1. `--config` で設定ファイル読込を可能にし、`viper` でCLIオプションとの優先順位を管理する。
2. `--verbose` で処理詳細ログを出力可能にする。
3. `--quiet` で情報ログを抑制し、警告・エラー中心の出力に切り替える。
4. `--output-mode path|json` で標準出力形式を切り替え可能にする。
5. `--print-json` は `--output-mode json` の互換エイリアスとして提供する。
6. `image-to-markdown` で既存実装の改善を行う。
   - `SUFFICIENT` 判定の誤検知回避（`NOT SUFFICIENT` との混同防止）
   - セッションログへ `question` を必ず保存
7. `image-to-markdown` の改善挙動は、互換性を保つために明示オプションで有効化できるようにする。
   - 例: `--strict-gap-judge`, `--save-question-log`, `--phase-sleep-ms`, `--round-sleep-ms`

## 実現方針 (Implementation Approach)

### 1. 構成方針

- 3ツールを単一Go module内の複数コマンドとして実装する。
- 想定構成:
  - `features/doc-convert/cmd/excel-to-pdf/main.go`
  - `features/doc-convert/cmd/pdf-to-image/main.go`
  - `features/doc-convert/cmd/image-to-markdown/main.go`
  - `features/doc-convert/internal/...` に共通ライブラリを配置
- 変換処理はコマンド本体から分離し、`internal/<tool>/service.go` で責務分割する。

### 2. `cobra` / `viper` 方針

- 全コマンド共通で以下を適用:
  - `cobra` でCLI定義
  - `viper.BindPFlag` で全オプションを設定へ束縛
  - `viper.AutomaticEnv` で環境変数上書きを許可
- 優先順位: `CLI > ENV > config file > default`

### 3. 入出力契約とパイプ連携方針

- 生成物の内容（バイナリ）を標準出力へ流す方式は採用しない。
- `stdout` には機械可読な結果のみを出し、ログは `stderr` へ分離する。
- `--stdin` 明示時のみ標準入力を読む。未指定時に暗黙でstdin待ちはしない。
- 代わりに、生成したファイルパス一覧を標準出力へ出すことで連結可能にする。
- 例:
  - `excel-to-pdf -i a.xlsx -o ./tmp/pdf | pdf-to-image --stdin -o ./tmp/img --format png | image-to-markdown --stdin --output-dir ./out`
  - `printf '%s\n' a.xlsx b.xlsx | excel-to-pdf --stdin -o ./tmp/pdf | pdf-to-image --stdin -o ./tmp/img --format jpg`

### 4. 画像解析アルゴリズム方針（`image-to-markdown` 再現）

- 既存 `features/image-to-markdown` の設計を再現し、以下フェーズを維持する。
  1. Phase 0: 画像分類
  2. Phase 1-4: 欠損情報を埋める適応ループ
  3. Phase 5: Markdown統合
- `--ref` で読み込んだ参照Markdownを、分類プロンプトと各フェーズの補助コンテキストへ注入する。
- セッションログ出力 (`_sessions`) を標準機能として維持し、後追い検証を可能にする。

#### 4.1 CLI パラメータ再現要件

- 既存実装との整合のため、少なくとも以下を受け付ける:
  - `--server`（default: `http://localhost:3100`）
  - `--agent`（default: `codex`）
  - `--model`（default: `gpt-5.3-codex`）
  - `--verbose`
- 本仕様の共通I/O契約（`-i` / `--stdin` / `-o` / `--output-dir`）と両立させる。
- 単一画像モードと複数入力モード（stdin経由）で同一アルゴリズムを適用する。

#### 4.2 API コールシーケンス再現要件

- 1画像あたり、以下の順序でセッションを進行させる。
  1. `CreateSession(agent, model, workdir)`
  2. `SendText(classifyPrompt + [Attached image: absPath])`
  3. `collectStreamText()`
  4. 分類が `simple_text` の場合:
     - `SendText(直接変換プロンプト)` -> `collectStreamText()` -> return
  5. `simple_text` 以外の場合、各 Phase で:
     - `SendText(AssessGapPrompt(...))` -> `collectStreamText()`
     - 十分でない場合:
       - `SendText(GenerateQuestionPrompt(...))` -> `collectStreamText()`
       - `sleep(5s)` -> `SendText(question + 強制suffix + [Attached image: absPath])`
       - `collectStreamText()`
  6. 各Phase終了後に `sleep(5s)`
  7. `SendText(GenerateMarkdownPrompt(phaseLogs))` -> `collectStreamText()`
  8. `TerminateSession()`

#### 4.3 プロンプト再現要件

- 以下の固定/テンプレート文字列を既存設計に準拠して再現する。
  - `ClassifyPrompt`
  - `simple_text` 直接変換プロンプト
  - `AssessGapPrompt`
  - `GenerateQuestionPrompt`
  - 実行質問時の強制 suffix
  - `GenerateMarkdownPrompt`
- 画像参照は毎回 `[Attached image: <absPath>]` を使い、画像バイナリ直送ではなくパス参照方式を採用する。
- 最終Markdownの制約（要約禁止、テーブル全行全列、入れ子展開、Mermaid + 説明文、書式情報反映、日本語出力）を維持する。

#### 4.4 既知挙動の扱い

- 再現対象として既知挙動を明示管理する。
  - `contains("SUFFICIENT")` の部分一致判定
  - `RoundLog.Question` の保存有無
  - simple_text ショートパス時のログ粒度
- 実装時は次の方針を採る:
  - デフォルトは既存互換挙動を再現する。
  - 改善挙動を導入する場合は、互換を壊さない明示オプション（例: `--strict-gap-judge`）で切替可能にする。

#### 4.5 改善ポイント（互換維持前提）

- 優先度P1: Gap判定の厳密化
  - 現状課題: `contains("SUFFICIENT")` により `"NOT SUFFICIENT"` を誤判定しうる。
  - 改善案: 行単位の厳密判定（例: `^SUFFICIENT$` または `Decision:SUFFICIENT`）へ変更。
  - 互換方針: 既定は互換、`--strict-gap-judge` 有効時に厳密判定を使用。

- 優先度P1: `RoundLog.Question` の保存保証
  - 現状課題: 質問生成後に `roundLog.Question` 未代入で空になりうる。
  - 改善案: 質問生成直後に必ず保存し、セッションログの追跡性を確保。
  - 互換方針: 既定有効でも互換性破壊がないため、標準動作として有効化してよい。

- 優先度P2: スリープ時間の設定可能化
  - 現状課題: ラウンド内/フェーズ間の固定 `5s` が処理時間を増大させる。
  - 改善案: `--round-sleep-ms`, `--phase-sleep-ms` で調整可能にし、既定値は `5000` を維持。
  - 互換方針: 既定値据え置きで互換維持。

- 優先度P2: ラウンド制御ロジックの明確化
  - 現状課題: 実質発火しない条件分岐があり、意図が不透明。
  - 改善案: `max-rounds` のみで上限管理し、拡張条件は削除または明示設定化。
  - 互換方針: 互換モードでは既存分岐を残し、改善モードで単純化。

- 優先度P3: simple_text 経路の最小ログ拡充
  - 現状課題: ショートパス時に Phaseログが薄く、解析根拠の追跡が弱い。
  - 改善案: 分類結果とショートパス実行理由を `SessionLog` に保存。
  - 互換方針: 出力Markdownには影響しないため、既定有効化しても問題が少ない。

### 5. 失敗時設計

- 個別コマンドは単一入力の失敗で即終了する。
- バルク実行は上位スクリプト責務とし、CLI自体は単機能を維持する。

## 検証シナリオ (Verification Scenarios)

1. 単一Excel変換の基本動作
   1. `excel-to-pdf -i ./samples/book1.xlsx -o ./tmp/pdf`
   2. `./tmp/pdf/book1.pdf` が生成される
   3. 標準出力に生成PDFパスが1行出力される
   4. ログは標準エラー出力にのみ表示される

2. PDFから画像への分解
   1. `pdf-to-image -i ./tmp/pdf/book1.pdf -o ./tmp/images --format png`
   2. `book1_001.png`, `book1_002.png`, ... がページ数分生成される
   3. 標準出力に生成画像パスがページ順で出力される

3. 画像からMarkdownへの変換（参照なし）
   1. `image-to-markdown -i ./tmp/images/book1_001.png -o ./tmp/md/book1_001.md`
   2. Markdownファイルが生成される
   3. `_sessions` 配下にセッションログが保存される

4. 画像からMarkdownへの変換（参照あり）
   1. `image-to-markdown -i ./tmp/images/book1_002.png -o ./tmp/md/book1_002.md -r "docs/glossary.*\\.md" -r "tmp/md/book1_001.*\\.md"`
   2. `-r` 指定でマッチした複数Markdownが読み込まれる
   3. 出力Markdownに用語統一および前段結果との整合が反映される

5. `--stdin` 明示のコマンド連結運用
   1. `excel-to-pdf -i ./samples/book1.xlsx -o ./tmp/pdf | pdf-to-image --stdin -o ./tmp/images --format png | image-to-markdown --stdin --output-dir ./tmp/md`
   2. Excel -> PDF -> Image -> Markdown が連続実行される
   3. 各段のログは標準エラー出力に分離される
   4. 失敗時に終了コードで異常検知できる

6. `--stdin` 入力バリデーション
   1. `pdf-to-image --stdin -o ./tmp/images` をstdin未接続で実行する
   2. 即時に引数エラー（終了コード2）となる
   3. ハングせずに終了する

7. `image-to-markdown` 再現シナリオ（single image）
   1. `image-to-markdown -i ./tmp/images/book1_010.png --output-dir ./tmp/md --server http://localhost:3100 --agent codex --model gpt-5.3-codex --verbose`
   2. `CreateSession -> Phase0 -> (必要ならPhase1-4) -> Phase5 -> TerminateSession` の順で完了する
   3. `./tmp/md/book1_010.md` と `./tmp/md/_sessions/book1_010_session.json` が生成される

8. `image-to-markdown` 再現シナリオ（simple_text short path）
   1. simple_text画像で `image-to-markdown` を実行する
   2. 分類後に直接変換プロンプト1回で完了し、Phase1-5は実行しない
   3. 出力Markdownが生成され、終了コード0で終わる

9. `image-to-markdown` プロンプト再現検証
   1. セッションログ/デバッグログでプロンプト送信内容を確認する
   2. `ClassifyPrompt`, `AssessGapPrompt`, `GenerateQuestionPrompt`, `GenerateMarkdownPrompt` が仕様どおりであることを確認する
   3. 実行質問で `[Attached image: <absPath>]` が付与されることを確認する

10. 改善モード検証（互換モードとの差分）
   1. `--strict-gap-judge` 有効時に `"NOT SUFFICIENT"` を十分判定しないことを確認する
   2. セッションログに `RoundLog.Question` が埋まることを確認する
   3. `--round-sleep-ms 0 --phase-sleep-ms 0` で処理時間短縮と機能同等性を確認する

## テスト項目 (Testing for the Requirements)

### ビルド・全体検証

1. ビルド＋単体テスト:
   - `scripts/process/build.sh`

2. 共通CLI基盤とファイル入出力の統合テスト:
   - `scripts/process/integration_test.sh --categories "common" --specify "excel-to-pdf|pdf-to-image|image-to-markdown|cobra|viper"`

3. LLM連携を含む画像解析統合テスト:
   - `scripts/process/integration_test.sh --categories "llm" --specify "image-to-markdown|phase|session"`

### 要件対応表

- 要件1-3,5（3ツール分離、`-i/-o`、単機能実行）:
  - 単体テスト: 各コマンドの引数解析テスト
  - 統合テスト: `common` カテゴリのCLI起動試験
- 要件4（`--stdin` と `-i` の排他、未指定エラー）:
  - 単体テスト: フラグ組み合わせバリデーションテスト
  - 統合テスト: stdin接続/未接続ケース
- 要件6（終了コード）:
  - 単体テスト: エラー種別ごとの戻り値テスト
  - 統合テスト: 不正入力・変換失敗ケース
- 要件7（`stdout`/`stderr` 分離）:
  - 単体テスト: ログ出力先のテスト
  - 統合テスト: パイプ連結時の出力混入防止
- 要件8（標準出力の生成物パス）:
  - 単体テスト: 出力フォーマットテスト
  - 統合テスト: パイプ連結シナリオ
- 要件9（ポータビリティ）:
  - 単体テスト: パス正規化・拡張子解決
  - 統合テスト: OS依存文字を含むパスケース
- `image-to-markdown` 要件（`-r/--ref` の正規表現複数指定）:
  - 単体テスト: 複数パターンの解決順序と重複除外
  - 統合テスト: 参照あり/なしでの出力差分確認
- `image-to-markdown` 要件（既存アルゴリズム再現）:
  - 単体テスト: 分類抽出、Phase遷移、終了条件、ログ構造
  - 統合テスト: APIコール順と最終Markdown生成の互換性
- `image-to-markdown` 要件（プロンプト原文互換）:
  - 単体テスト: 各プロンプトテンプレートの文字列一致/必須句含有
  - 統合テスト: 実行時プロンプトの構成検証（suffix + image attachment）
- 追加必須要件（設定/ログ/出力形式）:
  - 単体テスト: `--config`, `--verbose`, `--quiet`, `--output-mode`, `--print-json` の引数解決と優先順位
  - 統合テスト: `stdout` 機械可読性を維持したまま `stderr` にログ分離されること
- 追加必須要件（改善挙動の切替）:
  - 単体テスト: `--strict-gap-judge`, `--save-question-log`, `--phase-sleep-ms`, `--round-sleep-ms` の切替動作
  - 統合テスト: 互換モードと改善モードで期待どおりに挙動差分が出ること
