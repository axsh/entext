//go:build !windows

package exceltocsv

import (
	"context"
	"fmt"
)

type ExcelCOMBackend struct{}

func NewExcelCOMBackend() *ExcelCOMBackend {
	return &ExcelCOMBackend{}
}

func (b *ExcelCOMBackend) Name() string {
	return BackendExcelCOM
}

func (b *ExcelCOMBackend) ConvertSheets(_ context.Context, _, _ string, _ []int) ([]string, error) {
	return nil, fmt.Errorf("excel-com backend requires windows with microsoft excel")
}
