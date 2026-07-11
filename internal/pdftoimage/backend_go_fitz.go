package pdftoimage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/axsh/entext/internal/common/sheetmap"
	"github.com/axsh/entext/internal/pdfnative"
)

type GoFitzBackend struct {
	dpi      int
	sheetMap *sheetmap.SheetMap
}

func NewGoFitzBackend(dpi int, sm *sheetmap.SheetMap) *GoFitzBackend {
	if dpi <= 0 {
		dpi = 200
	}
	return &GoFitzBackend{
		dpi:      dpi,
		sheetMap: sm,
	}
}

func (b *GoFitzBackend) Name() string {
	return "go-fitz"
}

func (b *GoFitzBackend) Convert(_ context.Context, inputPDF string, outputDir string, format string) ([]string, error) {
	pages, err := pdfnative.RenderPages(inputPDF, b.dpi, format)
	if err != nil {
		return nil, fmt.Errorf("go-fitz rendering failed: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(pages))
	for i, page := range pages {
		name := BuildOutputName(i, page.Format, b.sheetMap)
		dst := filepath.Join(outputDir, name)
		if err := os.WriteFile(dst, page.Bytes, 0o644); err != nil {
			return nil, err
		}
		paths = append(paths, dst)
	}
	return paths, nil
}
