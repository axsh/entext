package csvhint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
)

type CsvHint struct {
	Path    string
	Content string
}

func ResolveCsvHints(explicitPaths []string, imagePath string, disableAuto bool) ([]CsvHint, error) {
	hints := make([]CsvHint, 0, len(explicitPaths))
	for _, raw := range explicitPaths {
		path := filepath.Clean(raw)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, apperr.NewValidationError(fmt.Errorf("%w: csv hint not found: %s", apperr.ErrInvalidArgs, path))
		}
		hints = append(hints, CsvHint{
			Path:    path,
			Content: string(data),
		})
	}
	if len(explicitPaths) > 0 || disableAuto {
		return hints, nil
	}
	autoPath, ok := resolveAutoPath(imagePath)
	if !ok {
		return hints, nil
	}
	data, err := os.ReadFile(autoPath)
	if err != nil {
		return hints, nil
	}
	return append(hints, CsvHint{
		Path:    autoPath,
		Content: string(data),
	}), nil
}

func resolveAutoPath(imagePath string) (string, bool) {
	base := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
	imageDir := filepath.Dir(imagePath)
	candidates := []string{
		filepath.Join(imageDir, base+".csv"),
		filepath.Join(imageDir, "csv", base+".csv"),
		filepath.Join(imageDir, "..", "csv", base+".csv"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}
