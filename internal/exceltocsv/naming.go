package exceltocsv

import (
	"fmt"
	"path/filepath"

	"github.com/axsh/entext/internal/common/pathutil"
)

func csvOutputPath(outputDir, inputPath string, sheetIndex int) string {
	base := pathutil.BaseNameWithoutExt(inputPath)
	name := fmt.Sprintf("%s.sheet-%d.csv", base, sheetIndex)
	return filepath.Join(outputDir, name)
}
