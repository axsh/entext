package pdftoimage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	backenderr "github.com/axsh/entext/internal/common/backend"
	"github.com/axsh/entext/internal/common/pathutil"
	"github.com/axsh/entext/internal/common/sheetmap"
)

type Backend interface {
	Name() string
	Convert(ctx context.Context, inputPDF string, outputDir string, format string) ([]string, error)
}

const (
	BackendAuto     = "auto"
	BackendPDFToPPM = "pdftoppm"
	BackendMagick   = "magick"

	EngineLegacy   = "legacy"
	EngineGoNative = "go-native"
)

type Service struct{}

func New() *Service {
	return &Service{}
}

func (s *Service) Convert(ctx context.Context, inputPDF string, outputDir string, format string, mode string) ([]string, error) {
	return s.ConvertWithOptions(ctx, inputPDF, outputDir, format, mode, EngineLegacy, 200, "")
}

func (s *Service) ConvertWithOptions(
	ctx context.Context,
	inputPDF string,
	outputDir string,
	format string,
	mode string,
	engine string,
	dpi int,
	sheetMapPath string,
) ([]string, error) {
	format = normalizeFormat(format)
	if format != "png" && format != "jpg" {
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	if dpi <= 0 {
		dpi = 200
	}
	sm, _ := loadSheetMap(inputPDF, sheetMapPath)
	var err error
	var chain []Backend
	if engine == EngineGoNative {
		chain = []Backend{NewGoFitzBackend(dpi, sm)}
	} else {
		chain, err = resolveBackendChain(mode)
		if err != nil {
			return nil, err
		}
	}
	outs, err := runChain(ctx, chain, inputPDF, outputDir, format)
	if err != nil {
		return nil, err
	}
	if engine == EngineGoNative || sm == nil {
		return outs, nil
	}
	return applySheetMapNaming(outs, outputDir, format, sm)
}

func runChain(ctx context.Context, chain []Backend, inputPDF string, outputDir string, format string) ([]string, error) {
	attempts := make([]backenderr.AttemptError, 0, len(chain))
	for _, backend := range chain {
		out, convErr := backend.Convert(ctx, inputPDF, outputDir, format)
		if convErr == nil {
			return out, nil
		}
		attempts = append(attempts, backenderr.AttemptError{
			Backend: backend.Name(),
			Err:     convErr,
		})
	}
	return nil, backenderr.NewAggregateError(attempts)
}

func normalizeFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "jpg"
	default:
		return strings.ToLower(format)
	}
}

func loadSheetMap(inputPDF string, explicitPath string) (*sheetmap.SheetMap, error) {
	if explicitPath != "" {
		return sheetmap.ReadCompat(explicitPath)
	}
	defaultPath := filepath.Join(filepath.Dir(inputPDF), pathutil.BaseNameWithoutExt(inputPDF)+".sheet-map.json")
	if _, err := os.Stat(defaultPath); err != nil {
		return nil, err
	}
	return sheetmap.ReadCompat(defaultPath)
}

func applySheetMapNaming(paths []string, outputDir string, format string, sm *sheetmap.SheetMap) ([]string, error) {
	renamed := make([]string, 0, len(paths))
	for i, src := range paths {
		dst := filepath.Join(outputDir, BuildOutputName(i, format, sm))
		if err := os.Rename(src, dst); err != nil {
			return nil, err
		}
		renamed = append(renamed, dst)
	}
	return renamed, nil
}

func resolveBackendChain(mode string) ([]Backend, error) {
	switch mode {
	case "", BackendAuto:
		return []Backend{
			NewPDFToPPMBackend(),
			NewMagickBackend(),
		}, nil
	case BackendPDFToPPM:
		return []Backend{NewPDFToPPMBackend()}, nil
	case BackendMagick:
		return []Backend{NewMagickBackend()}, nil
	default:
		return nil, fmt.Errorf("unsupported backend mode: %s", mode)
	}
}
