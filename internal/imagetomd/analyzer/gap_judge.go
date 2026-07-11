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

var (
	strictSufficientLine       = regexp.MustCompile(`(?m)^\s*SUFFICIENT\s*$`)
	strictInsufficientLine   = regexp.MustCompile(`(?m)^\s*INSUFFICIENT\s*$`)
	strictDecisionSufficient   = regexp.MustCompile(`(?mi)^\s*Decision\s*:\s*SUFFICIENT\s*$`)
	strictDecisionInsufficient = regexp.MustCompile(`(?mi)^\s*Decision\s*:\s*INSUFFICIENT\s*$`)

	compatInsufficientToken = regexp.MustCompile(`(?i)INSUFFICIENT`)
	compatLegacyNegativePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)NOT\s+SUFFICIENT`),
		regexp.MustCompile(`SUFFICIENT\s*ではありません`),
		regexp.MustCompile(`(?i)NOT_SUFFICIENT`),
	}
)

func isCompatInsufficient(resp string) bool {
	if compatInsufficientToken.MatchString(resp) {
		return true
	}
	for _, p := range compatLegacyNegativePatterns {
		if p.MatchString(resp) {
			return true
		}
	}
	return false
}

func isCompatSufficient(resp string) bool {
	return strings.Contains(strings.ToUpper(resp), "SUFFICIENT")
}

func isStrictInsufficient(resp string) bool {
	return strictInsufficientLine.MatchString(resp) ||
		strictDecisionInsufficient.MatchString(resp)
}

func isStrictSufficient(resp string) bool {
	return strictSufficientLine.MatchString(resp) ||
		strictDecisionSufficient.MatchString(resp)
}

func IsSufficient(resp string, mode GapJudgeMode) bool {
	switch mode {
	case GapJudgeStrict:
		if isStrictInsufficient(resp) {
			return false
		}
		return isStrictSufficient(resp)
	default:
		if isCompatInsufficient(resp) {
			return false
		}
		if isCompatSufficient(resp) {
			return true
		}
		return false
	}
}
