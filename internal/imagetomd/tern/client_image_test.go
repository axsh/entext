package tern

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestPNG(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "sample.png")
	// Minimal valid PNG header + IHDR chunk (1x1) for http.DetectContentType.
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
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write png: %v", err)
	}
	return path
}

func TestSendImagePromptIncludesImageContentPart(t *testing.T) {
	t.Parallel()

	srvState := &arcticTestServer{
		messageStreams: [][]sseEvent{{{Type: "text", Content: "simple_text"}, {Type: "result"}}},
	}
	srv := newArcticTestServer(srvState)
	defer srv.Close()

	imagePath := writeTestPNG(t, t.TempDir())
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

	got, err := client.SendImagePrompt(ctx, sessionID, "classify", imagePath)
	if err != nil {
		t.Fatalf("SendImagePrompt failed: %v", err)
	}
	if got != "simple_text" {
		t.Fatalf("unexpected response: %q", got)
	}
	if !hasImageContentPart(srvState.lastMessage) {
		t.Fatalf("expected image content part in POST body: %+v", srvState.lastMessage)
	}
}

func TestSendImagePromptReturnsErrImageReadFailedOnMissingFile(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{})
	defer srv.Close()

	client := NewClient(srv.URL)
	ctx := context.Background()
	sessionID, err := client.CreateSession(ctx, CreateSessionRequest{Agent: "codex", WorkDir: "."})
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	_, err = client.SendImagePrompt(ctx, sessionID, "classify", filepath.Join(t.TempDir(), "missing.png"))
	if !errors.Is(err, ErrImageReadFailed) {
		t.Fatalf("expected ErrImageReadFailed, got %v", err)
	}
}

func TestSendImagePromptHandlesUserInputRequiredThenCompletes(t *testing.T) {
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

	imagePath := writeTestPNG(t, t.TempDir())
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

	got, err := client.SendImagePrompt(ctx, sessionID, "convert", imagePath)
	if err != nil {
		t.Fatalf("SendImagePrompt failed: %v", err)
	}
	if got == "" || !containsSubstring(got, "| 選択 |") {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestSendImagePromptReturnsErrStreamStallOnIdleTimeout(t *testing.T) {
	t.Parallel()

	srv := newArcticTestServer(&arcticTestServer{hangAfterFirstEvent: true})
	defer srv.Close()

	imagePath := writeTestPNG(t, t.TempDir())
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

	_, err = client.SendImagePrompt(ctx, sessionID, "convert", imagePath)
	if !errors.Is(err, ErrStreamStall) {
		t.Fatalf("expected ErrStreamStall, got %v", err)
	}
}

func containsSubstring(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexSubstring(s, sub))
}

func indexSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
