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
