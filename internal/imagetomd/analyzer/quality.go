package analyzer

import (
	"regexp"
	"strings"
)

var explanatoryHeadingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)要素一覧（Phase`),
	regexp.MustCompile(`書式・注記・セル結合`),
	regexp.MustCompile(`意味対応・解釈`),
	regexp.MustCompile(`図解概要`),
	regexp.MustCompile(`(?m)^##\s+図解要素`),
}

var interactiveQuestionPattern = regexp.MustCompile(`(?i)(確認してください|教えてください|どちら|選択してください|please confirm|which one|could you)`)
var yesNoPattern = regexp.MustCompile(`(?i)\b(y/n|yes/no)\b`)

func looksLikeExplanatoryReport(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, p := range explanatoryHeadingPatterns {
		if p.MatchString(trimmed) {
			return true
		}
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "要素id") && strings.Contains(lower, "意味対応") {
		return true
	}
	return false
}

func needsFinalSynthesisRetry(markdown string) (bool, string) {
	if strings.TrimSpace(markdown) == "" {
		return true, "empty"
	}
	if looksLikePhaseReport(markdown) {
		return true, "phase_report"
	}
	if looksLikeExplanatoryReport(markdown) {
		return true, "explanatory_report"
	}
	return false, ""
}

func isSimpleTextOutputInsufficient(md string) (bool, string) {
	if strings.TrimSpace(md) == "" {
		return true, "empty"
	}
	if looksLikePlanOnly(md) {
		return true, "plan_only"
	}
	if looksLikeInteractiveQuestion(md) {
		return true, "interactive_text"
	}
	return false, ""
}

func looksLikeInteractiveQuestion(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if hasMarkdownDataLines(trimmed) {
		return false
	}
	if interactiveQuestionPattern.MatchString(trimmed) {
		return true
	}
	if yesNoPattern.MatchString(trimmed) {
		return true
	}
	if (strings.HasSuffix(trimmed, "?") || strings.HasSuffix(trimmed, "？")) && len(trimmed) < 200 {
		return true
	}
	return false
}

func hasMarkdownDataLines(text string) bool {
	tableLines := 0
	listLines := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "|") && strings.Count(line, "|") >= 2 {
			tableLines++
		}
		if strings.HasPrefix(line, "- ") {
			listLines++
		}
	}
	return tableLines >= 1 || listLines >= 2
}
