//go:build !windows

package exceltopdf

import (
	"context"
	"errors"
)

type ExcelCOMBackend struct{}

func NewExcelCOMBackend() *ExcelCOMBackend {
	return &ExcelCOMBackend{}
}

func (b *ExcelCOMBackend) Name() string {
	return BackendExcelCOM
}

func (b *ExcelCOMBackend) Convert(_ context.Context, _ string, _ string) (string, error) {
	return "", errors.New("excel-com backend is only available on windows")
}
