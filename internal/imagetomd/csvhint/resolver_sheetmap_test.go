package csvhint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindSheetMapNearImage(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pdfDir := filepath.Join(root, "pdf")
	imageDir := filepath.Join(root, "images")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.MkdirAll(imageDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	sidecar := filepath.Join(pdfDir, "book.sheet-map.json")
	if err := os.WriteFile(sidecar, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	image := filepath.Join(imageDir, "01_変更履歴.png")
	got, ok := findSheetMapFile(image)
	if !ok || got != sidecar {
		t.Fatalf("findSheetMapFile got %q ok=%t", got, ok)
	}
}

func TestResolveSheetIndexFromImageBasename(t *testing.T) {
	t.Parallel()
	sm := &struct {
		PageSheetNames []string
		SheetEntries   []struct {
			SheetIndex int
			SheetName  string
		}
	}{
		PageSheetNames: []string{"変更履歴", "書き換えルール"},
		SheetEntries: []struct {
			SheetIndex int
			SheetName  string
		}{
			{SheetIndex: 1, SheetName: "変更履歴"},
			{SheetIndex: 2, SheetName: "書き換えルール"},
		},
	}
	data, _ := json.Marshal(sm)
	var parsed struct {
		PageSheetNames []string `json:"page_sheet_names"`
		SheetEntries   []struct {
			SheetIndex int    `json:"sheet_index"`
			SheetName  string `json:"sheet_name"`
		} `json:"sheet_entries"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pageIdx, label, ok := parseImagePageBasename("01_変更履歴.png")
	if !ok || pageIdx != 0 || label != "変更履歴" {
		t.Fatalf("parseImagePageBasename got idx=%d label=%q ok=%t", pageIdx, label, ok)
	}
}

func TestResolveCsvPathFromSheetMap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pdfDir := filepath.Join(root, "pdf")
	csvDir := filepath.Join(root, "csv")
	imageDir := filepath.Join(root, "images")
	for _, d := range []string{pdfDir, csvDir, imageDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	xlsx := filepath.Join(root, "workbook.xlsm")
	sidecar := filepath.Join(pdfDir, "workbook.sheet-map.json")
	mapData := map[string]any{
		"version":        1,
		"source_xlsx":    xlsx,
		"page_sheet_names": []string{"変更履歴"},
		"sheet_entries": []map[string]any{
			{"sheet_index": 1, "sheet_name": "変更履歴", "export_status": "success", "page_count": 1},
		},
	}
	raw, _ := json.Marshal(mapData)
	if err := os.WriteFile(sidecar, raw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	csvPath := filepath.Join(csvDir, "workbook.sheet-1.csv")
	if err := os.WriteFile(csvPath, []byte("a,b\n1,2"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	image := filepath.Join(imageDir, "01_変更履歴.png")
	got, idx, name, ok := resolveSheetMapCsvPath(image)
	if !ok || got != csvPath || idx != 1 || name != "変更履歴" {
		t.Fatalf("resolveSheetMapCsvPath got path=%q idx=%d name=%q ok=%t", got, idx, name, ok)
	}
}

func TestResolveCsvHintsSheetMapAuto(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pdfDir := filepath.Join(root, "pdf")
	csvDir := filepath.Join(root, "csv")
	imageDir := filepath.Join(root, "images")
	for _, d := range []string{pdfDir, csvDir, imageDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	xlsx := filepath.Join(root, "workbook.xlsm")
	sidecar := filepath.Join(pdfDir, "workbook.sheet-map.json")
	mapData := map[string]any{
		"version":        1,
		"source_xlsx":    xlsx,
		"page_sheet_names": []string{"変更履歴"},
		"sheet_entries": []map[string]any{
			{"sheet_index": 1, "sheet_name": "変更履歴", "export_status": "success", "page_count": 1},
		},
	}
	raw, _ := json.Marshal(mapData)
	if err := os.WriteFile(sidecar, raw, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	csvPath := filepath.Join(csvDir, "workbook.sheet-1.csv")
	if err := os.WriteFile(csvPath, []byte("scoped,csv"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	image := filepath.Join(imageDir, "01_変更履歴.png")
	hints, err := ResolveCsvHints(nil, image, false)
	if err != nil {
		t.Fatalf("ResolveCsvHints: %v", err)
	}
	if len(hints) != 1 || hints[0].Path != csvPath || hints[0].SheetIndex != 1 {
		t.Fatalf("unexpected hints: %#v", hints)
	}
	if !strings.Contains(hints[0].Content, "scoped") {
		t.Fatalf("unexpected content: %s", hints[0].Content)
	}
}

func TestResolveCsvHintsExplicitOverridesSheetMap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	pdfDir := filepath.Join(root, "pdf")
	csvDir := filepath.Join(root, "csv")
	imageDir := filepath.Join(root, "images")
	for _, d := range []string{pdfDir, csvDir, imageDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
	}
	sidecar := filepath.Join(pdfDir, "workbook.sheet-map.json")
	if err := os.WriteFile(sidecar, []byte(`{"version":1,"source_xlsx":"w.xlsm","page_sheet_names":["変更履歴"],"sheet_entries":[{"sheet_index":1,"sheet_name":"変更履歴"}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	autoCsv := filepath.Join(csvDir, "workbook.sheet-1.csv")
	if err := os.WriteFile(autoCsv, []byte("auto"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	explicit := filepath.Join(root, "manual.csv")
	if err := os.WriteFile(explicit, []byte("manual"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	image := filepath.Join(imageDir, "01_変更履歴.png")
	hints, err := ResolveCsvHints([]string{explicit}, image, false)
	if err != nil {
		t.Fatalf("ResolveCsvHints: %v", err)
	}
	if len(hints) != 1 || hints[0].Path != explicit {
		t.Fatalf("expected explicit hint only: %#v", hints)
	}
}
