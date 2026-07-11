package csvhint

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
)

const MaxCsvInjectLines = 200

const csvTruncationNote = "\n... (CSV excerpt truncated; full file at Source path)\n"

// FilterCsvByScope returns CSV excerpt containing header rows through the No. column
// header plus data rows whose No. value matches visibleRowIDs.
// When visibleRowIDs is empty, returns the full input (caller relies on prompt constraints).
func FilterCsvByScope(content string, visibleRowIDs []string, maxLines int) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	if maxLines <= 0 {
		maxLines = MaxCsvInjectLines
	}

	records, headerIdx, noColIdx, ok := parseCSVRecords(content)
	if !ok {
		out, _ := TruncateCsvLines(content, maxLines)
		return out
	}

	idSet := make(map[string]struct{}, len(visibleRowIDs))
	for _, id := range visibleRowIDs {
		idSet[strings.TrimSpace(id)] = struct{}{}
	}

	var selected [][]string
	if len(idSet) == 0 {
		selected = records
	} else {
		selected = append(selected, records[:headerIdx+1]...)
		for _, rec := range records[headerIdx+1:] {
			if len(rec) <= noColIdx {
				continue
			}
			noVal := strings.TrimSpace(rec[noColIdx])
			if noVal == "" {
				continue
			}
			if _, keep := idSet[noVal]; keep {
				selected = append(selected, rec)
			}
		}
	}

	out := serializeCSVRecords(selected)
	out, _ = TruncateCsvLines(out, maxLines)
	return out
}

// TruncateCsvLines caps line count and appends a truncation note when exceeded.
func TruncateCsvLines(content string, maxLines int) (string, bool) {
	if maxLines <= 0 {
		return content, false
	}
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content, false
	}
	truncated := strings.Join(lines[:maxLines], "\n")
	return truncated + csvTruncationNote, true
}

func detectNoColumnIndex(lines []string) (headerLineIdx, noColIdx int, ok bool) {
	limit := len(lines)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		fields := splitCSVLine(lines[i])
		for j, f := range fields {
			if strings.TrimSpace(f) == "No." {
				return i, j, true
			}
		}
	}
	return 0, 0, false
}

func parseCSVRecords(content string) (records [][]string, headerIdx, noColIdx int, ok bool) {
	r := csv.NewReader(strings.NewReader(content))
	r.LazyQuotes = true
	r.FieldsPerRecord = -1
	all, err := r.ReadAll()
	if err != nil || len(all) == 0 {
		return nil, 0, 0, false
	}
	for i, rec := range all {
		if i > 5 {
			break
		}
		for j, f := range rec {
			if strings.TrimSpace(f) == "No." {
				return all, i, j, true
			}
		}
	}
	return nil, 0, 0, false
}

func serializeCSVRecords(records [][]string) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	for _, rec := range records {
		_ = w.Write(rec)
	}
	w.Flush()
	return strings.TrimRight(buf.String(), "\r\n") + "\n"
}

func splitCSVLine(line string) []string {
	r := csv.NewReader(strings.NewReader(line))
	r.LazyQuotes = true
	rec, err := r.Read()
	if err != nil {
		return strings.Split(line, ",")
	}
	return rec
}

func formatScopeIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, ", ")
}

// ScopeIDList formats visible row IDs for logging or metadata blocks.
func ScopeIDList(ids []string) string {
	return formatScopeIDs(ids)
}

// FilterStats returns a short debug summary of filter results.
func FilterStats(before, after string, scope []string) string {
	return fmt.Sprintf("scope=%s lines_before=%d lines_after=%d",
		formatScopeIDs(scope),
		strings.Count(before, "\n")+1,
		strings.Count(after, "\n")+1,
	)
}
