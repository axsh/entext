package sheetmap

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/axsh/entext/internal/common/pathutil"
)

func Write(pdfPath string, sourceXLSX string, m SheetMap) (string, error) {
	absPDF, err := filepath.Abs(pdfPath)
	if err != nil {
		return "", err
	}
	absSource, err := filepath.Abs(sourceXLSX)
	if err != nil {
		return "", err
	}
	m.Version = Version
	m.SourceXLSX = absSource
	m.PDFPath = absPDF

	sidecar := filepath.Join(filepath.Dir(absPDF), pathutil.BaseNameWithoutExt(absPDF)+".sheet-map.json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(sidecar, data, 0o644); err != nil {
		return "", err
	}
	return sidecar, nil
}

func Read(path string) (*SheetMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m SheetMap
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
