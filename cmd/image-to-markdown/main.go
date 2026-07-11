package main

import (
	"context"
	"fmt"
	"os"

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
		inputPath       string
		useStdin        bool
		outputPath      string
		outputDir       string
		configPath      string
		serverURL       string
		ternMode        string
		ternConfigPath  string
		agentName       string
		modelName       string
		refPatterns     []string
		csvHintPaths    []string
		noCsvHintAuto   bool
		strictGapJudge  bool
		saveQuestionLog bool
		roundSleepMS    int
		phaseSleepMS    int
		maxRounds       int
		verbose         bool
		quiet           bool
		outputMode      string
		printJSON       bool
	)

	cmd := &cobra.Command{
		Use:   "image-to-markdown",
		Short: "Analyze image and convert it to markdown",
		Run: func(cmd *cobra.Command, args []string) {
			v, err := config.SetupViper(cmd, configPath, "doc_convert_image_md")
			if err != nil {
				exitcode.ExitWithError(err)
			}
			cfg := config.ReadCommon(v)
			mode, err := commonio.ResolveOutputMode(cfg.OutputMode, cfg.PrintJSON)
			if err != nil {
				exitcode.ExitWithError(err)
			}
			logger := commonio.NewLogger(os.Stderr, cfg.Verbose, cfg.Quiet)
			if err := commonio.ValidateStdinReady(useStdin, os.Stdin); err != nil {
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
			if err := validateOutputContract(inputs, useStdin, outputPath, outputDir); err != nil {
				exitcode.ExitWithError(err)
			}

			convertConfig := entext.ImageToMarkdownConfig{
				ServerURL:       serverURL,
				TernMode:        ternMode,
				TernConfigPath:  ternConfigPath,
				Agent:           agentName,
				Model:           modelName,
				Verbose:         verbose,
				Quiet:           quiet,
				StrictGapJudge:  strictGapJudge,
				SaveQuestionLog: saveQuestionLog,
				RoundSleepMS:    roundSleepMS,
				PhaseSleepMS:    phaseSleepMS,
				MaxRounds:       maxRounds,
			}

			results := make([]string, 0, len(inputs))
			for _, input := range inputs {
				logger.Debug("starting image analysis", "input", input, "server", serverURL, "model", modelName)
				job := entext.ImageToMarkdownJob{
					InputPath:     input,
					OutputPath:    outputPath,
					OutputDir:     outputDir,
					RefPatterns:   refPatterns,
					CsvHintPaths:  csvHintPaths,
					NoCsvHintAuto: noCsvHintAuto,
				}
				if useStdin {
					job.OutputPath = ""
				}
				artifact, analyzeErr := entext.ConvertImageToMarkdown(ctx, job, convertConfig)
				if analyzeErr != nil {
					_, _ = os.Stderr.WriteString(analyzeErr.Error() + "\n")
					exitcode.ExitWithError(analyzeErr)
				}
				results = append(results, artifact.MarkdownPath)
			}
			if err := commonio.WriteResultPaths(os.Stdout, mode, results); err != nil {
				exitcode.ExitWithError(err)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(&inputPath, "input", "i", "", "Input image path")
	flags.BoolVar(&useStdin, "stdin", false, "Read input paths from stdin")
	flags.StringVarP(&outputPath, "output", "o", "", "Output markdown file path")
	flags.StringVar(&outputDir, "output-dir", ".", "Output directory for markdown files")
	flags.StringVar(&configPath, "config", "", "Config file path")
	flags.StringVar(&serverURL, "server", "http://localhost:3100", "Tern server URL")
	flags.StringVar(&ternMode, "tern-mode", "auto", "Tern runtime mode: auto|external|inproc")
	flags.StringVar(&ternConfigPath, "tern-config", "", "Tern in-process config file path")
	flags.StringVar(&agentName, "agent", "codex", "Agent name")
	flags.StringVar(&modelName, "model", "gpt-5.3-codex", "Model name")
	flags.StringSliceVarP(&refPatterns, "ref", "r", nil, "Reference markdown regex pattern (repeatable)")
	flags.StringSliceVar(&csvHintPaths, "csv-hint", nil, "Reference CSV hint path (repeatable)")
	flags.BoolVar(&noCsvHintAuto, "no-csv-hint-auto", false, "Disable automatic CSV hint resolution")
	flags.BoolVar(&strictGapJudge, "strict-gap-judge", false, "Enable strict SUFFICIENT judgment")
	flags.BoolVar(&saveQuestionLog, "save-question-log", true, "Store generated question in session log")
	flags.IntVar(&roundSleepMS, "round-sleep-ms", 5000, "Sleep between question generation and answer")
	flags.IntVar(&phaseSleepMS, "phase-sleep-ms", 5000, "Sleep between phases")
	flags.IntVar(&maxRounds, "max-rounds", 5, "Maximum rounds for each phase")
	flags.BoolVar(&verbose, "verbose", false, "Enable verbose logging")
	flags.BoolVar(&quiet, "quiet", false, "Suppress info logs")
	flags.StringVar(&outputMode, "output-mode", "path", "Output mode: path|json")
	flags.BoolVar(&printJSON, "print-json", false, "Alias for --output-mode json")

	if err := cmd.Execute(); err != nil {
		exitcode.ExitWithError(err)
	}
}

func validateOutputContract(_ []string, useStdin bool, outputPath string, outputDir string) error {
	if useStdin {
		if outputPath != "" {
			return apperr.NewValidationError(fmt.Errorf("--output cannot be used with --stdin"))
		}
		if outputDir == "" {
			return apperr.NewValidationError(fmt.Errorf("--output-dir is required when --stdin is set"))
		}
		return nil
	}
	if outputPath == "" && outputDir == "" {
		return apperr.NewValidationError(fmt.Errorf("--output or --output-dir is required"))
	}
	return nil
}
