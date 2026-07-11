package analyzer

import (
	"context"
	"path/filepath"
	"strings"
	"time"

	"github.com/axsh/entext/internal/imagetomd/refresolver"
	"github.com/axsh/entext/internal/imagetomd/tern"
)

type AnalyzeOptions struct {
	StrictGapJudge  bool
	SaveQuestionLog bool
	RoundSleepMS    int
	PhaseSleepMS    int
	MaxRounds       int
	Progress        func(format string, args ...any)
}

type Analyzer struct {
	client tern.Client
	agent  string
	model  string
	opts   AnalyzeOptions
}

func New(client tern.Client, agent string, model string, opts AnalyzeOptions) *Analyzer {
	if opts.RoundSleepMS <= 0 {
		opts.RoundSleepMS = 5000
	}
	if opts.PhaseSleepMS <= 0 {
		opts.PhaseSleepMS = 5000
	}
	if opts.MaxRounds <= 0 {
		opts.MaxRounds = 0
	}
	return &Analyzer{
		client: client,
		agent:  agent,
		model:  model,
		opts:   opts,
	}
}

func (a *Analyzer) Analyze(ctx context.Context, imagePath string, workDir string, refs []refresolver.RefDocument) (string, *SessionLog, error) {
	started := time.Now()
	a.progressf("step=analyze_start image=%s", imagePath)
	absPath, err := filepath.Abs(imagePath)
	if err != nil {
		return "", nil, err
	}
	absPath = filepath.ToSlash(absPath)

	sessionID, err := a.client.CreateSession(ctx, tern.CreateSessionRequest{
		Agent:   a.agent,
		Model:   a.model,
		WorkDir: workDir,
	})
	if err != nil {
		return "", nil, err
	}
	a.progressf("step=session_created session_id=%s", sessionID)
	defer func() { _ = a.client.TerminateSession(ctx, sessionID) }()

	log := &SessionLog{
		ImagePath: absPath,
		StartedAt: time.Now().UTC(),
		Phases:    make([]PhaseLog, 0, len(DefaultPhases)),
	}
	refContext := buildRefContext(refs)

	classResp, err := a.client.SendText(ctx, sessionID, ClassifyPrompt+refContext+AttachedImageLine(absPath))
	if err != nil {
		return "", nil, err
	}
	category := extractClassification(classResp)
	a.progressf("step=classify_done category=%s", category)
	log.Category = category

	if category == "simple_text" {
		a.progressf("step=simple_text_path")
		md, err := a.client.SendText(ctx, sessionID, SimpleTextPrompt)
		if err != nil {
			return "", nil, err
		}
		if strings.TrimSpace(md) == "" {
			return "", nil, ErrEmptyMarkdown
		}
		log.ShortPath = true
		log.CompletedAt = time.Now().UTC()
		return md, log, nil
	}

	mode := GapJudgeCompat
	if a.opts.StrictGapJudge {
		mode = GapJudgeStrict
	}

	for _, phase := range DefaultPhases {
		phaseStarted := time.Now()
		a.progressf("step=phase_start phase=%d phase_name=%s", phase.Num, phase.Name)
		phaseLog := PhaseLog{
			PhaseNum:  phase.Num,
			PhaseName: phase.Name,
			Goal:      phase.Goal,
			Rounds:    make([]RoundLog, 0, phase.MaxRounds),
		}
		known := ""
		maxRounds := phase.MaxRounds
		if a.opts.MaxRounds > 0 {
			maxRounds = a.opts.MaxRounds
		}
		extendedForComplexity := false
		for round := 0; round < maxRounds; round++ {
			roundNum := round + 1
			roundStarted := time.Now()
			a.progressf("step=round_start phase=%d round=%d", phase.Num, roundNum)
			a.progressf("step=assess_start phase=%d round=%d", phase.Num, roundNum)
			assessPrompt := AssessGapPrompt(phase, known)
			assessResult, err := a.client.SendText(ctx, sessionID, assessPrompt)
			if err != nil {
				return "", nil, err
			}
			sufficient := IsSufficient(assessResult, mode)
			a.progressf("step=assess_end phase=%d round=%d sufficient=%t", phase.Num, roundNum, sufficient)
			roundLog := RoundLog{
				KnownInfo:     known,
				GapAssessment: assessResult,
				Sufficient:    sufficient,
			}
			if sufficient {
				if phase.Num == 2 && !phaseLog.HasNonEmptyAnswer() {
					a.progressf("step=phase2_guard reason=no_answer_before_soft_limit")
					sufficient = false
					roundLog.Sufficient = false
				} else {
					phaseLog.Rounds = append(phaseLog.Rounds, roundLog)
					phaseLog.ExitReason = "soft_limit"
					a.progressf(
						"step=round_end phase=%d round=%d sufficient=%t answer_chars=0 elapsed_ms=%d retry_reason=none",
						phase.Num,
						roundNum,
						sufficient,
						time.Since(roundStarted).Milliseconds(),
					)
					break
				}
			}

			a.progressf("step=generate_question_start phase=%d round=%d", phase.Num, roundNum)
			question, err := a.client.SendText(ctx, sessionID, GenerateQuestionPrompt(phase, assessResult))
			if err != nil {
				return "", nil, err
			}
			a.progressf("step=generate_question_end phase=%d round=%d question_chars=%d", phase.Num, roundNum, len(strings.TrimSpace(question)))
			if a.opts.SaveQuestionLog {
				roundLog.Question = question
			}

			if err := sleepWithContext(ctx, time.Duration(a.opts.RoundSleepMS)*time.Millisecond); err != nil {
				return "", nil, err
			}
			a.progressf("step=execute_start phase=%d round=%d", phase.Num, roundNum)
			answerPrompt := question + ExecutionQuestionSuffix + refContext + AttachedImageLine(absPath)
			if phase.Num == 2 {
				answerPrompt = question + "\n\n" + Phase2ExecuteHint() + ExecutionQuestionSuffix + refContext + AttachedImageLine(absPath)
			}
			answer, err := a.client.SendText(ctx, sessionID, answerPrompt)
			if err != nil {
				return "", nil, err
			}
			roundLog.Answer = answer
			answerTrimmed := strings.TrimSpace(answer)
			switch {
			case answerTrimmed == "":
				a.progressf("step=execute_end phase=%d round=%d answer_chars=0 retry_reason=empty_answer", phase.Num, roundNum)
			case looksLikePlanOnly(answerTrimmed):
				a.progressf("step=execute_end phase=%d round=%d answer_chars=%d plan_only=true", phase.Num, roundNum, len(answerTrimmed))
			default:
				a.progressf("step=execute_end phase=%d round=%d answer_chars=%d plan_only=false", phase.Num, roundNum, len(answerTrimmed))
			}
			known = strings.TrimSpace(known + "\n\n" + answer)
			phaseLog.Rounds = append(phaseLog.Rounds, roundLog)
			a.progressf(
				"step=round_end phase=%d round=%d sufficient=%t answer_chars=%d elapsed_ms=%d retry_reason=none",
				phase.Num,
				roundNum,
				sufficient,
				len(answerTrimmed),
				time.Since(roundStarted).Milliseconds(),
			)

			if round == maxRounds-1 {
				phaseLog.ExitReason = "hard_limit"
			}
			// Complex table extraction tends to need additional rounds in phase 3/4.
			if !extendedForComplexity && (phase.Num == 3 || phase.Num == 4) && round == maxRounds-1 {
				lowerKnown := strings.ToLower(known)
				if strings.Contains(lowerKnown, "table") || strings.Contains(known, "表") {
					a.progressf("step=phase_extend phase=%d reason=complex_table extra_rounds=2", phase.Num)
					maxRounds += 2
					extendedForComplexity = true
				}
			}
		}
		log.Phases = append(log.Phases, phaseLog)
		a.progressf(
			"step=phase_end phase=%d phase_name=%s reason=%s elapsed_ms=%d",
			phase.Num,
			phase.Name,
			phaseLog.ExitReason,
			time.Since(phaseStarted).Milliseconds(),
		)

		if err := sleepWithContext(ctx, time.Duration(a.opts.PhaseSleepMS)*time.Millisecond); err != nil {
			return "", nil, err
		}
	}

	a.progressf("step=final_synthesis_start")
	finalPrompt := GenerateMarkdownPrompt(log.Phases)
	markdown, err := a.client.SendText(ctx, sessionID, finalPrompt)
	if err != nil {
		return "", nil, err
	}
	a.progressf("step=final_synthesis_end output_chars=%d", len(strings.TrimSpace(markdown)))
	if retry, reason := needsFinalSynthesisRetry(markdown); retry {
		a.progressf("step=final_synthesis_retry reason=%s", reason)
		retryPrompt := GenerateMarkdownRetryPrompt(buildAnswerCorpus(log.Phases))
		markdown, err = a.client.SendText(ctx, sessionID, retryPrompt)
		if err != nil {
			return "", nil, err
		}
		a.progressf("step=final_synthesis_retry_end output_chars=%d", len(strings.TrimSpace(markdown)))
	}
	if retry, _ := needsFinalSynthesisRetry(markdown); retry {
		return "", nil, ErrEmptyMarkdown
	}
	a.progressf("step=analyze_end output_chars=%d elapsed_ms=%d", len(strings.TrimSpace(markdown)), time.Since(started).Milliseconds())
	log.CompletedAt = time.Now().UTC()
	return markdown, log, nil
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func extractClassification(resp string) string {
	normalized := strings.ToLower(resp)
	candidates := []string{"simple_text", "complex_table", "diagram", "mixed"}
	best := "mixed"
	bestIndex := -1
	for _, candidate := range candidates {
		idx := strings.LastIndex(normalized, candidate)
		if idx >= bestIndex {
			bestIndex = idx
			best = candidate
		}
	}
	return best
}

func buildRefContext(refs []refresolver.RefDocument) string {
	if len(refs) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n[Reference markdown context]\n")
	for _, ref := range refs {
		b.WriteString("\n---\n")
		b.WriteString(ref.Path)
		b.WriteString("\n")
		b.WriteString(ref.Content)
		b.WriteString("\n")
	}
	return b.String()
}

func buildAnswerCorpus(phaseLogs []PhaseLog) string {
	var b strings.Builder
	for _, p := range phaseLogs {
		phaseHeaderWritten := false
		for _, r := range p.Rounds {
			answer := strings.TrimSpace(r.Answer)
			if answer == "" {
				continue
			}
			if !phaseHeaderWritten {
				b.WriteString("## Phase ")
				b.WriteString(strings.TrimSpace(p.PhaseName))
				b.WriteString("\n\n")
				phaseHeaderWritten = true
			}
			b.WriteString(answer)
			b.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(b.String())
}

func (a *Analyzer) progressf(format string, args ...any) {
	if a == nil || a.opts.Progress == nil {
		return
	}
	a.opts.Progress(format, args...)
}

func looksLikePlanOnly(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return false
	}
	planMarkers := []string{
		"i will",
		"i'll",
		"first,",
		"まず",
		"これから",
		"確認します",
		"作成します",
		"解析します",
	}
	for _, marker := range planMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	if strings.HasPrefix(lower, "画像") && strings.Contains(lower, "します") && !strings.Contains(lower, "|") && !strings.Contains(lower, "- ") {
		return true
	}
	return false
}

func looksLikePhaseReport(text string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "" {
		return false
	}
	if strings.Contains(trimmed, "q:") && strings.Contains(trimmed, "a:") && strings.Contains(trimmed, "phase") {
		return true
	}
	if strings.Contains(trimmed, "## phase 1") || strings.Contains(trimmed, "### phase 1") {
		return true
	}
	return false
}
