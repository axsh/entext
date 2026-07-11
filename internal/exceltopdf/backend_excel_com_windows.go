//go:build windows

package exceltopdf

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/axsh/entext/internal/common/pathutil"
)

type ExcelCOMBackend struct{}

func NewExcelCOMBackend() *ExcelCOMBackend {
	return &ExcelCOMBackend{}
}

func (b *ExcelCOMBackend) Name() string {
	return BackendExcelCOM
}

func (b *ExcelCOMBackend) Convert(ctx context.Context, input string, outputDir string) (string, error) {
	absInput, err := filepath.Abs(input)
	if err != nil {
		return "", err
	}
	target := filepath.Join(outputDir, pathutil.BaseNameWithoutExt(input)+".pdf")
	absOutput, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	ps := strings.Join([]string{
		"$ErrorActionPreference='Stop'",
		"$excel=New-Object -ComObject Excel.Application",
		"$excel.DisplayAlerts=$false",
		"$excel.Visible=$false",
		fmt.Sprintf("$wb=$excel.Workbooks.Open('%s')", strings.ReplaceAll(absInput, "'", "''")),
		fmt.Sprintf("$wb.ExportAsFixedFormat(0,'%s')", strings.ReplaceAll(absOutput, "'", "''")),
		"$wb.Close($false)",
		"$excel.Quit()",
		"[System.Runtime.Interopservices.Marshal]::ReleaseComObject($wb) | Out-Null",
		"[System.Runtime.Interopservices.Marshal]::ReleaseComObject($excel) | Out-Null",
	}, ";")
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("excel-com conversion failed: %w: %s", err, string(out))
	}
	return target, nil
}
