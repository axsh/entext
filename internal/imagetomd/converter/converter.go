package converter

import (
	"os"
	"path/filepath"

	"github.com/axsh/entext/internal/common/pathutil"
)

func BasenameFromImage(imagePath string) string {
	return pathutil.BaseNameWithoutExt(imagePath)
}

func WriteMarkdown(outputDir string, basename string, markdown string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(outputDir, basename+".md")
	if err := os.WriteFile(target, []byte(markdown), 0o644); err != nil {
		return "", err
	}
	return target, nil
}
