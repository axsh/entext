package pdftoimage

import (
	"fmt"

	"github.com/axsh/entext/internal/common/sheetmap"
)

func BuildOutputName(pageIndex int, format string, sm *sheetmap.SheetMap) string {
	if sm != nil && pageIndex < len(sm.PageSheetNames) {
		return fmt.Sprintf("%02d_%s.%s", pageIndex+1, sheetmap.SanitizeFilename(sm.PageSheetNames[pageIndex]), format)
	}
	return fmt.Sprintf("%02d_page.%s", pageIndex+1, format)
}
