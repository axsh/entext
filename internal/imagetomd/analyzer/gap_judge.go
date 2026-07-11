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

func IsSufficient(resp string, mode GapJudgeMode) bool {
	switch mode {
	case GapJudgeStrict:
		return strictSufficientLine.MatchString(resp) || strictDecisionLine.MatchString(resp)
	default:
		return strings.Contains(strings.ToUpper(resp), "SUFFICIENT")
	}
}
