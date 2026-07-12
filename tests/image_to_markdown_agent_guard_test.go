package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

type agentGuardSSEEvent struct {
	Type     string
	Content  string
	ToolName string
	PromptID string
	Choices  []string
}

func writeAgentGuardSSE(w http.ResponseWriter, events []agentGuardSSEEvent) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	for _, ev := range events {
		payload := map[string]any{"type": ev.Type, "content": ev.Content}
		if ev.ToolName != "" {
			payload["tool_name"] = ev.ToolName
		}
		if ev.PromptID != "" {
			payload["prompt_id"] = ev.PromptID
		}
		if len(ev.Choices) > 0 {
			payload["choices"] = ev.Choices
		}
		data, _ := json.Marshal(payload)
		fmt.Fprintf(w, "data: %s\n\n", data)
	}
	fmt.Fprintf(w, "data: [DONE]\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func newAgentGuardMockTern(messageStreams, respondStreams [][]agentGuardSSEEvent) *httptest.Server {
	messageCalls := 0
	respondCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"session_id":"test-session"}`))
	})
	mux.HandleFunc("/api/v1/sessions/test-session/messages", func(w http.ResponseWriter, r *http.Request) {
		idx := messageCalls
		messageCalls++
		if idx >= len(messageStreams) {
			writeAgentGuardSSE(w, nil)
			return
		}
		writeAgentGuardSSE(w, messageStreams[idx])
	})
	mux.HandleFunc("/api/v1/sessions/test-session/respond", func(w http.ResponseWriter, r *http.Request) {
		idx := respondCalls
		respondCalls++
		if idx >= len(respondStreams) {
			writeAgentGuardSSE(w, nil)
			return
		}
		writeAgentGuardSSE(w, respondStreams[idx])
	})
	mux.HandleFunc("/api/v1/sessions/test-session/terminate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return httptest.NewServer(mux)
}

func TestTernClientUserInputRequiredIntegration(t *testing.T) {
	t.Parallel()

	srv := newAgentGuardMockTern(
		[][]agentGuardSSEEvent{
			{{Type: "text", Content: "simple_text"}},
			{{Type: "user_input_required", Content: "Which column?", PromptID: "p1", Choices: []string{"colA", "colB"}}},
		},
		[][]agentGuardSSEEvent{
			{
				{Type: "text", Content: "| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |"},
				{Type: "result"},
			},
		},
	)
	defer srv.Close()

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "sample.png")
	if err := os.WriteFile(imagePath, []byte("png"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}
	outDir := filepath.Join(dir, "md")

	artifact, err := entext.ConvertImageToMarkdown(context.Background(), entext.ImageToMarkdownJob{
		InputPath: imagePath,
		OutputDir: outDir,
	}, entext.ImageToMarkdownConfig{
		ServerURL:    srv.URL,
		TernMode:     "external",
		RoundSleepMS: 1,
		PhaseSleepMS: 1,
	})
	if err != nil {
		t.Fatalf("ConvertImageToMarkdown failed: %v", err)
	}
	mdBytes, err := os.ReadFile(artifact.MarkdownPath)
	if err != nil {
		t.Fatalf("read markdown: %v", err)
	}
	md := string(mdBytes)
	if !strings.Contains(md, "| 選択 |") || !strings.Contains(md, "プレナビ") {
		t.Fatalf("unexpected markdown: %s", md)
	}
	sessionBytes, err := os.ReadFile(artifact.SessionPath)
	if err != nil {
		t.Fatalf("read session: %v", err)
	}
	if !strings.Contains(string(sessionBytes), "agent_guard_events") {
		t.Fatalf("expected agent_guard_events in session log: %s", string(sessionBytes))
	}
}
