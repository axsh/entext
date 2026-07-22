package excelanalyze

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/common/refprompt"
	"github.com/xuri/excelize/v2"
)

type fakeRenderer struct {
	images []SheetImage
	err    error
	called bool
}

func (f *fakeRenderer) Render(ctx context.Context, inputExcel, workDir string, opts PipelineOptions) (string, string, []SheetImage, error) {
	f.called = true
	if f.err != nil {
		return "", "", nil, f.err
	}
	// Touch work dir so keep-work-dir tests can observe it.
	_ = os.WriteFile(filepath.Join(workDir, "marker.txt"), []byte("ok"), 0o644)
	return filepath.Join(workDir, "out.pdf"), "", f.images, nil
}

type recordingSemantic struct {
	StaticSemantic
	lastHint string
}

func (r *recordingSemantic) AnalyzeSheets(ctx context.Context, images []SheetImage, hintText string) (map[string]string, error) {
	r.lastHint = hintText
	return r.StaticSemantic.AnalyzeSheets(ctx, images, hintText)
}

func writeTestXLSX(t *testing.T, path string) {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "Name")
	_ = f.SetCellValue(sheet, "A2", "Age")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func TestAnalyzeMergesSemanticAndCells(t *testing.T) {
	dir := t.TempDir()
	xlsx := filepath.Join(dir, "t.xlsx")
	writeTestXLSX(t, xlsx)
	out := filepath.Join(dir, "out.structure.md")

	sheetName := "Sheet1"
	f := excelize.NewFile()
	sheetName = f.GetSheetName(0)
	_ = f.Close()

	// Re-open saved file sheet name
	wb, err := excelize.OpenFile(xlsx)
	if err != nil {
		t.Fatal(err)
	}
	sheetName = wb.GetSheetName(0)
	_ = wb.Close()

	sem := &StaticSemantic{Out: map[string]string{sheetName: "Name is a label; B1 is input"}}
	_, err = Analyze(context.Background(), Options{
		InputPath:  xlsx,
		OutputPath: out,
		Semantic:   sem,
		Renderer:   &fakeRenderer{images: []SheetImage{{SheetName: sheetName, ImagePath: "x.png"}}},
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	md := string(raw)
	if !strings.Contains(md, "### Cell Mapping") {
		t.Fatal("missing cell mapping")
	}
	if !strings.Contains(md, "Name is a label") {
		t.Fatal("missing semantic")
	}
	if !strings.Contains(md, "A1") {
		t.Fatal("missing raw cell A1")
	}
}

func TestAnalyzeKeepWorkDirPreservesArtifacts(t *testing.T) {
	dir := t.TempDir()
	xlsx := filepath.Join(dir, "t.xlsx")
	writeTestXLSX(t, xlsx)
	keep := filepath.Join(dir, "work")
	out := filepath.Join(dir, "out.md")
	_, err := Analyze(context.Background(), Options{
		InputPath:   xlsx,
		OutputPath:  out,
		KeepWorkDir: keep,
		Semantic:    &StaticSemantic{},
		Renderer:    &fakeRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(keep, "marker.txt")); err != nil {
		t.Fatalf("expected keep work dir artifact: %v", err)
	}
}

func TestAnalyzeCleansTempWhenKeepWorkDirEmpty(t *testing.T) {
	dir := t.TempDir()
	xlsx := filepath.Join(dir, "t.xlsx")
	writeTestXLSX(t, xlsx)
	out := filepath.Join(dir, "out.md")
	var captured string
	renderer := &fakeRenderer{}
	res, err := Analyze(context.Background(), Options{
		InputPath:  xlsx,
		OutputPath: out,
		Semantic:   &StaticSemantic{},
		Renderer:   renderer,
		Logf: func(format string, args ...any) {
			msg := format
			if strings.Contains(format, "work_dir") {
				captured = filepath.Join(dir) // just mark called
			}
			_ = msg
			_ = args
			_ = captured
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.WorkDir != "" {
		t.Fatalf("expected empty work dir in result, got %q", res.WorkDir)
	}
	if !renderer.called {
		t.Fatal("renderer not called")
	}
}

func TestAnalyzeValidationMissingInput(t *testing.T) {
	_, err := Analyze(context.Background(), Options{OutputPath: "x.md", Semantic: &StaticSemantic{}})
	if err == nil || !apperr.IsValidationError(err) {
		t.Fatalf("expected validation, got %v", err)
	}
}

func TestAnalyzeInjectsHintTextIntoSemantic(t *testing.T) {
	dir := t.TempDir()
	xlsx := filepath.Join(dir, "t.xlsx")
	writeTestXLSX(t, xlsx)
	hintFile := filepath.Join(dir, "hint.txt")
	if err := os.WriteFile(hintFile, []byte("yellow cells only"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := &recordingSemantic{StaticSemantic: StaticSemantic{Out: map[string]string{}}}
	_, err := Analyze(context.Background(), Options{
		InputPath:  xlsx,
		OutputPath: filepath.Join(dir, "o.md"),
		Hints:      refprompt.HintInput{PromptFiles: []string{hintFile}, Prompts: []string{"inline-hint"}},
		Semantic:   rec,
		Renderer:   &fakeRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.lastHint, "yellow cells only") {
		t.Fatalf("hint missing file content: %q", rec.lastHint)
	}
	if !strings.Contains(rec.lastHint, "inline-hint") {
		t.Fatalf("hint missing inline: %q", rec.lastHint)
	}
	if !strings.Contains(rec.lastHint, refprompt.HintPolicyAnalyze) {
		t.Fatal("missing analyze policy")
	}
}
