package pdfimage

import (
	"context"

	"github.com/axsh/entext"
)

type Converter struct {
	format       string
	backend      string
	engine       string
	dpi          int
	sheetMapPath string
}

func New(format string) *Converter {
	return &Converter{format: format, backend: "auto", engine: "legacy", dpi: 200}
}

func NewWithBackend(format string, backend string) *Converter {
	if backend == "" {
		backend = "auto"
	}
	return &Converter{format: format, backend: backend, engine: "legacy", dpi: 200}
}

type ConverterOptions struct {
	Backend      string
	Engine       string
	DPI          int
	SheetMapPath string
}

func NewWithOptions(format string, opts ConverterOptions) *Converter {
	backend := opts.Backend
	if backend == "" {
		backend = "auto"
	}
	engine := opts.Engine
	if engine == "" {
		engine = "legacy"
	}
	dpi := opts.DPI
	if dpi <= 0 {
		dpi = 200
	}
	return &Converter{
		format:       format,
		backend:      backend,
		engine:       engine,
		dpi:          dpi,
		sheetMapPath: opts.SheetMapPath,
	}
}

func (c *Converter) Convert(ctx context.Context, inputPath string, outputDir string) ([]string, error) {
	artifact, err := entext.ConvertPDFToImageWithOptions(ctx, entext.FileJob{
		InputPath: inputPath,
		OutputDir: outputDir,
	}, c.format, entext.PDFImageOptions{
		Backend:      c.backend,
		Engine:       c.engine,
		DPI:          c.dpi,
		SheetMapPath: c.sheetMapPath,
	})
	if err != nil {
		return nil, err
	}
	return artifact.Paths, nil
}
