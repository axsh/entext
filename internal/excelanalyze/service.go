package excelanalyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/common/refprompt"
	"github.com/axsh/entext/internal/excelanalyze/structure"
	"github.com/axsh/entext/internal/excelcell"
)

type Options struct {
	InputPath   string
	OutputPath  string
	KeepWorkDir string
	Hints       refprompt.HintInput
	Pipeline    PipelineOptions
	Semantic    SemanticAnalyzer
	Renderer    ImageRenderer // nil => DefaultImageRenderer
	Verbose     bool
	Logf        func(format string, args ...any)
}

type Result struct {
	StructurePath string
	WorkDir       string
}

func Analyze(ctx context.Context, opts Options) (Result, error) {
	if strings.TrimSpace(opts.InputPath) == "" {
		return Result{}, apperr.NewValidationError(fmt.Errorf("input path is required"))
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return Result{}, apperr.NewValidationError(fmt.Errorf("output path is required"))
	}
	if opts.Semantic == nil {
		return Result{}, apperr.NewValidationError(fmt.Errorf("semantic analyzer is required"))
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	hintBundle, err := refprompt.Resolve(opts.Hints)
	if err != nil {
		return Result{}, err
	}
	hintText := refprompt.FormatForPrompt(hintBundle, refprompt.ModeAnalyze)
	logf("resolved hints refs=%d prompt_chars=%d", len(hintBundle.Refs), len(hintBundle.Prompts))

	workDir := strings.TrimSpace(opts.KeepWorkDir)
	cleanup := false
	if workDir == "" {
		workDir, err = os.MkdirTemp("", "excel-template-analyze-*")
		if err != nil {
			return Result{}, err
		}
		cleanup = true
	} else if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Result{}, err
	}
	if cleanup {
		defer func() { _ = os.RemoveAll(workDir) }()
	}

	renderer := opts.Renderer
	if renderer == nil {
		renderer = DefaultImageRenderer{}
	}
	logf("rendering template images input=%s work_dir=%s", opts.InputPath, workDir)
	pdfPath, sheetMapPath, images, err := renderer.Render(ctx, opts.InputPath, workDir, opts.Pipeline)
	if err != nil {
		return Result{}, err
	}
	logf("rendered pdf=%s sheet_map=%s images=%d", pdfPath, sheetMapPath, len(images))

	semanticBySheet, err := opts.Semantic.AnalyzeSheets(ctx, images, hintText)
	if err != nil {
		return Result{}, err
	}

	wb, err := excelcell.Open(opts.InputPath)
	if err != nil {
		return Result{}, fmt.Errorf("open workbook: %w", err)
	}
	defer func() { _ = wb.Close() }()
	snaps, err := wb.Snapshots()
	if err != nil {
		return Result{}, err
	}

	doc := structure.Document{
		Version:    structure.MarkdownVersion,
		SourcePath: opts.InputPath,
		AnalyzedAt: time.Now().UTC(),
		Backend:    fmt.Sprintf("pdf=%s/%s image=%s/%s", opts.Pipeline.PDFBackend, opts.Pipeline.PDFEngine, opts.Pipeline.ImageBackend, opts.Pipeline.ImageEngine),
		HintsUsed:  len(hintBundle.Refs) > 0 || hintBundle.Prompts != "",
	}
	for name, sem := range semanticBySheet {
		structure.MergeSemantic(&doc, name, sem)
	}
	if err := structure.AttachCellSnapshots(&doc, snaps); err != nil {
		return Result{}, err
	}
	// Re-apply semantic in case AttachCellSnapshots added sheets first via empty merge path.
	for name, sem := range semanticBySheet {
		structure.MergeSemantic(&doc, name, sem)
	}

	md := structure.Render(doc)
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return Result{}, err
	}
	if err := os.WriteFile(opts.OutputPath, []byte(md), 0o644); err != nil {
		return Result{}, err
	}
	logf("wrote structure markdown path=%s chars=%d", opts.OutputPath, len(md))

	resultWork := workDir
	if cleanup {
		resultWork = ""
	}
	return Result{StructurePath: opts.OutputPath, WorkDir: resultWork}, nil
}
