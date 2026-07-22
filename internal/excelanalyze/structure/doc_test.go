package structure

import (
	"strings"
	"testing"
	"time"

	"github.com/axsh/entext/internal/excelcell"
)

func TestRenderContainsRequiredSections(t *testing.T) {
	doc := Document{
		Version:    MarkdownVersion,
		SourcePath: "tmpl.xlsx",
		AnalyzedAt: time.Date(2026, 7, 22, 0, 0, 0, 0, time.UTC),
		Backend:    "test",
		HintsUsed:  true,
		Sheets: []Sheet{{
			Name:     "Sheet1",
			Index:    1,
			Overview: "form",
			Semantic: "labels on left",
			Fields: []Field{{
				ID:    "name",
				Label: "Name",
				Role:  "input",
				Cells: []string{"B1"},
			}},
			Notes: []string{"keep short"},
		}},
	}
	md := Render(doc)
	required := []string{
		"# Excel Template Structure",
		"## Metadata",
		"## Sheet: Sheet1",
		"### Overview",
		"### Semantic Structure",
		"### Cell Mapping",
		"### Edit Notes",
	}
	for _, s := range required {
		if !strings.Contains(md, s) {
			t.Fatalf("missing %q in:\n%s", s, md)
		}
	}
}

func TestRenderFillFieldCellTable(t *testing.T) {
	md := Render(Document{
		Version: MarkdownVersion,
		Sheets: []Sheet{{
			Name:  "S",
			Index: 1,
			Fields: []Field{{
				ID: "age", Label: "Age", Role: "input", Cells: []string{"B2"},
			}},
		}},
	})
	if !strings.Contains(md, "| field_id | label | sheet | cells | role |") {
		t.Fatal("missing table header")
	}
	if !strings.Contains(md, "| age | Age | S | B2 | input |") {
		t.Fatalf("missing row:\n%s", md)
	}
}

func TestAttachCellSnapshotsAddsRawAndFields(t *testing.T) {
	doc := &Document{Version: MarkdownVersion}
	err := AttachCellSnapshots(doc, []excelcell.SheetSnapshot{{
		Name:  "Sheet1",
		Index: 1,
		Cells: []excelcell.Cell{
			{Sheet: "Sheet1", Ref: "A1", Value: "Name"},
			{Sheet: "Sheet1", Ref: "B1", Value: ""},
		},
		MergeRanges: []string{"A3:B3"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Sheets) != 1 {
		t.Fatalf("sheets=%d", len(doc.Sheets))
	}
	md := Render(*doc)
	if !strings.Contains(md, "A1") || !strings.Contains(md, "Name") {
		t.Fatalf("raw cells missing:\n%s", md)
	}
	if !strings.Contains(md, "A3:B3") {
		t.Fatalf("merge missing:\n%s", md)
	}
}

func TestMergeSemantic(t *testing.T) {
	doc := &Document{Sheets: []Sheet{{Name: "Sheet1", Index: 1}}}
	MergeSemantic(doc, "Sheet1", "section header")
	if doc.Sheets[0].Semantic != "section header" {
		t.Fatalf("got %q", doc.Sheets[0].Semantic)
	}
}
