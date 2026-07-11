package exceltocsv

import "testing"

func TestParseSheetIndicesDelegatesToExcelToPDF(t *testing.T) {
	t.Parallel()
	got, err := ParseSheetIndices("1,3")
	if err != nil {
		t.Fatalf("ParseSheetIndices failed: %v", err)
	}
	if len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Fatalf("unexpected indices: %#v", got)
	}
	empty, err := ParseSheetIndices("")
	if err != nil {
		t.Fatalf("empty ParseSheetIndices failed: %v", err)
	}
	if empty != nil {
		t.Fatalf("expected nil slice for empty input, got %#v", empty)
	}
}
