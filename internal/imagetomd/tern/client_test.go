package tern

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestSendTextHandlesUserInputRequiredThenCompletes(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{
		messageStreams: [][]sseEvent{{
			{Type: "user_input_required", Content: "Which column?", PromptID: "p1", Choices: []string{"colA", "colB"}},
		}},
		respondStreams: [][]sseEvent{{
			{Type: "text", Content: "| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |"},
			{Type: "result"},
		}},
	})
	defer srv.Close()

	var logs []string
	client := NewClientWithSendOptions(srv.URL, SendOptions{
		TotalTimeout:     5 * time.Second,
		IdleTimeout:      2 * time.Second,
		MaxAutoResponses: 3,
		Progress: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	})

	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, CreateSessionRequest{Agent: "codex", WorkDir: "."})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	got, err := client.SendText(ctx, sessionID, "convert")
	if err != nil {
		t.Fatalf("SendText failed: %v", err)
	}
	if !strings.Contains(got, "| 選択 |") {
		t.Fatalf("unexpected response: %q", got)
	}
	if !containsProgress(logs, "step=agent_guard kind=user_input_required") {
		t.Fatalf("missing agent_guard progress: %v", logs)
	}
	events := client.LastSendGuardEvents()
	if len(events) != 1 || events[0].Kind != "user_input_required" {
		t.Fatalf("unexpected guard events: %+v", events)
	}
}

func TestSendTextReturnsErrInteractiveInputRequiredOnFourthPrompt(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{
		messageStreams: [][]sseEvent{{
			{Type: "user_input_required", Content: "q1"},
		}},
		respondStreams: [][]sseEvent{
			{{Type: "user_input_required", Content: "q2"}},
			{{Type: "user_input_required", Content: "q3"}},
			{{Type: "user_input_required", Content: "q4"}},
		},
	})
	defer srv.Close()

	client := NewClientWithSendOptions(srv.URL, SendOptions{
		TotalTimeout:     5 * time.Second,
		IdleTimeout:      2 * time.Second,
		MaxAutoResponses: 3,
	})

	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, CreateSessionRequest{Agent: "codex", WorkDir: "."})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	_, err = client.SendText(ctx, sessionID, "convert")
	if !errors.Is(err, ErrInteractiveInputRequired) {
		t.Fatalf("expected ErrInteractiveInputRequired, got %v", err)
	}
}

func TestSendTextReturnsErrStreamStallOnIdleTimeout(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{hangAfterFirstEvent: true})
	defer srv.Close()

	client := NewClientWithSendOptions(srv.URL, SendOptions{
		TotalTimeout:     5 * time.Second,
		IdleTimeout:      200 * time.Millisecond,
		MaxAutoResponses: 3,
	})

	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, CreateSessionRequest{Agent: "codex", WorkDir: "."})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	_, err = client.SendText(ctx, sessionID, "convert")
	if !errors.Is(err, ErrStreamStall) {
		t.Fatalf("expected ErrStreamStall, got %v", err)
	}
}

func TestSendTextRespectsTotalDeadline(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{hangBody: true})
	defer srv.Close()

	client := NewClientWithSendOptions(srv.URL, SendOptions{
		TotalTimeout:     200 * time.Millisecond,
		IdleTimeout:      0,
		MaxAutoResponses: 3,
	})

	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, CreateSessionRequest{Agent: "codex", WorkDir: "."})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	_, err = client.SendText(ctx, sessionID, "convert")
	if !errors.Is(err, ErrStreamStall) {
		t.Fatalf("expected ErrStreamStall, got %v", err)
	}
}

func TestSendTextRecordsToolUseViaProgress(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{
		messageStreams: [][]sseEvent{{
			{Type: "tool_use", ToolName: "read_file"},
			{Type: "text", Content: "done"},
			{Type: "result"},
		}},
	})
	defer srv.Close()

	var logs []string
	client := NewClientWithSendOptions(srv.URL, SendOptions{
		TotalTimeout: 5 * time.Second,
		Progress: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	})

	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, CreateSessionRequest{Agent: "codex", WorkDir: "."})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if _, err := client.SendText(ctx, sessionID, "convert"); err != nil {
		t.Fatalf("SendText failed: %v", err)
	}
	if !containsProgress(logs, "step=stream_tool_use tool=read_file") {
		t.Fatalf("missing tool_use progress: %v", logs)
	}
}
