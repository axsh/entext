package analyzer

import (
	"strings"
	"testing"

	"github.com/axsh/entext/internal/imagetomd/csvhint"
)

func TestBuildCsvHintContextContainsUsagePolicy(t *testing.T) {
	t.Parallel()
	got := buildCsvHintContext([]csvhint.CsvHint{{Path: "a.csv", Content: "col1,col2\n1,2"}})
	for _, want := range []string{
		"[Reference csv hint]",
		"大量の行・列データ",
		"CSV の該当セル値を参照して転記してよい",
		"表の構造",
		"Vision",
		"--- CSV content ---",
		"col1,col2",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in context:\n%s", want, got)
		}
	}
}

func TestPhase2CsvExecuteAppendWhenHintsPresent(t *testing.T) {
	t.Parallel()
	got := phase2CsvExecuteAppend([]csvhint.CsvHint{{Path: "a.csv", Content: "x"}})
	for _, want := range []string{
		"Phase 2 追加指示",
		"上記 CSV から転記してよい",
		"Markdown テーブル形式",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestCsvFinalSynthesisAppend(t *testing.T) {
	t.Parallel()
	got := csvFinalSynthesisAppend([]csvhint.CsvHint{{Path: "a.csv", Content: "x"}})
	for _, want := range []string{
		"CSV 参照により取得したセル値",
		"画像の表構造を超えない",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestBuildCsvHintContextEmptyReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := buildCsvHintContext(nil); got != "" {
		t.Fatalf("expected empty context, got %q", got)
	}
	if got := phase2CsvExecuteAppend(nil); got != "" {
		t.Fatalf("expected empty phase2 append, got %q", got)
	}
	if got := csvFinalSynthesisAppend(nil); got != "" {
		t.Fatalf("expected empty final append, got %q", got)
	}
}
