package excelfill

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/excelanalyze"
	"github.com/axsh/entext/internal/excelfill/dialog"
	"github.com/xuri/excelize/v2"
)

type noopRenderer struct{}

func (noopRenderer) Render(ctx context.Context, inputExcel, workDir string, opts excelanalyze.PipelineOptions) (string, string, []excelanalyze.SheetImage, error) {
	_ = os.MkdirAll(workDir, 0o755)
	img := filepath.Join(workDir, "page.png")
	_ = os.WriteFile(img, []byte("png"), 0o644)
	return "", "", []excelanalyze.SheetImage{{SheetName: "Sheet1", ImagePath: img}}, nil
}

func writeTemplate(t *testing.T, path string) string {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	_ = f.SetCellValue(sheet, "A1", "Name")
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return sheet
}

func TestFillSucceedsOnFirstVisualPass(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.xlsx")
	sheet := writeTemplate(t, tmpl)
	structure := filepath.Join(dir, "s.md")
	_ = os.WriteFile(structure, []byte("# Excel Template Structure\n"), 0o644)
	out := filepath.Join(dir, "out.xlsx")

	var stderr bytes.Buffer
	tr := dialog.NewTextTransport(strings.NewReader(""), &stderr)
	res, err := Fill(context.Background(), Options{
		TemplatePath:  tmpl,
		StructurePath: structure,
		OutputPath:    out,
		Transport:     tr,
		Filler:        &StaticFiller{Writes: []CellWrite{{Sheet: sheet, Cell: "B1", Value: "Alice"}}},
		Visual:        &StaticVisual{Sequence: [][]dialog.VisualIssue{nil}},
		Renderer:      noopRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Aborted || res.OutputPath != out {
		t.Fatalf("%+v", res)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatal(err)
	}
}

func TestFillRetriesOnVisualIssuesUntilPass(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.xlsx")
	sheet := writeTemplate(t, tmpl)
	structure := filepath.Join(dir, "s.md")
	_ = os.WriteFile(structure, []byte("# md\n"), 0o644)
	out := filepath.Join(dir, "out.xlsx")
	tr := dialog.NewTextTransport(strings.NewReader(""), &bytes.Buffer{})
	vis := &StaticVisual{Sequence: [][]dialog.VisualIssue{
		{{Kind: "overflow", Description: "too long"}},
		{{Kind: "overflow", Description: "still"}},
		nil,
	}}
	res, err := Fill(context.Background(), Options{
		TemplatePath:  tmpl,
		StructurePath: structure,
		OutputPath:    out,
		MaxRetries:    5,
		Transport:     tr,
		Filler:        &StaticFiller{Writes: []CellWrite{{Sheet: sheet, Cell: "B1", Value: "VeryLongName"}}},
		Visual:        vis,
		Renderer:      noopRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.RetriesUsed != 2 {
		t.Fatalf("retries=%d", res.RetriesUsed)
	}
}

func TestFillAsksContinueWhenMaxRetriesExhausted_Decline(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.xlsx")
	sheet := writeTemplate(t, tmpl)
	structure := filepath.Join(dir, "s.md")
	_ = os.WriteFile(structure, []byte("# md\n"), 0o644)
	out := filepath.Join(dir, "out.xlsx")
	tr := dialog.NewTextTransport(strings.NewReader("no\n"), &bytes.Buffer{})
	_, err := Fill(context.Background(), Options{
		TemplatePath:  tmpl,
		StructurePath: structure,
		OutputPath:    out,
		MaxRetries:    1,
		Transport:     tr,
		Filler:        &StaticFiller{Writes: []CellWrite{{Sheet: sheet, Cell: "B1", Value: "X"}}},
		Visual:        &StaticVisual{Sequence: [][]dialog.VisualIssue{{{Kind: "cutoff", Description: "bad"}}, {{Kind: "cutoff", Description: "bad"}}}},
		Renderer:      noopRenderer{},
	})
	if err == nil {
		t.Fatal("expected abort error")
	}
}

func TestFillAsksContinueWhenMaxRetriesExhausted_Accept(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.xlsx")
	sheet := writeTemplate(t, tmpl)
	structure := filepath.Join(dir, "s.md")
	_ = os.WriteFile(structure, []byte("# md\n"), 0o644)
	out := filepath.Join(dir, "out.xlsx")
	tr := dialog.NewTextTransport(strings.NewReader("yes\n"), &bytes.Buffer{})
	vis := &StaticVisual{Sequence: [][]dialog.VisualIssue{
		{{Kind: "cutoff", Description: "bad"}},
		{{Kind: "cutoff", Description: "bad"}},
		nil,
	}}
	res, err := Fill(context.Background(), Options{
		TemplatePath:    tmpl,
		StructurePath:   structure,
		OutputPath:      out,
		MaxRetries:      1,
		ContinueRetries: 2,
		Transport:       tr,
		Filler:          &StaticFiller{Writes: []CellWrite{{Sheet: sheet, Cell: "B1", Value: "X"}}},
		Visual:          vis,
		Renderer:        noopRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Aborted {
		t.Fatal("should succeed after continue")
	}
}

func TestFillDefaultMaxRetriesIsFive(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.xlsx")
	sheet := writeTemplate(t, tmpl)
	structure := filepath.Join(dir, "s.md")
	_ = os.WriteFile(structure, []byte("# md\n"), 0o644)
	// Always fail visually; preseed continue retries=0 and decline.
	issues := make([][]dialog.VisualIssue, 6)
	for i := range issues {
		issues[i] = []dialog.VisualIssue{{Kind: "overflow", Description: "x"}}
	}
	tr := dialog.NewTextTransport(strings.NewReader("no\n"), &bytes.Buffer{})
	_, err := Fill(context.Background(), Options{
		TemplatePath:  tmpl,
		StructurePath: structure,
		OutputPath:    filepath.Join(dir, "o.xlsx"),
		MaxRetries:    0, // default 5
		Transport:     tr,
		Filler:        &StaticFiller{Writes: []CellWrite{{Sheet: sheet, Cell: "B1", Value: "X"}}},
		Visual:        &StaticVisual{Sequence: issues},
		Renderer:      noopRenderer{},
	})
	if err == nil {
		t.Fatal("expected abort")
	}
}

func TestFillDoesNotModifyTemplateSource(t *testing.T) {
	dir := t.TempDir()
	tmpl := filepath.Join(dir, "t.xlsx")
	sheet := writeTemplate(t, tmpl)
	before, _ := os.ReadFile(tmpl)
	structure := filepath.Join(dir, "s.md")
	_ = os.WriteFile(structure, []byte("# md\n"), 0o644)
	tr := dialog.NewTextTransport(strings.NewReader(""), &bytes.Buffer{})
	_, err := Fill(context.Background(), Options{
		TemplatePath:  tmpl,
		StructurePath: structure,
		OutputPath:    filepath.Join(dir, "o.xlsx"),
		Transport:     tr,
		Filler:        &StaticFiller{Writes: []CellWrite{{Sheet: sheet, Cell: "B1", Value: "Alice"}}},
		Visual:        &StaticVisual{Sequence: [][]dialog.VisualIssue{nil}},
		Renderer:      noopRenderer{},
	})
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(tmpl)
	if string(before) != string(after) {
		t.Fatal("template modified")
	}
}
