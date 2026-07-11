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
		backend    string
		sheets     string
		configPath string
		verbose    bool
		quiet      bool
		outputMode string
		printJSON  bool
	)

	cmd := &cobra.Command{
		Use:   "excel-to-csv",
		Short: "Convert Excel files into CSV cell-value hints",
		Run: func(cmd *cobra.Command, args []string) {
			v, err := config.SetupViper(cmd, configPath, "doc_convert_excel_csv")
			if err != nil {
				_, _ = os.Stderr.WriteString(err.Error() + "\n")
				exitcode.ExitWithError(err)
			}
			cfg := config.ReadCommon(v)
			mode, err := commonio.ResolveOutputMode(cfg.OutputMode, cfg.PrintJSON)
			if err != nil {
				_, _ = os.Stderr.WriteString(err.Error() + "\n")
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
				logger.Error("input validation failed", "error", err)
				exitcode.ExitWithError(err)
			}
			results := make([]string, 0, len(inputs))
			for _, input := range inputs {
				logger.Debug("converting excel file to csv", "input", input, "output_dir", outputDir, "backend", backend, "sheets", sheets)
				artifact, convErr := entext.ConvertExcelToCSVWithOptions(ctx, entext.FileJob{
					InputPath: input,
					OutputDir: outputDir,
				}, entext.ExcelCSVOptions{
					Backend: backend,
					Sheets:  sheets,
				})
				if convErr != nil {
					logger.Error("conversion failed", "input", input, "error", convErr)
					exitcode.ExitWithError(convErr)
				}
				results = append(results, artifact.Paths...)
			}
			if err := commonio.WriteResultPaths(os.Stdout, mode, results); err != nil {
				logger.Error("failed to write output", "error", err)
				exitcode.ExitWithError(err)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&inputPath, "input", "i", "", "Input Excel file path")
	flags.BoolVar(&useStdin, "stdin", false, "Read input paths from stdin")
	flags.StringVarP(&outputDir, "output-dir", "o", ".", "Output directory")
	flags.StringVar(&backend, "backend", "auto", "Backend: auto|libreoffice|excel-com")
	flags.StringVar(&sheets, "sheets", "", "Comma-separated 1-based sheet indices (e.g. 1,3,5)")
	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flags.BoolVar(&quiet, "quiet", false, "Suppress info logs")
	flags.StringVar(&outputMode, "output-mode", "path", "Output mode: path|json")
	flags.BoolVar(&printJSON, "print-json", false, "Alias for --output-mode json")

	if err := cmd.Execute(); err != nil {
		exitcode.ExitWithError(err)
	}
}
