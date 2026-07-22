package structure

import (
	"strings"
)

// ParseFields extracts Cell Mapping table rows from structure markdown.
func ParseFields(md string) ([]Field, error) {
	lines := strings.Split(md, "\n")
	inTable := false
	var fields []Field
	currentSheet := ""
	for _, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "## Sheet:") {
			currentSheet = strings.TrimSpace(strings.TrimPrefix(trim, "## Sheet:"))
			inTable = false
			continue
		}
		if trim == "### Cell Mapping" {
			inTable = true
			continue
		}
		if strings.HasPrefix(trim, "### ") || strings.HasPrefix(trim, "## ") {
			inTable = false
			continue
		}
		if !inTable || !strings.HasPrefix(trim, "|") {
			continue
		}
		cols := splitMDRow(trim)
		if len(cols) < 5 {
			continue
		}
		if cols[0] == "field_id" || cols[0] == "---" || cols[0] == "—" || strings.HasPrefix(cols[0], "---") {
			continue
		}
		cells := strings.Split(cols[3], ",")
		clean := make([]string, 0, len(cells))
		for _, c := range cells {
			c = strings.TrimSpace(c)
			if c != "" {
				clean = append(clean, c)
			}
		}
		sheet := cols[2]
		if sheet == "" {
			sheet = currentSheet
		}
		fields = append(fields, Field{
			ID:    cols[0],
			Label: cols[1],
			Sheet: sheet,
			Cells: clean,
			Role:  cols[4],
		})
	}
	return fields, nil
}

func splitMDRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}
