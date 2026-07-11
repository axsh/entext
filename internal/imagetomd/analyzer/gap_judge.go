package analyzer

import (
	"regexp"
	"strings"
)

type GapJudgeMode string

const (
	GapJudgeCompat GapJudgeMode = "compat"
	GapJudgeStrict GapJudgeMode = "strict"
)

var strictSufficientLine = regexp.MustCompile(`(?m)^\s*SUFFICIENT\s*$`)
var strictDecisionLine = regexp.MustCompile(`(?mi)^\s*Decision\s*:\s*SUFFICIENT\s*$`)

var compatNegativePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)NOT\s+SUFFICIENT`),
	regexp.MustCompile(`SUFFICIENT\s*ではありません`),
	regexp.MustCompile(`(?i)INSUFFICIENT`),
	regexp.MustCompile(`(?i)NOT_SUFFICIENT`),
}

func isCompatNegativeSufficient(resp string) bool {
	for _, p := range compatNegativePatterns {
		if p.MatchString(resp) {
			return true
		}
	}
	return false
}

func IsSufficient(resp string, mode GapJudgeMode) bool {
	switch mode {
	case GapJudgeStrict:
		return strictSufficientLine.MatchString(resp) || strictDecisionLine.MatchString(resp)
	default:
		if isCompatNegativeSufficient(resp) {
			return false
		}
		return strings.Contains(strings.ToUpper(resp), "SUFFICIENT")
	}
}
