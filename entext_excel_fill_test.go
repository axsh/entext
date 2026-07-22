package entext

import (
	"context"
	"testing"
)

func TestFillExcelValidation(t *testing.T) {
	_, err := FillExcel(context.Background(), ExcelFillJob{}, ExcelFillConfig{})
	if err == nil || !IsValidation(err) {
		t.Fatalf("expected validation, got %v", err)
	}
}
