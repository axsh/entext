package sheetmap

import (
	"encoding/json"
	"testing"
)

func TestSheetMapJSONContract(t *testing.T) {
	t.Parallel()
	in := SheetMap{
		Version:        Version,
		SourceXLSX:     "a.xlsx",
		PDFPath:        "a.pdf",
		PageSheetNames: []string{"S1", "S2", "S2"},
		SheetEntries: []SheetEntry{
			{SheetIndex: 1, SheetName: "S1", ExportStatus: "success", PageCount: 1},
			{SheetIndex: 2, SheetName: "S2", ExportStatus: "success", PageCount: 2},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if raw["version"] != float64(Version) {
		t.Fatalf("unexpected version: %v", raw["version"])
	}
	if _, ok := raw["sheet_entries"]; !ok {
		t.Fatalf("sheet_entries is missing")
	}

	total := 0
	for _, e := range in.SheetEntries {
		if e.ExportStatus == "success" {
			total += e.PageCount
		}
	}
	if len(in.PageSheetNames) != total {
		t.Fatalf("page_sheet_names length mismatch: got %d want %d", len(in.PageSheetNames), total)
	}
}
