package pathutil

import (
	"path/filepath"
	"strings"
)

func BaseNameWithoutExt(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}
