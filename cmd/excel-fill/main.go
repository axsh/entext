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
		templatePath    string
		structurePath   string
		outputPath      string
		refPatterns     []string
		refDirs         []string
		prompts         []string
		promptFiles     []string
		mode            string
		maxRetries      int
		continueRetries int
		configPath      string
		serverURL       string
		ternMode        string
		ternConfigPath  string
		agentName       string
		modelName       string
		backend         string
		engine          string
		imageBackend    string
		imageEngine     string
		dpi             int
		verbose         bool
		quiet           bool
		outputMode      string
		printJSON       bool
	)

	cmd := &cobra.Command{
		Use:   "excel-fill",
		Short: "Interactively fill an Excel template using structure markdown",
		Run: func(cmd *cobra.Command, args []string) {
			v, err := config.SetupViper(cmd, configPath, "doc_excel_fill")
			if err != nil {
				exitcode.ExitWithError(err)
			}
			cfg := config.ReadCommon(v)
			outMode, err := commonio.ResolveOutputMode(cfg.OutputMode, cfg.PrintJSON)
			if err != nil {
				exitcode.ExitWithError(err)
			}
			logger := commonio.NewLogger(os.Stderr, cfg.Verbose, cfg.Quiet)

			if strings.TrimSpace(templatePath) == "" {
				exitcode.ExitWithError(apperr.NewValidationError(fmt.Errorf("--template is required")))
			}
			if strings.TrimSpace(structurePath) == "" {
				exitcode.ExitWithError(apperr.NewValidationError(fmt.Errorf("--structure is required")))
			}
			if strings.TrimSpace(outputPath) == "" {
				exitcode.ExitWithError(apperr.NewValidationError(fmt.Errorf("-o/--output is required")))
			}

			logger.Debug("filling excel template", "template", templatePath, "structure", structurePath, "output", outputPath, "mode", mode)
			artifact, err := entext.FillExcel(ctx, entext.ExcelFillJob{
				TemplatePath:    templatePath,
				StructurePath:   structurePath,
				OutputPath:      outputPath,
				RefPatterns:     refPatterns,
				RefDirs:         refDirs,
				Prompts:         prompts,
				PromptFiles:     promptFiles,
				Mode:            mode,
				MaxRetries:      maxRetries,
				ContinueRetries: continueRetries,
			}, entext.ExcelFillConfig{
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
				logger.Error("fill failed", "error", err)
				exitcode.ExitWithError(err)
			}
			if err := commonio.WriteResultPaths(os.Stdout, outMode, []string{artifact.OutputPath}); err != nil {
				logger.Error("failed to write output", "error", err)
				exitcode.ExitWithError(err)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&templatePath, "template", "", "Template Excel path")
	flags.StringVar(&structurePath, "structure", "", "Structure markdown from excel-template-analyze")
	flags.StringVarP(&outputPath, "output", "o", "", "Output filled Excel path")
	flags.StringSliceVar(&refPatterns, "ref", nil, "Reference markdown regexp (repeatable)")
	flags.StringSliceVar(&refDirs, "ref-dir", nil, "Reference markdown directory (repeatable)")
	flags.StringSliceVar(&prompts, "prompt", nil, "Additional prompt (repeatable)")
	flags.StringSliceVar(&promptFiles, "prompt-file", nil, "Additional prompt file (repeatable)")
	flags.StringVar(&mode, "mode", "text", "Dialog mode: text|json")
	flags.IntVar(&maxRetries, "max-retries", 5, "Max visual verification failures before continue confirm")
	flags.IntVar(&continueRetries, "continue-retries", 0, "If >0, auto-continue with this many extra retries when exhausted")
	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.StringVar(&serverURL, "server-url", "http://localhost:3100", "Tern server URL")
	flags.StringVar(&ternMode, "tern-mode", "auto", "Tern mode: auto|external|inproc")
	flags.StringVar(&ternConfigPath, "tern-config", "", "Tern config path")
	flags.StringVar(&agentName, "agent", "codex", "Agent name")
	flags.StringVar(&modelName, "model", "gpt-5.3-codex", "Model name")
	flags.StringVar(&backend, "backend", "auto", "Excel PDF backend")
	flags.StringVar(&engine, "engine", "go-native", "Excel PDF engine")
	flags.StringVar(&imageBackend, "image-backend", "auto", "PDF image backend")
	flags.StringVar(&imageEngine, "image-engine", "go-native", "PDF image engine")
	flags.IntVar(&dpi, "dpi", 200, "Image DPI")
	flags.BoolVar(&verbose, "verbose", false, "Verbose logging")
	flags.BoolVar(&quiet, "quiet", false, "Quiet logging")
	flags.StringVar(&outputMode, "output-mode", "path", "Output mode: path|json")
	flags.BoolVar(&printJSON, "print-json", false, "Alias for --output-mode json")

	if err := cmd.Execute(); err != nil {
		exitcode.ExitWithError(err)
	}
}
