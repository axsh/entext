package tern

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

type sseEvent struct {
	Type     string
	Content  string
	ToolName string
	PromptID string
	Choices  []string
}

func writeSSE(w io.Writer, events []sseEvent) {
	for _, ev := range events {
		payload := map[string]any{
			"type":    ev.Type,
			"content": ev.Content,
		}
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
}

func writeSSEStream(w http.ResponseWriter, events []sseEvent) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.WriteHeader(http.StatusOK)
	if f, ok := w.(http.Flusher); ok {
		writeSSE(w, events)
		f.Flush()
		return
	}
	writeSSE(w, events)
}

type capturedMessage struct {
	Content []map[string]any
}

func parseMessageBody(r *http.Request) (capturedMessage, error) {
	var body struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return capturedMessage{}, err
	}
	return capturedMessage{Content: body.Content}, nil
}

func hasImageContentPart(msg capturedMessage) bool {
	for _, part := range msg.Content {
		if typ, _ := part["type"].(string); typ == "image" {
			return true
		}
	}
	return false
}

type arcticTestServer struct {
	messageStreams [][]sseEvent
	respondStreams [][]sseEvent
	messageCalls   int
	respondCalls   int
	hangBody            bool
	hangAfterFirstEvent bool
	lastMessage         capturedMessage
}

func (s *arcticTestServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"session_id":"test-session"}`))
	})
	mux.HandleFunc("/api/v1/sessions/test-session/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		idx := s.messageCalls
		s.messageCalls++
		if msg, err := parseMessageBody(r); err == nil {
			s.lastMessage = msg
		}
		if s.hangAfterFirstEvent {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			payload, _ := json.Marshal(map[string]any{"type": "text", "content": "partial"})
			fmt.Fprintf(w, "data: %s\n\n", payload)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
			return
		}
		if s.hangBody {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			<-r.Context().Done()
			return
		}
		if idx >= len(s.messageStreams) {
			writeSSEStream(w, nil)
			return
		}
		writeSSEStream(w, s.messageStreams[idx])
	})
	mux.HandleFunc("/api/v1/sessions/test-session/respond", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.NotFound(w, r)
			return
		}
		idx := s.respondCalls
		s.respondCalls++
		if idx >= len(s.respondStreams) {
			writeSSEStream(w, nil)
			return
		}
		writeSSEStream(w, s.respondStreams[idx])
	})
	mux.HandleFunc("/api/v1/sessions/test-session/terminate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func newArcticTestServer(s *arcticTestServer) *httptest.Server {
	return httptest.NewServer(s.handler())
}

func containsProgress(logs []string, substr string) bool {
	for _, line := range logs {
		if strings.Contains(line, substr) {
			return true
		}
	}
	return false
}
