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

func TestAnalyzeRetriesWhenFinalLooksLikePhaseReport(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)+3)
	responses = append(responses, "mixed")
	for range DefaultPhases {
		responses = append(responses, "SUFFICIENT")
	}
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

	responses := make([]string, 0, len(DefaultPhases)+3)
	responses = append(responses, "mixed")
	for range DefaultPhases {
		responses = append(responses, "SUFFICIENT")
	}
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

	responses := make([]string, 0, len(DefaultPhases)+3)
	responses = append(responses, "mixed")
	for range DefaultPhases {
		responses = append(responses, "SUFFICIENT")
	}
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
