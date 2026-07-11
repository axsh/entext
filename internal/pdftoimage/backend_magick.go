package pdftoimage

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/axsh/entext/internal/common/pathutil"
)

type MagickBackend struct{}

func NewMagickBackend() *MagickBackend {
	return &MagickBackend{}
}

func (b *MagickBackend) Name() string {
	return BackendMagick
}

func (b *MagickBackend) Convert(ctx context.Context, inputPDF string, outputDir string, format string) ([]string, error) {
	base := pathutil.BaseNameWithoutExt(inputPDF)
	prefix := filepath.Join(outputDir, base)
	targetPattern := prefix + "-%03d." + format
	cmd := exec.CommandContext(
		ctx,
		"magick",
		"-density",
		"200",
		inputPDF,
		targetPattern,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("magick conversion failed: %w: %s", err, string(out))
	}
	return normalizeOutputs(inputPDF, outputDir, format, prefix+"-*."+format)
}
