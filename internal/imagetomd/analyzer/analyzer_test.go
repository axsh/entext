package analyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/imagetomd/csvhint"
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

func (c *queueClient) LastSendGuardEvents() []tern.AgentGuardEvent {
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

	md, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

	md, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

func TestAnalyzeMapsInsufficientAssessmentToSufficientFalse(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "INSUFFICIENT\n未取得: 列見出し")
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if !strings.Contains(joined, "phase=2 round=1 sufficient=false") {
		t.Fatalf("expected phase=2 assess insufficient=false in logs:\n%s", joined)
	}
}

func TestAnalyzePhase2ContinuesWhenAssessIsInsufficientToken(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "INSUFFICIENT\n未取得: 行3")
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	joined := strings.Join(logs, "\n")
	if strings.Contains(joined, "step=phase2_guard") {
		t.Fatalf("phase2 guard should not run when assess is INSUFFICIENT:\n%s", joined)
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

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
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

type recordingClient struct {
	queueClient
	prompts []string
}

func (c *recordingClient) SendText(ctx context.Context, sessionID string, prompt string) (string, error) {
	c.prompts = append(c.prompts, prompt)
	return c.queueClient.SendText(ctx, sessionID, prompt)
}

func TestAnalyzeInjectsCsvHintOnPhase2Round1Only(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# 変更履歴\nok")

	hints := []csvhint.CsvHint{{Path: "h.csv", Content: "a,b\n1,2"}}
	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, hints)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(client.prompts) == 0 {
		t.Fatalf("expected recorded prompts")
	}
	if strings.Contains(client.prompts[0], "[Reference csv hint]") || strings.Contains(client.prompts[0], "--- CSV content ---") {
		t.Fatalf("classify prompt must not include csv hint: %s", client.prompts[0])
	}
	var assessPrompt string
	var phase2ExecutePrompt string
	for _, prompt := range client.prompts {
		if strings.Contains(prompt, "現在は画像解析の Phase") {
			assessPrompt = prompt
		}
		if strings.Contains(prompt, "--- CSV content ---") && strings.Contains(prompt, "| No. | 変更箇所 |") {
			phase2ExecutePrompt = prompt
		}
	}
	if assessPrompt == "" {
		t.Fatalf("expected assess prompt")
	}
	if strings.Contains(assessPrompt, "[Reference csv hint]") {
		t.Fatalf("assess prompt must not include csv hint")
	}
	if phase2ExecutePrompt == "" {
		t.Fatalf("expected phase2 execute prompt")
	}
	if !strings.Contains(phase2ExecutePrompt, "--- CSV content ---") {
		t.Fatalf("phase2 round1 execute missing scoped csv content")
	}
	if !strings.Contains(phase2ExecutePrompt, "可視スコープ外") {
		t.Fatalf("phase2 execute missing scope constraint")
	}
}

func TestAnalyzeClassifyUsesRefOnlyWithoutCsv(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# ok\n| No |\n|---|\n| 1 |")

	hints := []csvhint.CsvHint{{Path: "h.csv", Content: "a,b\n1,2"}}
	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, hints)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	for i, prompt := range client.prompts {
		if i == 0 {
			continue
		}
		if strings.Contains(prompt, "現在は画像解析の Phase 1") && strings.Contains(prompt, "UNATTENDED BATCH MODE") {
			if strings.Contains(prompt, "--- CSV content ---") {
				t.Fatalf("phase1 execute must not include csv content")
			}
		}
	}
}

func TestAnalyzePhase2KnownInitializesWithVisibleScope(t *testing.T) {
	t.Parallel()

	responses := []string{"mixed"}
	responses = append(responses, "INSUFFICIENT", "q1", phase1Round1AnswerFixture)
	responses = append(responses, "INSUFFICIENT", "q2", "| No | x |")
	responses = append(responses, "SUFFICIENT", "SUFFICIENT")
	responses = append(responses, "# Final\n| 43 |\n| 44 |")

	hints := []csvhint.CsvHint{{Path: "h.csv", Content: "a,b\n1,2"}}
	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, log, err := a.Analyze(context.Background(), "dummy.png", ".", nil, hints)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	var phase2 *PhaseLog
	for i := range log.Phases {
		if log.Phases[i].PhaseNum == 2 {
			phase2 = &log.Phases[i]
			break
		}
	}
	if phase2 == nil || len(phase2.Rounds) == 0 {
		t.Fatalf("expected phase2 rounds")
	}
	known := phase2.Rounds[0].KnownInfo
	if !strings.Contains(known, "43, 44") && !strings.Contains(known, "43") {
		t.Fatalf("phase2 known should include visible scope: %s", known)
	}
}

func TestAnalyzeAssessKnownDoesNotGrowQuadratically(t *testing.T) {
	t.Parallel()

	bigAnswer := strings.Repeat("row-data ", 400)
	responses := []string{"mixed"}
	responses = append(responses, "INSUFFICIENT", "q1", phase1Round1AnswerFixture)
	responses = append(responses, "INSUFFICIENT", "q2", bigAnswer)
	responses = append(responses, "INSUFFICIENT", "q3", "| No | x |")
	for i := 0; i < 8; i++ {
		responses = append(responses, "SUFFICIENT")
	}
	responses = append(responses, "# Final\nok", "# Final\nok")

	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    2,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	var assessKnown []int
	for _, prompt := range client.prompts {
		if strings.Contains(prompt, "現在は画像解析の Phase 1") {
			if idx := strings.Index(prompt, "---\n"); idx >= 0 {
				rest := prompt[idx+4:]
				if end := strings.Index(rest, "\n---"); end >= 0 {
					assessKnown = append(assessKnown, len(rest[:end]))
				}
			}
		}
	}
	if len(assessKnown) < 2 {
		t.Fatalf("expected multiple phase1 assess prompts, got %d", len(assessKnown))
	}
	if assessKnown[1] > maxLatestAnswerInPrompt+2000 {
		t.Fatalf("known grew quadratically: %v", assessKnown)
	}
}

func TestAnalyzeFinalSynthesisIncludesCsvAppendWhenHintsPresent(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# 変更履歴\n| No. |\n|---|---|\n| 1 |")

	hints := []csvhint.CsvHint{{Path: "h.csv", Content: "a,b\n1,2"}}
	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, hints)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	finalPrompt := client.prompts[len(client.prompts)-1]
	if !strings.Contains(finalPrompt, "可視スコープ外") {
		t.Fatalf("final synthesis missing csv scope policy: %s", finalPrompt)
	}
	if strings.Contains(finalPrompt, "--- CSV content ---") {
		t.Fatalf("final synthesis must not embed csv body")
	}
}

func TestAnalyzeWithoutHintsUnchangedPromptShape(t *testing.T) {
	t.Parallel()

	responses := make([]string, 0, len(DefaultPhases)*3+3)
	responses = append(responses, "mixed")
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# 変更履歴\nok")

	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	for _, prompt := range client.prompts {
		if strings.Contains(prompt, "[Reference csv hint]") {
			t.Fatalf("unexpected csv hint in prompt without hints")
		}
	}
}

func TestAnalyzeSimpleTextPromptIncludesNonInteractiveSuffixAndImage(t *testing.T) {
	t.Parallel()

	client := &recordingClient{queueClient: queueClient{responses: []string{
		"simple_text",
		"| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |",
	}}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{RoundSleepMS: 0, PhaseSleepMS: 0})

	_, log, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(client.prompts) < 2 {
		t.Fatalf("expected at least 2 prompts, got %d", len(client.prompts))
	}
	second := client.prompts[1]
	if !strings.Contains(second, "[Attached image:") || !strings.Contains(second, "UNATTENDED BATCH MODE") {
		t.Fatalf("simple_text prompt missing guard pieces: %q", second)
	}
	if !log.ShortPath {
		t.Fatalf("expected short path success")
	}
}

func TestAnalyzeSimpleTextRetriesOnInteractiveQuestion(t *testing.T) {
	t.Parallel()

	client := &recordingClient{queueClient: queueClient{responses: []string{
		"simple_text",
		"Could you confirm which column to use?",
		"| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |",
	}}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{RoundSleepMS: 0, PhaseSleepMS: 0})

	md, _, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(client.prompts) != 3 {
		t.Fatalf("expected 3 SendText calls, got %d", len(client.prompts))
	}
	if !strings.Contains(md, "| 選択 |") {
		t.Fatalf("unexpected markdown: %s", md)
	}
}

func TestAnalyzeSimpleTextFallsBackToComplexTable(t *testing.T) {
	t.Parallel()

	responses := []string{"simple_text", "Could you confirm?", "Could you confirm again?"}
	responses = appendDefaultPhaseResponses(responses, "")
	responses = append(responses, "# Final\n\n| Col |\n|---|\n| v |")

	client := &recordingClient{queueClient: queueClient{responses: responses}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{
		MaxRounds:    1,
		RoundSleepMS: 0,
		PhaseSleepMS: 0,
	})

	_, log, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if log.SimpleTextFallback == nil {
		t.Fatalf("expected simple_text_fallback log")
	}
	if log.ShortPath {
		t.Fatalf("expected complex path after fallback")
	}
	if len(log.Phases) == 0 {
		t.Fatalf("expected complex phases in log")
	}
}

func TestAnalyzeSimpleTextDoesNotFallbackOnStreamStall(t *testing.T) {
	t.Parallel()

	client := &stallOnSecondSendClient{queueClient: queueClient{responses: []string{
		"simple_text",
		"unused",
	}}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{RoundSleepMS: 0, PhaseSleepMS: 0})

	_, log, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if !errors.Is(err, tern.ErrStreamStall) {
		t.Fatalf("expected ErrStreamStall, got %v", err)
	}
	if log.SimpleTextFallback != nil {
		t.Fatalf("stall must not trigger fallback")
	}
}

func TestSessionLogRecordsAgentGuardEvents(t *testing.T) {
	t.Parallel()

	client := &guardEventClient{queueClient: queueClient{responses: []string{
		"simple_text",
		"| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |",
	}}}
	a := New(client, "codex", "gpt-5.3-codex", AnalyzeOptions{RoundSleepMS: 0, PhaseSleepMS: 0})

	_, log, err := a.Analyze(context.Background(), "dummy.png", ".", nil, nil)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if len(log.AgentGuardEvents) == 0 {
		t.Fatalf("expected agent guard events in session log")
	}
}

type stallOnSecondSendClient struct {
	queueClient
}

func (c *stallOnSecondSendClient) SendText(ctx context.Context, sessionID, prompt string) (string, error) {
	if c.idx == 1 {
		c.idx++
		return "", tern.ErrStreamStall
	}
	return c.queueClient.SendText(ctx, sessionID, prompt)
}

type guardEventClient struct {
	queueClient
	guard []tern.AgentGuardEvent
}

func (c *guardEventClient) SendText(ctx context.Context, sessionID, prompt string) (string, error) {
	out, err := c.queueClient.SendText(ctx, sessionID, prompt)
	c.guard = []tern.AgentGuardEvent{{
		Kind:         "user_input_required",
		PromptID:     "p1",
		AutoResponse: true,
	}}
	return out, err
}

func (c *guardEventClient) LastSendGuardEvents() []tern.AgentGuardEvent {
	return c.guard
}
