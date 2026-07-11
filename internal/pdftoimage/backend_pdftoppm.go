package pdftoimage

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/axsh/entext/internal/common/pathutil"
)

type PDFToPPMBackend struct{}

func NewPDFToPPMBackend() *PDFToPPMBackend {
	return &PDFToPPMBackend{}
}

func (b *PDFToPPMBackend) Name() string {
	return BackendPDFToPPM
}

func (b *PDFToPPMBackend) Convert(ctx context.Context, inputPDF string, outputDir string, format string) ([]string, error) {
	base := pathutil.BaseNameWithoutExt(inputPDF)
	prefix := filepath.Join(outputDir, base)
	cmd := exec.CommandContext(
		ctx,
		"pdftoppm",
		"-"+format,
		"-forcenum",
		inputPDF,
		prefix,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("pdftoppm conversion failed: %w: %s", err, string(out))
	}
	return normalizeOutputs(inputPDF, outputDir, format, prefix+"-*."+format)
}
