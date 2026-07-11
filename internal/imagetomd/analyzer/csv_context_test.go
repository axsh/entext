package analyzer

import (
	"strings"
	"testing"

	"github.com/axsh/entext/internal/imagetomd/csvhint"
)

func TestBuildCsvHintContext_ModeFullIncludesContent(t *testing.T) {
	t.Parallel()
	scope := PhaseVisibleScope{VisibleRowIDs: []string{"43", "44"}}
	got := buildCsvHintContext([]csvhint.CsvHint{{Path: "a.csv", Content: ",No.,col\n,43,x\n,1,y"}}, CsvInjectFullContent, scope)
	for _, want := range []string{
		"[Reference csv hint]",
		"【最優先: 出力スコープ】",
		"画像可視スコープ内",
		"--- CSV content ---",
		",43,x",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in context:\n%s", want, got)
		}
	}
	if strings.Contains(got, ",1,y") {
		t.Fatalf("scoped excerpt must not include row 1: %s", got)
	}
}

func TestBuildCsvHintContext_ModePathOnlyOmitsContent(t *testing.T) {
	t.Parallel()
	got := buildCsvHintContext([]csvhint.CsvHint{{Path: "a.csv", Content: "secret,data"}}, CsvInjectPathOnly, PhaseVisibleScope{})
	if strings.Contains(got, "--- CSV content ---") || strings.Contains(got, "secret,data") {
		t.Fatalf("path-only mode must omit CSV body: %s", got)
	}
	if !strings.Contains(got, "Source:") {
		t.Fatalf("path-only mode should include source path: %s", got)
	}
}

func TestBuildCsvHintContext_ModeNoneReturnsEmpty(t *testing.T) {
	t.Parallel()
	got := buildCsvHintContext([]csvhint.CsvHint{{Path: "a.csv", Content: "x"}}, CsvInjectNone, PhaseVisibleScope{})
	if got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildCsvHintContext_IncludesVisibleScopeBlock(t *testing.T) {
	t.Parallel()
	scope := PhaseVisibleScope{VisibleRowIDs: []string{"43", "44"}, ColumnCount: 9}
	got := buildCsvHintContext([]csvhint.CsvHint{{Path: "a.csv", Content: "x"}}, CsvInjectPathOnly, scope)
	if !strings.Contains(got, "可視データ行 No.: 43, 44") {
		t.Fatalf("missing scope metadata: %s", got)
	}
}

func TestPhase2CsvExecuteAppend_ForbidsOffImageRows(t *testing.T) {
	t.Parallel()
	got := phase2CsvExecuteAppend([]csvhint.CsvHint{{Path: "a.csv"}}, PhaseVisibleScope{VisibleRowIDs: []string{"43"}})
	for _, want := range []string{
		"可視スコープ外",
		"画像可視スコープ内",
		"出力に含めない",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestCsvFinalSynthesisAppend_NoCsvContent(t *testing.T) {
	t.Parallel()
	got := csvFinalSynthesisAppend([]csvhint.CsvHint{{Path: "a.csv", Content: "secret"}}, PhaseVisibleScope{})
	if strings.Contains(got, "secret") || strings.Contains(got, "--- CSV content ---") {
		t.Fatalf("synthesis append must not embed CSV body: %s", got)
	}
	for _, want := range []string{
		"CSV 参照により取得したセル値",
		"可視スコープ外",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q", want)
		}
	}
}

func TestBuildCsvHintContextEmptyReturnsEmpty(t *testing.T) {
	t.Parallel()
	if got := buildCsvHintContext(nil, CsvInjectFullContent, PhaseVisibleScope{}); got != "" {
		t.Fatalf("expected empty context, got %q", got)
	}
	if got := phase2CsvExecuteAppend(nil, PhaseVisibleScope{}); got != "" {
		t.Fatalf("expected empty phase2 append, got %q", got)
	}
	if got := csvFinalSynthesisAppend(nil, PhaseVisibleScope{}); got != "" {
		t.Fatalf("expected empty final append, got %q", got)
	}
}
