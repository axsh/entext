package csvhint

import (
	"strings"
	"testing"
)

func TestFilterCsvByScope_KeepsOnlyVisibleRowIDs(t *testing.T) {
	t.Parallel()
	full := ",No.,変更箇所\n,1,row1\n,43,row43\n,44,row44\n"
	got := FilterCsvByScope(full, []string{"43", "44"}, MaxCsvInjectLines)
	if strings.Contains(got, ",1,") || strings.Contains(got, "row1") {
		t.Fatalf("filtered CSV must not contain row 1: %s", got)
	}
	for _, want := range []string{"No.", "43", "row43", "44", "row44"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in filtered CSV: %s", want, got)
		}
	}
}

func TestFilterCsvByScope_EmptyScopeReturnsFullWithCap(t *testing.T) {
	t.Parallel()
	full := ",No.,col\n,1,a\n,2,b\n"
	got := FilterCsvByScope(full, nil, MaxCsvInjectLines)
	if !strings.Contains(got, ",1,a") || !strings.Contains(got, ",2,b") {
		t.Fatalf("empty scope should keep full CSV: %s", got)
	}
}

func TestTruncateCsvLines_AddsTruncationNote(t *testing.T) {
	t.Parallel()
	var lines []string
	for i := 0; i < 201; i++ {
		lines = append(lines, "line")
	}
	content := strings.Join(lines, "\n")
	got, truncated := TruncateCsvLines(content, 200)
	if !truncated {
		t.Fatal("expected truncation")
	}
	if !strings.Contains(got, "CSV excerpt truncated") {
		t.Fatalf("missing truncation note: %s", got)
	}
	if strings.Count(got, "line\n") > 200 {
		t.Fatalf("expected at most 200 data lines before note")
	}
}

func TestFilterCsvByScope_GoldenScenarioTwoVisibleRows(t *testing.T) {
	t.Parallel()
	full := ",No.,変更箇所,作成・変更者\n,1,early,梅沢\n,42,late,長谷\n,43,【詳細設計】,秋葉達也\n,44,【詳細設計】,藤本華子\n"
	got := FilterCsvByScope(full, []string{"43", "44"}, MaxCsvInjectLines)
	if strings.Contains(got, ",1,") || strings.Contains(got, ",42,") {
		t.Fatalf("filter must drop off-image rows: %s", got)
	}
	for _, want := range []string{"43", "44", "秋葉達也", "藤本華子"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in filtered excerpt: %s", want, got)
		}
	}
	dataRows := 0
	for _, line := range strings.Split(strings.TrimSpace(got), "\n") {
		if strings.HasPrefix(line, ",43,") || strings.HasPrefix(line, ",44,") {
			dataRows++
		}
	}
	if dataRows != 2 {
		t.Fatalf("expected 2 scoped data rows, got %d in:\n%s", dataRows, got)
	}
}

func TestDetectNoColumnIndex(t *testing.T) {
	t.Parallel()
	lines := []string{
		",変更履歴,,,",
		",No.,変更箇所,変更内容",
		",1,data",
	}
	headerIdx, colIdx, ok := detectNoColumnIndex(lines)
	if !ok {
		t.Fatal("expected header detection")
	}
	if headerIdx != 1 || colIdx != 1 {
		t.Fatalf("got headerIdx=%d colIdx=%d", headerIdx, colIdx)
	}
}
