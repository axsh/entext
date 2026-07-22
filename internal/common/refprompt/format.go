package refprompt

import (
	"strings"
)

type FormatMode string

const (
	ModeAnalyze FormatMode = "analyze"
	ModeFill    FormatMode = "fill"
)

const HintPolicyAnalyze = `Hint usage policy (template analyze):
- Reference markdown and additional prompts are hints for understanding template meaning (fill areas, fixed labels, reading order).
- They are NOT ground truth for cell addresses; cell coordinates come from the Excel library read.
- Prefer image layout for structure; use hints to disambiguate fill vs label regions.`

const HintPolicyFill = `Hint usage policy (excel fill):
- Reference markdown and additional prompts guide what values to write and formatting conventions.
- Cell coordinates in the structure markdown are authoritative for where to write.
- Do not invent cell addresses that are absent from the structure markdown.`

func FormatForPrompt(b HintBundle, mode FormatMode) string {
	var sb strings.Builder
	switch mode {
	case ModeFill:
		sb.WriteString(HintPolicyFill)
	default:
		sb.WriteString(HintPolicyAnalyze)
	}
	sb.WriteString("\n\n")

	if len(b.Refs) > 0 {
		sb.WriteString("[Reference markdown context]\n")
		for _, doc := range b.Refs {
			sb.WriteString("### ")
			sb.WriteString(doc.Path)
			sb.WriteString("\n")
			sb.WriteString(doc.Content)
			sb.WriteString("\n\n")
		}
	}
	if strings.TrimSpace(b.Prompts) != "" {
		sb.WriteString("[Additional prompt hints]\n")
		sb.WriteString(b.Prompts)
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}
