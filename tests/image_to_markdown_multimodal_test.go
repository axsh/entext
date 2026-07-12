package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext"
)

type multimodalCapturedMessage struct {
	Content []map[string]any
}

func parseMultimodalMessageBody(r *http.Request) (multimodalCapturedMessage, error) {
	var body struct {
		Content []map[string]any `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return multimodalCapturedMessage{}, err
	}
	return multimodalCapturedMessage{Content: body.Content}, nil
}

func hasMultimodalImagePart(msg multimodalCapturedMessage) bool {
	for _, part := range msg.Content {
		if typ, _ := part["type"].(string); typ == "image" {
			return true
		}
	}
	return false
}

func writeMinimalPNG(path string) error {
	data := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	return os.WriteFile(path, data, 0o644)
}

func newMultimodalMockTern(messageStreams, respondStreams [][]agentGuardSSEEvent, captured *[]multimodalCapturedMessage) *httptest.Server {
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
		if captured != nil {
			if msg, err := parseMultimodalMessageBody(r); err == nil {
				*captured = append(*captured, msg)
			}
		}
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

func TestImageToMarkdownMultimodalClassifyIntegration(t *testing.T) {
	t.Parallel()

	var captured []multimodalCapturedMessage
	srv := newMultimodalMockTern(
		[][]agentGuardSSEEvent{
			{{Type: "text", Content: "simple_text"}},
			{{Type: "text", Content: "| 選択 | 列番号 |\n|---|---|\n| プレナビ | 44 |"}, {Type: "result"}},
		},
		nil,
		&captured,
	)
	defer srv.Close()

	dir := t.TempDir()
	imagePath := filepath.Join(dir, "sample.png")
	if err := writeMinimalPNG(imagePath); err != nil {
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
	if len(captured) == 0 {
		t.Fatalf("expected captured multimodal POST bodies")
	}
	imageFound := false
	for i, msg := range captured {
		if hasMultimodalImagePart(msg) {
			imageFound = true
			continue
		}
		t.Logf("message %d body without image: %+v", i, msg)
	}
	if !imageFound {
		t.Fatalf("expected at least one POST with type:image content part")
	}
}
