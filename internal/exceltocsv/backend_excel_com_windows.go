//go:build windows

package exceltocsv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

func (b *ExcelCOMBackend) ConvertSheets(ctx context.Context, input, outputDir string, sheets []int) ([]string, error) {
	absInput, err := filepath.Abs(input)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, err
	}
	absOutputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return nil, err
	}
	basename := pathutil.BaseNameWithoutExt(input)
	sheetFilter := buildSheetFilterLiteral(sheets)
	ps := strings.Join([]string{
		"$ErrorActionPreference='Stop'",
		"$excel=New-Object -ComObject Excel.Application",
		"$excel.DisplayAlerts=$false",
		"$excel.Visible=$false",
		fmt.Sprintf("$wb=$excel.Workbooks.Open('%s')", escapePowerShellSingleQuoted(absInput)),
		fmt.Sprintf("$outputDir='%s'", escapePowerShellSingleQuoted(absOutputDir)),
		fmt.Sprintf("$basename='%s'", escapePowerShellSingleQuoted(basename)),
		fmt.Sprintf("$sheetFilter=@(%s)", sheetFilter),
		"$paths=@()",
		"$idx=1",
		"foreach ($ws in $wb.Worksheets) {",
		"  if ($sheetFilter.Count -gt 0 -and ($sheetFilter -notcontains $idx)) { $idx++; continue }",
		fmt.Sprintf("  $outPath=Join-Path $outputDir ($basename + '.sheet-' + $idx + '.csv')"),
		"  $ws.SaveAs($outPath, 62)",
		"  $paths += $outPath",
		"  $idx++",
		"}",
		"$wb.Close($false)",
		"$excel.Quit()",
		"[System.Runtime.Interopservices.Marshal]::ReleaseComObject($wb) | Out-Null",
		"[System.Runtime.Interopservices.Marshal]::ReleaseComObject($excel) | Out-Null",
		"$paths -join [Environment]::NewLine",
	}, "\n")
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("excel-com csv conversion failed: %w: %s", err, string(out))
	}
	paths := parseOutputPaths(string(out))
	if len(paths) == 0 {
		return nil, fmt.Errorf("excel-com csv conversion produced no output files")
	}
	for _, path := range paths {
		if err := ensureUTF8BOM(path); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

func buildSheetFilterLiteral(sheets []int) string {
	if len(sheets) == 0 {
		return ""
	}
	parts := make([]string, len(sheets))
	for i, n := range sheets {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func escapePowerShellSingleQuoted(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

func parseOutputPaths(raw string) []string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths
}
