package analyzer

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type PhaseVisibleScope struct {
	VisibleRowIDs      []string
	ColumnCount        int
	ColumnNames        []string
	VisibleBlankRows   int
	NestedPlaceholders []string
}

func (s PhaseVisibleScope) IsEmpty() bool {
	return len(s.VisibleRowIDs) == 0 &&
		s.ColumnCount == 0 &&
		len(s.ColumnNames) == 0 &&
		s.VisibleBlankRows == 0 &&
		len(s.NestedPlaceholders) == 0
}

func (s PhaseVisibleScope) SummaryText() string {
	if s.IsEmpty() {
		return ""
	}
	var b strings.Builder
	b.WriteString("[画像可視スコープ]\n")
	if len(s.VisibleRowIDs) > 0 {
		b.WriteString("- 可視データ行 No.: ")
		b.WriteString(strings.Join(s.VisibleRowIDs, ", "))
		b.WriteString("\n")
	}
	if s.ColumnCount > 0 {
		b.WriteString("- 列数: ")
		b.WriteString(strconv.Itoa(s.ColumnCount))
		b.WriteString("\n")
	}
	if len(s.ColumnNames) > 0 {
		b.WriteString("- 列名: ")
		b.WriteString(strings.Join(s.ColumnNames, ", "))
		b.WriteString("\n")
	}
	if s.VisibleBlankRows > 0 {
		b.WriteString("- 可視空欄行: ")
		b.WriteString(strconv.Itoa(s.VisibleBlankRows))
		b.WriteString("\n")
	}
	if len(s.NestedPlaceholders) > 0 {
		b.WriteString("- 入れ子: ")
		b.WriteString(strings.Join(s.NestedPlaceholders, ", "))
		b.WriteString("\n")
	}
	if len(s.VisibleRowIDs) > 0 {
		b.WriteString("- 出力禁止: 上記 No. 以外のデータ行を CSV から追加しないこと\n")
	}
	return strings.TrimSpace(b.String())
}

// ExtractVisibleScopeFromPhase1 scans Phase 1 round answers; later rounds override conflicts.
func ExtractVisibleScopeFromPhase1(phaseLog PhaseLog) PhaseVisibleScope {
	var scope PhaseVisibleScope
	for _, round := range phaseLog.Rounds {
		answer := strings.TrimSpace(round.Answer)
		if answer == "" {
			continue
		}
		if ids := extractRowIDsFromText(answer); len(ids) > 0 {
			scope.VisibleRowIDs = ids
		}
		if cols := extractColumnNamesFromText(answer); len(cols) > 0 {
			scope.ColumnNames = cols
			scope.ColumnCount = len(cols)
		} else if n := extractColumnCountFromText(answer); n > 0 {
			scope.ColumnCount = n
		}
		if blanks := extractVisibleBlankRows(answer); blanks > 0 {
			scope.VisibleBlankRows = blanks
		}
		if ph := extractPlaceholderIDs(answer); len(ph) > 0 {
			scope.NestedPlaceholders = ph
		}
	}
	return scope
}

const maxLatestAnswerInPrompt = 2000

func buildPromptKnownInfo(scope PhaseVisibleScope, previousAnswer string, roundNum int) string {
	summary := scope.SummaryText()
	if roundNum <= 1 {
		return summary
	}
	prev := strings.TrimSpace(previousAnswer)
	if prev == "" {
		return summary
	}
	if len(prev) > maxLatestAnswerInPrompt {
		prev = prev[:maxLatestAnswerInPrompt] + "\n... (truncated)"
	}
	if summary == "" {
		return "[直近ラウンド回答]\n" + prev
	}
	return summary + "\n\n[直近ラウンド回答]\n" + prev
}

var (
	reNoBacktick     = regexp.MustCompile("`No\\.(\\d+)`")
	reNoPlain        = regexp.MustCompile(`No\.(\d+)`)
	reMdRowID        = regexp.MustCompile(`(?m)^\|\s*(\d+)\s*\|`)
	reBlankRows      = regexp.MustCompile(`空欄(\d+)`)
	reVisibleBlank   = regexp.MustCompile(`データ\d+\s*\+\s*空欄(\d+)`)
	reColumnCount    = regexp.MustCompile(`列数\s*\|\s*(\d+)`)
	reColumnCountJP  = regexp.MustCompile(`(\d+)列`)
	rePlaceholder    = regexp.MustCompile(`\[SUB_TABLE_P\d+\]`)
	reColumnNamesRow = regexp.MustCompile(`全列名\s*\|\s*([^|]+)\|`)
)

func extractRowIDsFromText(text string) []string {
	if idx := strings.Index(text, "取得済みデータ行"); idx >= 0 {
		end := idx + 300
		if end > len(text) {
			end = len(text)
		}
		if ids := collectRowIDs(text[idx:end]); len(ids) > 0 {
			return ids
		}
	}
	for _, marker := range []string{"表示データ行", "可視データ行", "可視行数"} {
		if idx := strings.Index(text, marker); idx >= 0 {
			end := idx + 400
			if end > len(text) {
				end = len(text)
			}
			if ids := collectRowIDs(text[idx:end]); len(ids) > 0 {
				return ids
			}
		}
	}
	return collectRowIDs(text)
}

func collectRowIDs(zone string) []string {
	seen := make(map[string]struct{})
	var ids []string
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		ids = append(ids, raw)
	}
	for _, m := range reNoBacktick.FindAllStringSubmatch(zone, -1) {
		add(m[1])
	}
	for _, m := range reNoPlain.FindAllStringSubmatch(zone, -1) {
		add(m[1])
	}
	for _, m := range reMdRowID.FindAllStringSubmatch(zone, -1) {
		add(m[1])
	}
	sort.Slice(ids, func(i, j int) bool {
		ai, _ := strconv.Atoi(ids[i])
		aj, _ := strconv.Atoi(ids[j])
		return ai < aj
	})
	return ids
}

func extractColumnNamesFromText(text string) []string {
	if m := reColumnNamesRow.FindStringSubmatch(text); len(m) == 2 {
		parts := strings.Split(m[1], "/")
		var cols []string
		for _, p := range parts {
			p = strings.TrimSpace(strings.Trim(p, "`"))
			if p != "" {
				cols = append(cols, p)
			}
		}
		if len(cols) > 0 {
			return cols
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "No.") || !strings.Contains(line, "変更") {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(line), "|") && strings.Count(line, "|") >= 3 {
			fields := strings.Split(line, "|")
			var cols []string
			for _, f := range fields {
				f = strings.TrimSpace(f)
				if f != "" && f != "---" && !strings.HasPrefix(f, "-") {
					cols = append(cols, f)
				}
			}
			if len(cols) >= 3 {
				return cols
			}
		}
	}
	return nil
}

func extractColumnCountFromText(text string) int {
	if m := reColumnCount.FindStringSubmatch(text); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	best := 0
	for _, m := range reColumnCountJP.FindAllStringSubmatch(text, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > best && n <= 50 {
			best = n
		}
	}
	return best
}

func extractVisibleBlankRows(text string) int {
	if m := reVisibleBlank.FindStringSubmatch(text); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	if m := reBlankRows.FindStringSubmatch(text); len(m) == 2 {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return 0
}

func extractPlaceholderIDs(text string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, m := range rePlaceholder.FindAllString(text, -1) {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
