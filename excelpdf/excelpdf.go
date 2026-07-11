package excelpdf

import (
	"context"

	"github.com/axsh/entext"
)

type Converter struct {
	backend string
	engine  string
	sheets  string
}

func New() *Converter {
	return &Converter{backend: "auto", engine: "legacy"}
}

func NewWithBackend(backend string) *Converter {
	if backend == "" {
		backend = "auto"
	}
	return &Converter{backend: backend, engine: "legacy"}
}

type ConverterOptions struct {
	Backend string
	Engine  string
	Sheets  string
}

func NewWithOptions(opts ConverterOptions) *Converter {
	backend := opts.Backend
	if backend == "" {
		backend = "auto"
	}
	engine := opts.Engine
	if engine == "" {
		engine = "legacy"
	}
	return &Converter{
		backend: backend,
		engine:  engine,
		sheets:  opts.Sheets,
	}
}

func (c *Converter) Convert(ctx context.Context, inputPath string, outputDir string) (string, error) {
	artifact, err := entext.ConvertExcelToPDFWithOptions(ctx, entext.FileJob{
		InputPath: inputPath,
		OutputDir: outputDir,
	}, entext.ExcelPDFOptions{
		Backend: c.backend,
		Engine:  c.engine,
		Sheets:  c.sheets,
	})
	if err != nil {
		return "", err
	}
	if len(artifact.Paths) == 0 {
		return "", nil
	}
	return artifact.Paths[0], nil
}
