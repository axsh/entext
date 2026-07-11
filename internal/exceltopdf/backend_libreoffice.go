package exceltopdf

import (
	"context"
	"fmt"
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

func (b *LibreOfficeBackend) Convert(ctx context.Context, input string, outputDir string) (string, error) {
	cmd := exec.CommandContext(
		ctx,
		"libreoffice",
		"--headless",
		"--convert-to",
		"pdf",
		"--outdir",
		outputDir,
		input,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("libreoffice conversion failed: %w: %s", err, string(out))
	}
	return filepath.Join(outputDir, pathutil.BaseNameWithoutExt(input)+".pdf"), nil
}
