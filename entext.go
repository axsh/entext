package entext

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/exceltopdf"
	"github.com/axsh/entext/internal/exceltocsv"
	"github.com/axsh/entext/internal/imagetomd/analyzer"
	"github.com/axsh/entext/internal/imagetomd/converter"
	"github.com/axsh/entext/internal/imagetomd/csvhint"
	"github.com/axsh/entext/internal/imagetomd/refresolver"
	"github.com/axsh/entext/internal/imagetomd/tern"
	"github.com/axsh/entext/internal/pdftoimage"
)

type FileJob struct {
	InputPath string
	OutputDir string
}

type FileArtifact struct {
	Paths        []string
	SheetMapPath string
}

type ExcelPDFOptions struct {
	Backend string
	Engine  string
	Sheets  string
}

type ExcelCSVOptions struct {
	Backend string
	Sheets  string
}

type PDFImageOptions struct {
	Backend      string
	Engine       string
	DPI          int
	SheetMapPath string
}

type ImageToMarkdownConfig struct {
	ServerURL       string
	Agent           string
	Model           string
	TernMode        string
	TernConfigPath  string
	Verbose         bool
	Quiet           bool
	StrictGapJudge  bool
	SaveQuestionLog bool
	RoundSleepMS    int
	PhaseSleepMS    int
	MaxRounds       int
}

type ImageToMarkdownJob struct {
	InputPath     string
	OutputPath    string
	OutputDir     string
	RefPatterns   []string
	CsvHintPaths  []string
	NoCsvHintAuto bool
}

type MarkdownArtifact struct {
	MarkdownPath string
	SessionPath  string
}

var (
	ErrInvalidArgs   = errors.New("invalid arguments")
	ErrInputRequired = errors.New("input is required")
)

type ValidationError struct {
	Err error
}

func (e *ValidationError) Error() string {
	if e == nil || e.Err == nil {
		return ErrInvalidArgs.Error()
	}
	return e.Err.Error()
}

func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func IsValidation(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}

func DefaultImageToMarkdownConfig() ImageToMarkdownConfig {
	return ImageToMarkdownConfig{
		ServerURL:       "http://localhost:3100",
		Agent:           "codex",
		Model:           "gpt-5.3-codex",
		TernMode:        "auto",
		StrictGapJudge:  false,
		SaveQuestionLog: true,
		RoundSleepMS:    5000,
		PhaseSleepMS:    5000,
		MaxRounds:       5,
	}
}

func ConvertExcelToPDF(ctx context.Context, job FileJob) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	svc := exceltopdf.New()
	out, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, exceltopdf.BackendAuto, exceltopdf.EngineLegacy, nil)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: []string{out.PDFPath}, SheetMapPath: out.SheetMapPath}, nil
}

func ConvertExcelToPDFWithBackend(ctx context.Context, job FileJob, backend string) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	if !isValidExcelBackend(backend) {
		return FileArtifact{}, newValidation("excel backend must be auto, libreoffice, or excel-com")
	}
	svc := exceltopdf.New()
	out, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, backend, exceltopdf.EngineLegacy, nil)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: []string{out.PDFPath}, SheetMapPath: out.SheetMapPath}, nil
}

func ConvertExcelToPDFWithOptions(ctx context.Context, job FileJob, opts ExcelPDFOptions) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	if !isValidExcelBackend(opts.Backend) {
		return FileArtifact{}, newValidation("excel backend must be auto, libreoffice, or excel-com")
	}
	if !isValidEngine(opts.Engine) {
		return FileArtifact{}, newValidation("engine must be go-native or legacy")
	}
	indices, err := exceltopdf.ParseSheetIndices(opts.Sheets)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	svc := exceltopdf.New()
	out, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, opts.Backend, opts.Engine, indices)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: []string{out.PDFPath}, SheetMapPath: out.SheetMapPath}, nil
}

func ConvertExcelToCSV(ctx context.Context, job FileJob) (FileArtifact, error) {
	return ConvertExcelToCSVWithOptions(ctx, job, ExcelCSVOptions{Backend: exceltocsv.BackendAuto})
}

func ConvertExcelToCSVWithOptions(ctx context.Context, job FileJob, opts ExcelCSVOptions) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	if !isValidExcelBackend(opts.Backend) {
		return FileArtifact{}, newValidation("excel backend must be auto, libreoffice, or excel-com")
	}
	indices, err := exceltocsv.ParseSheetIndices(opts.Sheets)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	svc := exceltocsv.New()
	out, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, opts.Backend, indices)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: out.CSVPaths}, nil
}

func ConvertPDFToImage(ctx context.Context, job FileJob, format string) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	if format != "png" && format != "jpg" {
		return FileArtifact{}, newValidation("format must be png or jpg")
	}
	svc := pdftoimage.New()
	outs, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, format, pdftoimage.BackendAuto, pdftoimage.EngineLegacy, 200, "")
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: outs}, nil
}

func ConvertPDFToImageWithBackend(ctx context.Context, job FileJob, format string, backend string) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	if format != "png" && format != "jpg" {
		return FileArtifact{}, newValidation("format must be png or jpg")
	}
	if !isValidPDFBackend(backend) {
		return FileArtifact{}, newValidation("pdf backend must be auto, pdftoppm, or magick")
	}
	svc := pdftoimage.New()
	outs, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, format, backend, pdftoimage.EngineLegacy, 200, "")
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: outs}, nil
}

func ConvertPDFToImageWithOptions(ctx context.Context, job FileJob, format string, opts PDFImageOptions) (FileArtifact, error) {
	if job.InputPath == "" {
		return FileArtifact{}, newValidation("input_path is required")
	}
	if job.OutputDir == "" {
		return FileArtifact{}, newValidation("output_dir is required")
	}
	if format != "png" && format != "jpg" && format != "jpeg" {
		return FileArtifact{}, newValidation("format must be png or jpg")
	}
	if !isValidPDFBackend(opts.Backend) {
		return FileArtifact{}, newValidation("pdf backend must be auto, pdftoppm, or magick")
	}
	if !isValidEngine(opts.Engine) {
		return FileArtifact{}, newValidation("engine must be go-native or legacy")
	}
	if opts.DPI <= 0 {
		opts.DPI = 200
	}
	svc := pdftoimage.New()
	outs, err := svc.ConvertWithOptions(ctx, job.InputPath, job.OutputDir, format, opts.Backend, opts.Engine, opts.DPI, opts.SheetMapPath)
	if err != nil {
		return FileArtifact{}, wrapError(err)
	}
	return FileArtifact{Paths: outs}, nil
}

func ConvertImageToMarkdown(ctx context.Context, job ImageToMarkdownJob, cfg ImageToMarkdownConfig) (MarkdownArtifact, error) {
	if job.InputPath == "" {
		return MarkdownArtifact{}, newValidation("input_path is required")
	}
	if job.OutputPath == "" && job.OutputDir == "" {
		return MarkdownArtifact{}, newValidation("output_path or output_dir is required")
	}
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:3100"
	}
	if cfg.Agent == "" {
		cfg.Agent = "codex"
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-5.3-codex"
	}
	if cfg.TernMode == "" {
		cfg.TernMode = string(tern.ModeAuto)
	}
	switch cfg.TernMode {
	case string(tern.ModeAuto), string(tern.ModeExternal), string(tern.ModeInProc):
	default:
		return MarkdownArtifact{}, newValidation("tern mode must be auto, external, or inproc")
	}
	if cfg.RoundSleepMS <= 0 {
		cfg.RoundSleepMS = 5000
	}
	if cfg.PhaseSleepMS <= 0 {
		cfg.PhaseSleepMS = 5000
	}
	if cfg.MaxRounds <= 0 {
		cfg.MaxRounds = 5
	}

	refs, err := refresolver.ResolveRefs(job.RefPatterns, ".")
	if err != nil {
		return MarkdownArtifact{}, wrapError(err)
	}
	csvHints, err := csvhint.ResolveCsvHints(job.CsvHintPaths, job.InputPath, job.NoCsvHintAuto)
	if err != nil {
		return MarkdownArtifact{}, wrapError(err)
	}
	runtime, err := tern.BuildRuntime(ctx, tern.RuntimeRequest{
		Mode:           tern.Mode(cfg.TernMode),
		ExternalServer: cfg.ServerURL,
		ConfigPath:     cfg.TernConfigPath,
		Agent:          cfg.Agent,
		Model:          cfg.Model,
		WorkingDir:     ".",
	})
	if err != nil {
		return MarkdownArtifact{}, wrapError(err)
	}
	defer func() { _ = runtime.Shutdown(context.Background()) }()
	if cfg.Verbose && !cfg.Quiet {
		_, _ = fmt.Fprintf(
			os.Stderr,
			"[image-to-markdown] step=runtime_ready image=%s mode=%s endpoint=%s\n",
			job.InputPath,
			runtime.ModeUsed,
			runtime.Endpoint,
		)
	}
	client := runtime.Client
	basename := converter.BasenameFromImage(job.InputPath)
	targetMD := job.OutputPath
	if targetMD == "" {
		targetMD = filepath.Join(job.OutputDir, basename+".md")
	}
	sessionDir := filepath.Join(filepath.Dir(targetMD), "_sessions")
	if err := os.MkdirAll(filepath.Dir(targetMD), 0o755); err != nil {
		return MarkdownArtifact{}, err
	}
	an := analyzer.New(client, cfg.Agent, cfg.Model, analyzer.AnalyzeOptions{
		StrictGapJudge:  cfg.StrictGapJudge,
		SaveQuestionLog: cfg.SaveQuestionLog,
		RoundSleepMS:    cfg.RoundSleepMS,
		PhaseSleepMS:    cfg.PhaseSleepMS,
		MaxRounds:       cfg.MaxRounds,
		SessionPersist: func(log *analyzer.SessionLog) error {
			return log.Save(sessionDir, basename)
		},
		Progress: func(format string, args ...any) {
			if !cfg.Verbose || cfg.Quiet {
				return
			}
			_, _ = fmt.Fprintf(os.Stderr, "[image-to-markdown] "+format+"\n", args...)
		},
	})
	md, sessionLog, err := an.Analyze(ctx, job.InputPath, ".", refs, csvHints)
	if err != nil {
		return MarkdownArtifact{}, wrapError(err)
	}

	if err := os.WriteFile(targetMD, []byte(md), 0o644); err != nil {
		return MarkdownArtifact{}, err
	}
	if err := sessionLog.Save(sessionDir, basename); err != nil {
		return MarkdownArtifact{}, err
	}
	return MarkdownArtifact{
		MarkdownPath: targetMD,
		SessionPath:  filepath.Join(sessionDir, basename+"_session.json"),
	}, nil
}

func isValidExcelBackend(backend string) bool {
	return backend == "" ||
		backend == exceltopdf.BackendAuto ||
		backend == exceltopdf.BackendLibreOffice ||
		backend == exceltopdf.BackendExcelCOM
}

func isValidPDFBackend(backend string) bool {
	return backend == "" ||
		backend == pdftoimage.BackendAuto ||
		backend == pdftoimage.BackendPDFToPPM ||
		backend == pdftoimage.BackendMagick
}

func isValidEngine(engine string) bool {
	return engine == "" ||
		engine == exceltopdf.EngineLegacy ||
		engine == exceltopdf.EngineGoNative
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}
	if apperr.IsValidationError(err) {
		return &ValidationError{Err: fmt.Errorf("%w: %w", ErrInvalidArgs, err)}
	}
	return err
}

func newValidation(format string, args ...any) error {
	return &ValidationError{
		Err: apperr.NewValidationError(
			fmt.Errorf("%w: %s", apperr.ErrInvalidArgs, fmt.Sprintf(format, args...)),
		),
	}
}
