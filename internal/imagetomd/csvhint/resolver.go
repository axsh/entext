package csvhint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/common/pathutil"
	"github.com/axsh/entext/internal/common/sheetmap"
)

type CsvHint struct {
	Path       string
	Content    string
	SheetIndex int
	SheetName  string
}

var imagePageBasenamePattern = regexp.MustCompile(`^(\d{2})_(.+)\.(png|jpe?g|webp)$`)

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
	autoPath, sheetIndex, sheetName, ok := resolveAutoPath(imagePath)
	if !ok {
		return hints, nil
	}
	data, err := os.ReadFile(autoPath)
	if err != nil {
		return hints, nil
	}
	return append(hints, CsvHint{
		Path:       autoPath,
		Content:    string(data),
		SheetIndex: sheetIndex,
		SheetName:  sheetName,
	}), nil
}

func resolveAutoPath(imagePath string) (path string, sheetIndex int, sheetName string, ok bool) {
	base := strings.TrimSuffix(filepath.Base(imagePath), filepath.Ext(imagePath))
	imageDir := filepath.Dir(imagePath)
	candidates := []string{
		filepath.Join(imageDir, base+".csv"),
		filepath.Join(imageDir, "csv", base+".csv"),
		filepath.Join(imageDir, "..", "csv", base+".csv"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, 0, "", true
		}
	}
	return resolveSheetMapCsvPath(imagePath)
}

func resolveSheetMapCsvPath(imagePath string) (path string, sheetIndex int, sheetName string, ok bool) {
	mapPath, found := findSheetMapFile(imagePath)
	if !found {
		return "", 0, "", false
	}
	data, err := os.ReadFile(mapPath)
	if err != nil {
		return "", 0, "", false
	}
	var sm sheetmap.SheetMap
	if err := json.Unmarshal(data, &sm); err != nil {
		return "", 0, "", false
	}
	pageIndex, sheetLabel, parsed := parseImagePageBasename(imagePath)
	if !parsed {
		return "", 0, "", false
	}
	sheetIndex, sheetName = resolveSheetIndexFromMap(&sm, pageIndex, sheetLabel)
	if sheetIndex <= 0 {
		return "", 0, "", false
	}
	workbookBase := pathutil.BaseNameWithoutExt(sm.SourceXLSX)
	if workbookBase == "" {
		return "", 0, "", false
	}
	imageDir := filepath.Dir(imagePath)
	csvCandidates := []string{
		filepath.Join(imageDir, "..", "csv", fmt.Sprintf("%s.sheet-%d.csv", workbookBase, sheetIndex)),
		filepath.Join(imageDir, "csv", fmt.Sprintf("%s.sheet-%d.csv", workbookBase, sheetIndex)),
		filepath.Join(imageDir, "..", "csv", fmt.Sprintf("%s.sheet-%d.csv", workbookBase, sheetIndex)),
	}
	for _, candidate := range csvCandidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, sheetIndex, sheetName, true
		}
	}
	return "", 0, "", false
}

func findSheetMapFile(imagePath string) (string, bool) {
	imageDir := filepath.Dir(imagePath)
	searchDirs := []string{
		filepath.Join(imageDir, "..", "pdf"),
		filepath.Join(imageDir, ".."),
		imageDir,
	}
	for _, dir := range searchDirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.sheet-map.json"))
		if err != nil || len(matches) == 0 {
			continue
		}
		return matches[0], true
	}
	return "", false
}

func parseImagePageBasename(imagePath string) (pageIndex int, sheetLabel string, ok bool) {
	base := filepath.Base(imagePath)
	m := imagePageBasenamePattern.FindStringSubmatch(base)
	if len(m) != 4 {
		return 0, "", false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, "", false
	}
	return n - 1, m[2], true
}

func resolveSheetIndexFromMap(sm *sheetmap.SheetMap, pageIndex int, sheetLabel string) (sheetIndex int, sheetName string) {
	if sm == nil {
		return 0, ""
	}
	if pageIndex >= 0 && pageIndex < len(sm.PageSheetNames) {
		sheetName = sm.PageSheetNames[pageIndex]
	}
	if sheetName == "" {
		sheetName = sheetLabel
	}
	for _, entry := range sm.SheetEntries {
		if entry.SheetName == sheetName || sheetmap.SanitizeFilename(entry.SheetName) == sheetLabel {
			return entry.SheetIndex, entry.SheetName
		}
	}
	if pageIndex >= 0 && pageIndex < len(sm.SheetEntries) {
		entry := sm.SheetEntries[pageIndex]
		return entry.SheetIndex, entry.SheetName
	}
	return pageIndex + 1, sheetName
}
