package entext

import (
	"context"
	"testing"
)

func TestAnalyzeExcelTemplateValidation(t *testing.T) {
	_, err := AnalyzeExcelTemplate(context.Background(), ExcelTemplateAnalyzeJob{}, ExcelTemplateAnalyzeConfig{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !IsValidation(err) {
		t.Fatalf("expected IsValidation, got %T %v", err, err)
	}

	_, err = AnalyzeExcelTemplate(context.Background(), ExcelTemplateAnalyzeJob{
		InputPath: "x.xlsx",
	}, ExcelTemplateAnalyzeConfig{})
	if err == nil || !IsValidation(err) {
		t.Fatalf("expected output validation, got %v", err)
	}
}
