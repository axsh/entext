package main

import (
	"context"
	"os"

	"github.com/axsh/entext"
	"github.com/axsh/entext/internal/common/config"
	"github.com/axsh/entext/internal/common/exitcode"
	commonio "github.com/axsh/entext/internal/common/io"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()
	var (
		inputPath  string
		useStdin   bool
		outputDir  string
		format     string
		backend    string
		engine     string
		dpi        int
		sheetMap   string
		configPath string
		verbose    bool
		quiet      bool
		outputMode string
		printJSON  bool
	)

	cmd := &cobra.Command{
		Use:   "pdf-to-image",
		Short: "Convert PDF into images",
		Run: func(cmd *cobra.Command, args []string) {
			v, err := config.SetupViper(cmd, configPath, "doc_convert_pdf_image")
			if err != nil {
				_, _ = os.Stderr.WriteString(err.Error() + "\n")
				exitcode.ExitWithError(err)
			}
			cfg := config.ReadCommon(v)
			mode, err := commonio.ResolveOutputMode(cfg.OutputMode, cfg.PrintJSON)
			if err != nil {
				exitcode.ExitWithError(err)
			}
			logger := commonio.NewLogger(os.Stderr, cfg.Verbose, cfg.Quiet)
			if err := commonio.ValidateStdinReady(useStdin, os.Stdin); err != nil {
				logger.Error("stdin validation failed", "error", err)
				exitcode.ExitWithError(err)
			}

			inputs, err := commonio.ResolveInputPaths(commonio.ResolveInputArgs{
				InputPath: inputPath,
				UseStdin:  useStdin,
				Stdin:     os.Stdin,
			})
			if err != nil {
				exitcode.ExitWithError(err)
			}
			results := make([]string, 0)
			for _, input := range inputs {
				logger.Debug("converting pdf file", "input", input, "output_dir", outputDir, "format", format, "backend", backend, "engine", engine, "dpi", dpi, "sheet_map", sheetMap)
				artifact, convErr := entext.ConvertPDFToImageWithOptions(ctx, entext.FileJob{
					InputPath: input,
					OutputDir: outputDir,
				}, format, entext.PDFImageOptions{
					Backend:      backend,
					Engine:       engine,
					DPI:          dpi,
					SheetMapPath: sheetMap,
				})
				if convErr != nil {
					logger.Error("conversion failed", "input", input, "error", convErr)
					exitcode.ExitWithError(convErr)
				}
				results = append(results, artifact.Paths...)
			}
			if err := commonio.WriteResultPaths(os.Stdout, mode, results); err != nil {
				exitcode.ExitWithError(err)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&inputPath, "input", "i", "", "Input PDF file path")
	flags.BoolVar(&useStdin, "stdin", false, "Read input paths from stdin")
	flags.StringVarP(&outputDir, "output-dir", "o", ".", "Output directory")
	flags.StringVar(&format, "format", "png", "Output format: png|jpg")
	flags.StringVar(&backend, "backend", "auto", "Backend: auto|pdftoppm|magick")
	flags.StringVar(&engine, "engine", "legacy", "Engine: legacy|go-native")
	flags.IntVar(&dpi, "dpi", 200, "Output image DPI")
	flags.StringVar(&sheetMap, "sheet-map", "", "Path to sheet-map JSON sidecar")
	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flags.BoolVar(&quiet, "quiet", false, "Suppress info logs")
	flags.StringVar(&outputMode, "output-mode", "path", "Output mode: path|json")
	flags.BoolVar(&printJSON, "print-json", false, "Alias for --output-mode json")

	if err := cmd.Execute(); err != nil {
		exitcode.ExitWithError(err)
	}
}
