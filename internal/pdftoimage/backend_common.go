package pdftoimage

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/axsh/entext/internal/common/pathutil"
)

func normalizeOutputs(inputPDF string, outputDir string, format string, pattern string) ([]string, error) {
	base := pathutil.BaseNameWithoutExt(inputPDF)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	paths := make([]string, 0, len(matches))
	for i, src := range matches {
		dst := filepath.Join(outputDir, fmt.Sprintf("%s_%03d.%s", base, i+1, format))
		if err := os.Rename(src, dst); err != nil {
			return nil, err
		}
		paths = append(paths, dst)
	}
	return paths, nil
}
