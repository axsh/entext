package exceltopdf

import (
	"context"
	"fmt"

	backenderr "github.com/axsh/entext/internal/common/backend"
	"github.com/axsh/entext/internal/common/sheetmap"
)

type Backend interface {
	Name() string
	Convert(ctx context.Context, input string, outputDir string) (string, error)
}

const (
	BackendAuto        = "auto"
	BackendLibreOffice = "libreoffice"
	BackendExcelCOM    = "excel-com"

	EngineLegacy   = "legacy"
	EngineGoNative = "go-native"
)

type Service struct {
	goos string
}

type ConvertResult struct {
	PDFPath      string
	SheetMapPath string
	SheetMap     sheetmap.SheetMap
}

func New() *Service {
	return &Service{goos: runtimeGOOS()}
}

func (s *Service) Convert(ctx context.Context, input string, outputDir string, mode string) (string, error) {
	result, err := s.ConvertWithOptions(ctx, input, outputDir, mode, EngineLegacy, nil)
	if err != nil {
		return "", err
	}
	return result.PDFPath, nil
}

func (s *Service) ConvertWithOptions(ctx context.Context, input string, outputDir string, mode string, engine string, sheets []int) (ConvertResult, error) {
	if engine == "" {
		engine = EngineLegacy
	}
	if engine == EngineGoNative {
		return s.convertGoNative(ctx, input, outputDir, sheets)
	}
	chain, err := resolveBackendChain(mode, s.goos)
	if err != nil {
		return ConvertResult{}, err
	}
	out, err := runChain(ctx, chain, input, outputDir)
	if err != nil {
		return ConvertResult{}, err
	}
	m := sheetmap.SheetMap{
		SheetEntries: []sheetmap.SheetEntry{
			{
				SheetIndex:   1,
				SheetName:    "Sheet1",
				ExportStatus: "success",
				PageCount:    1,
			},
		},
		PageSheetNames: []string{"Sheet1"},
	}
	smPath, err := sheetmap.Write(out, input, m)
	if err != nil {
		return ConvertResult{}, err
	}
	return ConvertResult{
		PDFPath:      out,
		SheetMapPath: smPath,
		SheetMap:     m,
	}, nil
}

func runChain(ctx context.Context, chain []Backend, input string, outputDir string) (string, error) {
	attempts := make([]backenderr.AttemptError, 0, len(chain))
	for _, backend := range chain {
		out, convErr := backend.Convert(ctx, input, outputDir)
		if convErr == nil {
			return out, nil
		}
		attempts = append(attempts, backenderr.AttemptError{
			Backend: backend.Name(),
			Err:     convErr,
		})
	}
	return "", backenderr.NewAggregateError(attempts)
}

func (s *Service) convertGoNative(ctx context.Context, input string, outputDir string, sheets []int) (ConvertResult, error) {
	backend := NewGoNativeBackend()
	pdfPath, m, err := backend.Convert(ctx, input, outputDir, sheets)
	if err != nil {
		return ConvertResult{}, err
	}
	smPath, err := sheetmap.Write(pdfPath, input, m)
	if err != nil {
		return ConvertResult{}, err
	}
	return ConvertResult{
		PDFPath:      pdfPath,
		SheetMapPath: smPath,
		SheetMap:     m,
	}, nil
}

func resolveBackendChain(mode string, goos string) ([]Backend, error) {
	switch mode {
	case "", BackendAuto:
		if goos == "windows" {
			return []Backend{
				NewExcelCOMBackend(),
				NewLibreOfficeBackend(),
			}, nil
		}
		return []Backend{NewLibreOfficeBackend()}, nil
	case BackendExcelCOM:
		return []Backend{NewExcelCOMBackend()}, nil
	case BackendLibreOffice:
		return []Backend{NewLibreOfficeBackend()}, nil
	default:
		return nil, fmt.Errorf("unsupported backend mode: %s", mode)
	}
}
