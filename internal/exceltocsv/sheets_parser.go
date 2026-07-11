package exceltocsv

import (
	"github.com/axsh/entext/internal/exceltopdf"
)

func ParseSheetIndices(raw string) ([]int, error) {
	return exceltopdf.ParseSheetIndices(raw)
}
