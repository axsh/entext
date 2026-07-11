# entext

Enterprise document conversion toolkit with CLI and embeddable Go API.

## Installation

```bash
go get github.com/axsh/entext
```

## Requirements

- Go `1.26.4+` (required by `github.com/axsh/arctic-tern`)
- `arctic-tern` compatible server endpoint for image analysis
- `excel-to-pdf --engine go-native`:
  - Windows
  - Installed Microsoft Excel (COM automation)
- `pdf-to-image --engine go-native`:
  - CGO-enabled Go toolchain
  - `go-fitz` supported build environment
- Legacy engine (`--engine legacy`) still depends on external tools:
  - Excel -> PDF: `excel-com` (Windows Excel COM) or `libreoffice`
  - PDF -> image: `pdftoppm` or `magick` (ImageMagick + PDF delegate)

## CLI Usage

`entext` provides three CLI commands under `cmd`.

```bash
bin/entext/excel-to-pdf -i ./sample.xlsx -o ./tmp/pdf --backend auto
bin/entext/pdf-to-image -i ./tmp/pdf/sample.pdf -o ./tmp/images --format png --backend auto
bin/entext/image-to-markdown -i ./tmp/images/sample_001.png -o ./tmp/md/sample.md
```

All commands support:

- `-i/--input`
- `--stdin` (one path per line)
- `-o/--output` or `--output-dir` (command dependent)
- `--output-mode path|json`

Image-to-markdown tern runtime options:

- `image-to-markdown --tern-mode auto|external|inproc`
- `image-to-markdown --tern-config ./settings/tern/tern-config.yaml`
- `auto` tries `--server` first and falls back to in-process tern server boot.
- `inproc` boots `github.com/axsh/arctic-tern/server` in the same process.

`image-to-markdown` final output is table-faithful Markdown centered on the source table from the image.
Gap judgment is limited to conversion fidelity (faithful transcription from the image) and uses a binary verdict: `SUFFICIENT` or `INSUFFICIENT`.
Intermediate phase analysis tables and explanatory report sections (for example element inventories or interpretation tables) are not included in the delivered document.

Backend selection:

- `excel-to-pdf --backend auto|libreoffice|excel-com`
  - `auto`: Windows=`excel-com -> libreoffice`, non-Windows=`libreoffice`
- `pdf-to-image --backend auto|pdftoppm|magick`
  - `auto`: `pdftoppm -> magick`
- Specific backend mode does not fallback to other backends.
- `auto` mode returns aggregated errors with tried backend names and reasons.

Go-native related options:

- `excel-to-pdf --engine legacy|go-native`
- `excel-to-pdf --sheets "1,3,5"`
- `pdf-to-image --engine legacy|go-native`
- `pdf-to-image --dpi 200`
- `pdf-to-image --sheet-map ./tmp/pdf/sample.sheet-map.json`

`excel-to-pdf` writes sidecar metadata (`*.sheet-map.json`) next to generated PDF.
`pdf-to-image --engine go-native` reads sidecar in the following order:

1. Explicit `--sheet-map` path
2. Auto-detected `<input_pdf_basename>.sheet-map.json` in the same directory

Go-native sidecar behavior:

- `sheet_entries` contains real sheet name, page count, and failure reason when available.
- `page_sheet_names` is expanded per page and is used for image naming.
- Output naming format is `NN_<sanitized_sheet_name>.<ext>`.
- If sidecar is missing or page mapping is insufficient, naming falls back to `NN_page.<ext>`.

## Go API Usage

Import the root package:

```go
import "github.com/axsh/entext"
```

Example:

```go
ctx := context.Background()

pdf, err := entext.ConvertExcelToPDF(ctx, entext.FileJob{
    InputPath: "sample.xlsx",
    OutputDir: "tmp/pdf",
})
if err != nil {
    return err
}

imgs, err := entext.ConvertPDFToImage(ctx, entext.FileJob{
    InputPath: pdf.Paths[0],
    OutputDir: "tmp/images",
}, "png")
if err != nil {
    return err
}

cfg := entext.DefaultImageToMarkdownConfig()
md, err := entext.ConvertImageToMarkdown(ctx, entext.ImageToMarkdownJob{
    InputPath: imgs.Paths[0],
    OutputDir: "tmp/md",
}, cfg)
if err != nil {
    return err
}
_ = md
```

Backend-explicit API:

```go
pdf, err := entext.ConvertExcelToPDFWithBackend(ctx, entext.FileJob{
    InputPath: "sample.xlsx",
    OutputDir: "tmp/pdf",
}, "excel-com")

imgs, err := entext.ConvertPDFToImageWithBackend(ctx, entext.FileJob{
    InputPath: "tmp/pdf/sample.pdf",
    OutputDir: "tmp/images",
}, "png", "magick")
```

Go-native options API:

```go
pdf, err := entext.ConvertExcelToPDFWithOptions(ctx, entext.FileJob{
    InputPath: "sample.xlsx",
    OutputDir: "tmp/pdf",
}, entext.ExcelPDFOptions{
    Backend: "auto",
    Engine:  "go-native",
    Sheets:  "1,3,5",
})

imgs, err := entext.ConvertPDFToImageWithOptions(ctx, entext.FileJob{
    InputPath: pdf.Paths[0],
    OutputDir: "tmp/images",
}, "png", entext.PDFImageOptions{
    Backend:      "auto",
    Engine:       "go-native",
    DPI:          300,
    SheetMapPath: pdf.SheetMapPath,
})
```

Image-to-markdown in-process runtime API:

```go
cfg := entext.DefaultImageToMarkdownConfig()
cfg.TernMode = "inproc"
cfg.TernConfigPath = "settings/tern/tern-config.yaml"
cfg.ServerURL = "http://localhost:3100" // used when tern mode is external/auto
```

## Vault Keyring Utility

`features/vault-cli` provides keyring setup for tern vault references:

```bash
go run ./cmd/vault-cli --help
go run ./cmd/vault-cli set --provider openai --name default --secret "YOUR_KEY"
go run ./cmd/vault-cli set --provider anthropic --name default --secret "YOUR_KEY"
go run ./cmd/vault-cli get --provider openai --name default
```

These map to:

- `vault://providers/openai/default`
- `vault://providers/anthropic/default`
