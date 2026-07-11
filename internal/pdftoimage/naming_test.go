package pdftoimage

import (
	"testing"

	"github.com/axsh/entext/internal/common/sheetmap"
)

func TestBuildOutputName(t *testing.T) {
	t.Parallel()
	sm := &sheetmap.SheetMap{
		PageSheetNames: []string{"A Sheet", "B/Sheet"},
	}
	if got := BuildOutputName(0, "png", sm); got != "01_A_Sheet.png" {
		t.Fatalf("unexpected name: %s", got)
	}
	if got := BuildOutputName(1, "png", sm); got != "02_BSheet.png" {
		t.Fatalf("unexpected name: %s", got)
	}
	if got := BuildOutputName(2, "png", sm); got != "03_page.png" {
		t.Fatalf("unexpected fallback name: %s", got)
	}
}
