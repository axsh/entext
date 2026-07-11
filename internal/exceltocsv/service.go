package exceltocsv

import (
	"context"
	"fmt"
	"os"

	backenderr "github.com/axsh/entext/internal/common/backend"
)

type Backend interface {
	Name() string
	ConvertSheets(ctx context.Context, input string, outputDir string, sheets []int) ([]string, error)
}

const (
	BackendAuto        = "auto"
	BackendLibreOffice = "libreoffice"
	BackendExcelCOM    = "excel-com"
)

type Service struct {
	goos string
}

type ConvertResult struct {
	CSVPaths []string
}

func New() *Service {
	return &Service{goos: runtimeGOOS()}
}

func (s *Service) ConvertWithOptions(ctx context.Context, input string, outputDir string, mode string, sheets []int) (ConvertResult, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ConvertResult{}, err
	}
	chain, err := resolveBackendChain(mode, s.goos)
	if err != nil {
		return ConvertResult{}, err
	}
	paths, err := runChain(ctx, chain, input, outputDir, sheets)
	if err != nil {
		return ConvertResult{}, err
	}
	return ConvertResult{CSVPaths: paths}, nil
}

func runChain(ctx context.Context, chain []Backend, input, outputDir string, sheets []int) ([]string, error) {
	attempts := make([]backenderr.AttemptError, 0, len(chain))
	for _, backend := range chain {
		out, convErr := backend.ConvertSheets(ctx, input, outputDir, sheets)
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
