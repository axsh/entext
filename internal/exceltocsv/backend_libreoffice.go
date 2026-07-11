package exceltocsv

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/axsh/entext/internal/common/pathutil"
)

type LibreOfficeBackend struct{}

func NewLibreOfficeBackend() *LibreOfficeBackend {
	return &LibreOfficeBackend{}
}

func (b *LibreOfficeBackend) Name() string {
	return BackendLibreOffice
}

func (b *LibreOfficeBackend) ConvertSheets(ctx context.Context, input, outputDir string, sheets []int) ([]string, error) {
	if err := validateLibreOfficeSheets(sheets); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(
		ctx,
		"libreoffice",
		"--headless",
		"--convert-to",
		"csv",
		"--outdir",
		outputDir,
		input,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("libreoffice csv conversion failed: %w: %s", err, string(out))
	}
	loDefault := filepath.Join(outputDir, pathutil.BaseNameWithoutExt(input)+".csv")
	target := csvOutputPath(outputDir, input, 1)
	if err := os.Rename(loDefault, target); err != nil {
		return nil, fmt.Errorf("libreoffice csv rename failed: %w", err)
	}
	if err := ensureUTF8BOM(target); err != nil {
		return nil, err
	}
	return []string{target}, nil
}

func validateLibreOfficeSheets(sheets []int) error {
	if len(sheets) == 0 {
		return nil
	}
	if len(sheets) > 1 {
		return fmt.Errorf("libreoffice backend supports single-sheet export in v1; use excel-com on windows for multi-sheet")
	}
	if sheets[0] != 1 {
		return fmt.Errorf("libreoffice backend supports single-sheet export in v1; use excel-com on windows for multi-sheet")
	}
	return nil
}
