package excelanalyze

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/axsh/entext/internal/common/sheetmap"
	"github.com/axsh/entext/internal/exceltopdf"
	"github.com/axsh/entext/internal/pdftoimage"
)

type PipelineOptions struct {
	PDFBackend   string
	PDFEngine    string
	ImageBackend string
	ImageEngine  string
	DPI          int
	Sheets       string // reserved
}

type ImageRenderer interface {
	Render(ctx context.Context, inputExcel, workDir string, opts PipelineOptions) (pdfPath, sheetMapPath string, images []SheetImage, err error)
}

type DefaultImageRenderer struct{}

func (DefaultImageRenderer) Render(ctx context.Context, inputExcel, workDir string, opts PipelineOptions) (string, string, []SheetImage, error) {
	return RenderTemplateImages(ctx, inputExcel, workDir, opts)
}

func RenderTemplateImages(ctx context.Context, inputExcel, workDir string, opts PipelineOptions) (pdfPath, sheetMapPath string, images []SheetImage, err error) {
	pdfDir := filepath.Join(workDir, "pdf")
	imgDir := filepath.Join(workDir, "images")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		return "", "", nil, err
	}
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		return "", "", nil, err
	}

	pdfBackend := opts.PDFBackend
	if pdfBackend == "" {
		pdfBackend = exceltopdf.BackendAuto
	}
	pdfEngine := opts.PDFEngine
	if pdfEngine == "" {
		pdfEngine = exceltopdf.EngineGoNative
	}
	pdfSvc := exceltopdf.New()
	pdfRes, err := pdfSvc.ConvertWithOptions(ctx, inputExcel, pdfDir, pdfBackend, pdfEngine, nil)
	if err != nil {
		return "", "", nil, fmt.Errorf("excel to pdf: %w", err)
	}

	imgBackend := opts.ImageBackend
	if imgBackend == "" {
		imgBackend = pdftoimage.BackendAuto
	}
	imgEngine := opts.ImageEngine
	if imgEngine == "" {
		imgEngine = pdftoimage.EngineGoNative
	}
	dpi := opts.DPI
	if dpi <= 0 {
		dpi = 200
	}
	imgSvc := pdftoimage.New()
	paths, err := imgSvc.ConvertWithOptions(ctx, pdfRes.PDFPath, imgDir, "png", imgBackend, imgEngine, dpi, pdfRes.SheetMapPath)
	if err != nil {
		return "", "", nil, fmt.Errorf("pdf to image: %w", err)
	}

	names := []string{}
	if pdfRes.SheetMapPath != "" {
		if m, readErr := sheetmap.Read(pdfRes.SheetMapPath); readErr == nil && m != nil {
			names = append(names, m.PageSheetNames...)
		}
	}
	images = make([]SheetImage, 0, len(paths))
	for i, p := range paths {
		name := fmt.Sprintf("Sheet-%d", i+1)
		if i < len(names) && strings.TrimSpace(names[i]) != "" {
			name = names[i]
		}
		images = append(images, SheetImage{SheetName: name, ImagePath: p})
	}
	return pdfRes.PDFPath, pdfRes.SheetMapPath, images, nil
}
