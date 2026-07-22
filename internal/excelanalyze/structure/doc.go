package structure

import (
	"fmt"
	"strings"
	"time"

	"github.com/axsh/entext/internal/excelcell"
	"github.com/xuri/excelize/v2"
)

const MarkdownVersion = "1"

type Document struct {
	Version    string
	SourcePath string
	AnalyzedAt time.Time
	Backend    string
	HintsUsed  bool
	Sheets     []Sheet
	RawNotes   map[string][]string // sheet -> raw cell lines (optional display)
}

type Sheet struct {
	Name     string
	Index    int
	Overview string
	Semantic string
	Fields   []Field
	Notes    []string
	RawCells []string
}

type Field struct {
	ID    string
	Label string
	Role  string
	Cells []string
	Merge string
}

func Render(doc Document) string {
	var sb strings.Builder
	sb.WriteString("# Excel Template Structure\n\n")
	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- version: %s\n", orDefault(doc.Version, MarkdownVersion)))
	sb.WriteString(fmt.Sprintf("- source: %s\n", doc.SourcePath))
	if !doc.AnalyzedAt.IsZero() {
		sb.WriteString(fmt.Sprintf("- analyzed_at: %s\n", doc.AnalyzedAt.UTC().Format(time.RFC3339)))
	}
	sb.WriteString(fmt.Sprintf("- backend: %s\n", doc.Backend))
	sb.WriteString(fmt.Sprintf("- hints_used: %t\n\n", doc.HintsUsed))

	for _, sh := range doc.Sheets {
		sb.WriteString(fmt.Sprintf("## Sheet: %s\n\n", sh.Name))
		sb.WriteString("### Overview\n\n")
		if strings.TrimSpace(sh.Overview) == "" {
			sb.WriteString("(none)\n\n")
		} else {
			sb.WriteString(sh.Overview)
			sb.WriteString("\n\n")
		}
		sb.WriteString("### Semantic Structure\n\n")
		if strings.TrimSpace(sh.Semantic) == "" {
			sb.WriteString("(none)\n\n")
		} else {
			sb.WriteString(sh.Semantic)
			sb.WriteString("\n\n")
		}
		sb.WriteString("### Cell Mapping\n\n")
		sb.WriteString("| field_id | label | sheet | cells | role |\n")
		sb.WriteString("| --- | --- | --- | --- | --- |\n")
		if len(sh.Fields) == 0 {
			sb.WriteString("| — | — | — | — | — |\n")
		} else {
			for _, f := range sh.Fields {
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					esc(f.ID), esc(f.Label), esc(sh.Name), esc(strings.Join(f.Cells, ",")), esc(f.Role)))
			}
		}
		sb.WriteString("\n")
		if len(sh.RawCells) > 0 {
			sb.WriteString("#### Raw Cells\n\n")
			for _, line := range sh.RawCells {
				sb.WriteString("- ")
				sb.WriteString(line)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("### Edit Notes\n\n")
		if len(sh.Notes) == 0 {
			sb.WriteString("(none)\n\n")
		} else {
			for _, n := range sh.Notes {
				sb.WriteString("- ")
				sb.WriteString(n)
				sb.WriteString("\n")
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func MergeSemantic(doc *Document, sheetName, semanticMarkdown string) {
	if doc == nil {
		return
	}
	for i := range doc.Sheets {
		if doc.Sheets[i].Name == sheetName {
			if strings.TrimSpace(semanticMarkdown) != "" {
				doc.Sheets[i].Semantic = semanticMarkdown
			}
			return
		}
	}
	doc.Sheets = append(doc.Sheets, Sheet{Name: sheetName, Semantic: semanticMarkdown})
}

func AttachCellSnapshots(doc *Document, snaps []excelcell.SheetSnapshot) error {
	if doc == nil {
		return fmt.Errorf("nil document")
	}
	byName := map[string]int{}
	for i := range doc.Sheets {
		byName[doc.Sheets[i].Name] = i
	}
	for _, snap := range snaps {
		idx, ok := byName[snap.Name]
		if !ok {
			doc.Sheets = append(doc.Sheets, Sheet{Name: snap.Name, Index: snap.Index})
			idx = len(doc.Sheets) - 1
			byName[snap.Name] = idx
		}
		sh := &doc.Sheets[idx]
		if sh.Index == 0 {
			sh.Index = snap.Index
		}
		raw := make([]string, 0, len(snap.Cells)+len(snap.MergeRanges))
		for _, c := range snap.Cells {
			raw = append(raw, fmt.Sprintf("%s=%q", c.Ref, c.Value))
		}
		for _, m := range snap.MergeRanges {
			raw = append(raw, "merge:"+m)
			sh.Notes = append(sh.Notes, "merged cells: "+m)
		}
		sh.RawCells = raw

		// Heuristic: non-empty label in column A with empty/missing right neighbor => input candidate.
		valueByRef := map[string]string{}
		for _, c := range snap.Cells {
			valueByRef[c.Ref] = c.Value
		}
		for _, c := range snap.Cells {
			col, row, err := parseA1(c.Ref)
			if err != nil || col != 1 {
				continue
			}
			right, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				continue
			}
			if _, exists := valueByRef[right]; exists && valueByRef[right] != "" {
				continue
			}
			id := slugID(c.Value)
			if id == "" {
				continue
			}
			// Keep vision fields if already present with same id.
			found := false
			for _, f := range sh.Fields {
				if f.ID == id {
					found = true
					break
				}
			}
			if found {
				continue
			}
			sh.Fields = append(sh.Fields, Field{
				ID:    id,
				Label: c.Value,
				Role:  "input",
				Cells: []string{right},
			})
		}
	}
	return nil
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func esc(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func slugID(label string) string {
	label = strings.TrimSpace(strings.ToLower(label))
	if label == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else if r == ' ' || r == '_' || r == '-' {
			b.WriteByte('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func parseA1(ref string) (col, row int, err error) {
	// Minimal A1 parser for heuristic (A=1).
	ref = strings.ToUpper(strings.TrimSpace(ref))
	i := 0
	for i < len(ref) && ref[i] >= 'A' && ref[i] <= 'Z' {
		col = col*26 + int(ref[i]-'A'+1)
		i++
	}
	if i == 0 || i == len(ref) {
		return 0, 0, fmt.Errorf("bad ref %q", ref)
	}
	for ; i < len(ref); i++ {
		if ref[i] < '0' || ref[i] > '9' {
			return 0, 0, fmt.Errorf("bad ref %q", ref)
		}
		row = row*10 + int(ref[i]-'0')
	}
	return col, row, nil
}
