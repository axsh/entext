package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/axsh/entext"
	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/common/config"
	"github.com/axsh/entext/internal/common/exitcode"
	commonio "github.com/axsh/entext/internal/common/io"
	"github.com/spf13/cobra"
)

func main() {
	ctx := context.Background()
	var (
		inputPath      string
		outputPath     string
		outputDir      string
		keepWorkDir    string
		refPatterns    []string
		refDirs        []string
		prompts        []string
		promptFiles    []string
		configPath     string
		serverURL      string
		ternMode       string
		ternConfigPath string
		agentName      string
		modelName      string
		backend        string
		engine         string
		imageBackend   string
		imageEngine    string
		dpi            int
		verbose        bool
		quiet          bool
		outputMode     string
		printJSON      bool
	)

	cmd := &cobra.Command{
		Use:   "excel-template-analyze",
		Short: "Analyze Excel template into structure markdown",
		Run: func(cmd *cobra.Command, args []string) {
			v, err := config.SetupViper(cmd, configPath, "doc_excel_template_analyze")
			if err != nil {
				exitcode.ExitWithError(err)
			}
			cfg := config.ReadCommon(v)
			mode, err := commonio.ResolveOutputMode(cfg.OutputMode, cfg.PrintJSON)
			if err != nil {
				exitcode.ExitWithError(err)
			}
			logger := commonio.NewLogger(os.Stderr, cfg.Verbose, cfg.Quiet)

			if strings.TrimSpace(inputPath) == "" {
				exitcode.ExitWithError(apperr.NewValidationError(fmt.Errorf("-i/--input is required")))
			}
			out := strings.TrimSpace(outputPath)
			if out == "" && strings.TrimSpace(outputDir) == "" {
				exitcode.ExitWithError(apperr.NewValidationError(fmt.Errorf("--output or --output-dir is required")))
			}

			logger.Debug("analyzing excel template", "input", inputPath, "output", out, "output_dir", outputDir)
			artifact, err := entext.AnalyzeExcelTemplate(ctx, entext.ExcelTemplateAnalyzeJob{
				InputPath:   inputPath,
				OutputPath:  out,
				OutputDir:   outputDir,
				KeepWorkDir: keepWorkDir,
				RefPatterns: refPatterns,
				RefDirs:     refDirs,
				Prompts:     prompts,
				PromptFiles: promptFiles,
			}, entext.ExcelTemplateAnalyzeConfig{
				ServerURL:      serverURL,
				TernMode:       ternMode,
				TernConfigPath: ternConfigPath,
				Agent:          agentName,
				Model:          modelName,
				PDFBackend:     backend,
				PDFEngine:      engine,
				ImageBackend:   imageBackend,
				ImageEngine:    imageEngine,
				DPI:            dpi,
				Verbose:        verbose,
				Quiet:          quiet,
			})
			if err != nil {
				logger.Error("analyze failed", "error", err)
				exitcode.ExitWithError(err)
			}
			if err := commonio.WriteResultPaths(os.Stdout, mode, []string{artifact.StructurePath}); err != nil {
				logger.Error("failed to write output", "error", err)
				exitcode.ExitWithError(err)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&inputPath, "input", "i", "", "Input Excel template path")
	flags.StringVarP(&outputPath, "output", "o", "", "Output structure markdown path")
	flags.StringVar(&outputDir, "output-dir", "", "Output directory (writes <basename>.structure.md)")
	flags.StringVar(&keepWorkDir, "keep-work-dir", "", "Keep intermediate PDF/images in this directory")
	flags.StringSliceVar(&refPatterns, "ref", nil, "Reference markdown regexp (repeatable)")
	flags.StringSliceVar(&refDirs, "ref-dir", nil, "Reference markdown directory (repeatable, recursive)")
	flags.StringSliceVar(&prompts, "prompt", nil, "Additional prompt hint (repeatable)")
	flags.StringSliceVar(&promptFiles, "prompt-file", nil, "Additional prompt file (repeatable)")
	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.StringVar(&serverURL, "server-url", "http://localhost:3100", "Tern server URL")
	flags.StringVar(&ternMode, "tern-mode", "auto", "Tern mode: auto|external|inproc")
	flags.StringVar(&ternConfigPath, "tern-config", "", "Tern config path")
	flags.StringVar(&agentName, "agent", "codex", "Agent name")
	flags.StringVar(&modelName, "model", "gpt-5.3-codex", "Model name")
	flags.StringVar(&backend, "backend", "auto", "Excel PDF backend: auto|libreoffice|excel-com")
	flags.StringVar(&engine, "engine", "go-native", "Excel PDF engine: go-native|legacy")
	flags.StringVar(&imageBackend, "image-backend", "auto", "PDF image backend: auto|pdftoppm|magick")
	flags.StringVar(&imageEngine, "image-engine", "go-native", "PDF image engine: go-native|legacy")
	flags.IntVar(&dpi, "dpi", 200, "Image DPI")
	flags.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flags.BoolVar(&quiet, "quiet", false, "Suppress info logs")
	flags.StringVar(&outputMode, "output-mode", "path", "Output mode: path|json")
	flags.BoolVar(&printJSON, "print-json", false, "Alias for --output-mode json")

	if err := cmd.Execute(); err != nil {
		exitcode.ExitWithError(err)
	}
}
