package analyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/imagetomd/tern"
)

type queueClient struct {
	responses []string
	createErr error
	sendErr   error
	idx       int
}

func (c *queueClient) CreateSession(context.Context, tern.CreateSessionRequest) (string, error) {
	if c.createErr != nil {
		return "", c.createErr
	}
	return "session-1", nil
}

func (c *queueClient) SendText(context.Context, string, string) (string, error) {
	if c.sendErr != nil {
		return "", c.sendErr
	}
	if c.idx >= len(c.responses) {
		return "", errors.New("unexpected SendText call")
	}
	out := c.responses[c.idx]
	c.idx++
	return out, nil
}

func (c *queueClient) TerminateSession(context.Context, string) error {
	return nil
}

func appendDefaultPhaseResponses(responses []string, phase2Assess string) []string {
	for _, phase := range DefaultPhases {
		assess := "SUFFICIENT"
		if phase.Num == 2 && phase2Assess != "" {
			assess = phase2Assess
		}
		responses = append(responses, assess)
		if phase.Num == 2 {
			responses = append(responses, "phase2 question", "| No. | 変更箇所 |\n|---|---|\n| 43 | x |")
		}
	}
	return responses
}

func TestAnalyzeRetriesWhenFinalLooksLikePhaseReport(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "### Phase 1\nQ: q\nA: a")
	responses = append(responses, "# Final\n\n| Col |\n|---|\n| v |")

	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	md, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if !strings.Contains(md, "| Col |") {
		t.Fatalf("retry markdown was not returned: %s", md)
	}
}

func TestAnalyzeReturnsEmptyMarkdownErrorWhenRetryStillInvalid(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "### Phase 1\nQ: q\nA: a")
	responses = append(responses, "### Phase 2\nQ: q\nA: a")

	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if !errors.Is(err, ErrEmptyMarkdown) {
		t.Fatalf("expected ErrEmptyMarkdown, got %v", err)
	}
}

func TestAnalyzeProgressContainsPhaseRoundAndRetryFields(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "")
	responses = append(responses, "# Final\nok")

	var logs []string
	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
		Progress: func(format string, args ...any) {
			logs = append(logs, strings.TrimSpace(fmt.Sprintf(format, args...)))
		},
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	joined := strings.Join(logs, "\n")
	mustContain := []string{
		"step=phase_start",
		"step=round_start",
		"step=assess_end",
		"step=round_end",
		"step=final_synthesis_retry",
		"step=analyze_end",
	}
	for _, token := range mustContain {
		if !strings.Contains(joined, token) {
			t.Fatalf("missing progress token %q in logs:\n%s", token, joined)
		}
	}
}

func TestAnalyzeRetriesWhenFinalLooksLikeExplanatoryReport(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "## 要素一覧（Phase 1）\n| 要素ID |")
	responses = append(responses, "# 変更履歴\n\n| No. | 変更箇所 |\n|---|---|\n| 43 | x |")

	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	md, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if !strings.Contains(md, "| No. |") {
		t.Fatalf("retry markdown was not returned: %s", md)
	}
}

func TestAnalyzePhase2RequiresNonEmptyAnswerBeforeSoftLimit(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# 変更履歴\nok")

	var logs []string
	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
		Progress: func(format string, args ...any) {
			logs = append(logs, strings.TrimSpace(fmt.Sprintf(format, args...)))
		},
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "step=phase2_guard") {
		t.Fatalf("expected phase2 guard in logs:\n%s", joined)
	}
	if !strings.Contains(joined, "phase=2 round=1") {
		t.Fatalf("expected phase=2 round=1 in logs:\n%s", joined)
	}
}

func TestAnalyzePhase2ContinuesWhenCompatAssessIsNegated(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "SUFFICIENT ではありません")
	responses = append(responses, "# 変更履歴\nok")

	var logs []string
	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
		Progress: func(format string, args ...any) {
			logs = append(logs, strings.TrimSpace(fmt.Sprintf(format, args...)))
		},
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, "step=phase2_guard") {
		t.Fatalf("phase2 guard should not run when assess is negated:\n%s", joined)
	}
	if !strings.Contains(joined, "phase=2 round=1") {
		t.Fatalf("expected phase=2 round=1 in logs:\n%s", joined)
	}
}

func TestAnalyzePersistsSessionIncrementally(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# 変更履歴\nok")

	var persistCount int
	var lastStatus string
	var lastPhaseCount int
	client := &queueClient{responses: responses}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
		SessionPersist: func(log *SessionLog) error {
			persistCount++
			lastStatus = log.Status
			lastPhaseCount = len(log.Phases)
			return nil
		},
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if persistCount < 3 {
		t.Fatalf("expected multiple persists, got %d", persistCount)
	}
	if lastStatus != "completed" {
		t.Fatalf("last status got %q want completed", lastStatus)
	}
	if lastPhaseCount != len(DefaultPhases) {
		t.Fatalf("last phase count got %d want %d", lastPhaseCount, len(DefaultPhases))
	}
}
