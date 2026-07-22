package excelfill

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/common/refprompt"
	"github.com/axsh/entext/internal/excelanalyze"
	"github.com/axsh/entext/internal/excelcell"
	"github.com/axsh/entext/internal/excelfill/dialog"
)

type Options struct {
	TemplatePath    string
	StructurePath   string
	OutputPath      string
	Hints           refprompt.HintInput
	Mode            string
	MaxRetries      int
	ContinueRetries int
	Transport       dialog.Transport
	Filler          Filler
	Visual          VisualChecker
	Renderer        excelanalyze.ImageRenderer
	WorkDir         string
	Pipeline        excelanalyze.PipelineOptions
	Verbose         bool
	Logf            func(format string, args ...any)
}

type Result struct {
	OutputPath  string
	RetriesUsed int
	LastIssues  []dialog.VisualIssue
	Aborted     bool
}

func Fill(ctx context.Context, opts Options) (Result, error) {
	if strings.TrimSpace(opts.TemplatePath) == "" {
		return Result{}, apperr.NewValidationError(fmt.Errorf("template path is required"))
	}
	if strings.TrimSpace(opts.StructurePath) == "" {
		return Result{}, apperr.NewValidationError(fmt.Errorf("structure path is required"))
	}
	if strings.TrimSpace(opts.OutputPath) == "" {
		return Result{}, apperr.NewValidationError(fmt.Errorf("output path is required"))
	}
	if opts.Filler == nil || opts.Visual == nil || opts.Transport == nil {
		return Result{}, apperr.NewValidationError(fmt.Errorf("filler, visual, and transport are required"))
	}
	maxRetries := opts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 5
	}
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}

	structureMD, err := os.ReadFile(opts.StructurePath)
	if err != nil {
		return Result{}, apperr.NewValidationError(fmt.Errorf("structure file: %w", err))
	}
	hintBundle, err := refprompt.Resolve(opts.Hints)
	if err != nil {
		return Result{}, err
	}
	hintText := refprompt.FormatForPrompt(hintBundle, refprompt.ModeFill)

	workDir := strings.TrimSpace(opts.WorkDir)
	if workDir == "" {
		workDir, err = os.MkdirTemp("", "excel-fill-*")
		if err != nil {
			return Result{}, err
		}
		defer func() { _ = os.RemoveAll(workDir) }()
	} else if err := os.MkdirAll(workDir, 0o755); err != nil {
		return Result{}, err
	}

	workXLSX := filepath.Join(workDir, "filled.xlsx")
	if err := excelcell.CopyFile(opts.TemplatePath, workXLSX); err != nil {
		return Result{}, err
	}

	answers := map[string]string{}
	history := []dialog.Message{}
	var lastIssues []dialog.VisualIssue
	retriesUsed := 0
	renderer := opts.Renderer
	if renderer == nil {
		renderer = excelanalyze.DefaultImageRenderer{}
	}

	for {
		fc := FillContext{
			StructureMD:    string(structureMD),
			HintText:       hintText,
			Answers:        answers,
			History:        history,
			VisualFeedback: lastIssues,
		}
		questions, writes, err := opts.Filler.Plan(ctx, fc)
		if err != nil {
			return Result{}, err
		}
		if len(questions) > 0 {
			q := dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeQuestion, Prompt: "Additional information required", Fields: questions}
			if err := opts.Transport.Send(ctx, q); err != nil {
				return Result{}, err
			}
			history = append(history, q)
			ans, err := opts.Transport.Receive(ctx)
			if err != nil {
				return Result{}, err
			}
			history = append(history, ans)
			for k, v := range ans.Values {
				answers[k] = v
			}
			if ans.Text != "" && len(ans.Values) == 0 {
				answers["_text"] = ans.Text
			}
			continue
		}
		if len(writes) == 0 {
			return Result{}, fmt.Errorf("filler returned no writes")
		}

		_ = opts.Transport.Send(ctx, dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeStatus, Status: "writing_cells"})
		if err := applyWrites(workXLSX, writes); err != nil {
			return Result{}, err
		}

		_ = opts.Transport.Send(ctx, dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeStatus, Status: "visual_check"})
		_, _, images, err := renderer.Render(ctx, workXLSX, filepath.Join(workDir, "visual"), opts.Pipeline)
		if err != nil {
			return Result{}, fmt.Errorf("visual render: %w", err)
		}
		paths := make([]string, 0, len(images))
		for _, img := range images {
			paths = append(paths, img.ImagePath)
		}
		issues, err := opts.Visual.Check(ctx, paths, hintText)
		if err != nil {
			return Result{}, err
		}
		if len(issues) == 0 {
			if err := excelcell.CopyFile(workXLSX, opts.OutputPath); err != nil {
				return Result{}, err
			}
			done := dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeDone, OutputPath: opts.OutputPath, RetriesUsed: retriesUsed}
			_ = opts.Transport.Send(ctx, done)
			return Result{OutputPath: opts.OutputPath, RetriesUsed: retriesUsed}, nil
		}

		lastIssues = issues
		_ = opts.Transport.Send(ctx, dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeVisualIssue, Issues: issues})
		retriesUsed++
		logf("visual issues found count=%d retries_used=%d max=%d", len(issues), retriesUsed, maxRetries)

		if retriesUsed < maxRetries {
			continue
		}

		// Exhausted.
		prompt := fmt.Sprintf("Reached max retries (%d). Continue?", maxRetries)
		_ = opts.Transport.Send(ctx, dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeContinueConfirm, Prompt: prompt, Issues: issues})
		additional := opts.ContinueRetries
		if additional <= 0 {
			dec, err := opts.Transport.Receive(ctx)
			if err != nil {
				return Result{}, err
			}
			if dec.Continue == nil || !*dec.Continue {
				_ = opts.Transport.Send(ctx, dialog.Message{Role: dialog.RoleAssistant, Type: dialog.TypeError, Error: "aborted after visual retries", Issues: issues, OutputPath: workXLSX})
				return Result{OutputPath: workXLSX, RetriesUsed: retriesUsed, LastIssues: issues, Aborted: true}, fmt.Errorf("excel fill aborted after %d visual retries", retriesUsed)
			}
			if dec.AdditionalRetries != nil && *dec.AdditionalRetries > 0 {
				additional = *dec.AdditionalRetries
			} else {
				additional = maxRetries
			}
		}
		maxRetries += additional
		logf("continuing with max_retries=%d", maxRetries)
	}
}

func applyWrites(path string, writes []CellWrite) error {
	wb, err := excelcell.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = wb.Close() }()
	for _, w := range writes {
		if err := wb.SetCellValue(w.Sheet, w.Cell, w.Value); err != nil {
			return err
		}
	}
	return wb.SaveAs(path)
}
