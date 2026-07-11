package tern

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	arcticclient "github.com/axsh/arctic-tern/client/v1"
)

type Client interface {
	CreateSession(ctx context.Context, req CreateSessionRequest) (string, error)
	SendText(ctx context.Context, sessionID string, text string) (string, error)
	TerminateSession(ctx context.Context, sessionID string) error
}

type CreateSessionRequest struct {
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	WorkDir string `json:"work_dir"`
}

type ArcticClient struct {
	client   *arcticclient.Client
	mu       sync.Mutex
	sessions map[string]*arcticclient.Session
}

func NewClient(baseURL string) *ArcticClient {
	return &ArcticClient{
		client: arcticclient.New(
			strings.TrimRight(baseURL, "/"),
			arcticclient.WithNoTimeout(),
		),
		sessions: make(map[string]*arcticclient.Session),
	}
}

func NewClientWithHTTP(baseURL string, hc *http.Client) *ArcticClient {
	return &ArcticClient{
		client: arcticclient.New(
			strings.TrimRight(baseURL, "/"),
			arcticclient.WithHTTPClient(hc),
		),
		sessions: make(map[string]*arcticclient.Session),
	}
}

func (c *ArcticClient) Health(ctx context.Context) error {
	_, err := c.client.Health(ctx)
	return err
}

func (c *ArcticClient) CreateSession(ctx context.Context, req CreateSessionRequest) (string, error) {
	session, err := c.client.CreateSession(ctx, arcticclient.SessionRequest{
		Agent:   req.Agent,
		Model:   req.Model,
		WorkDir: req.WorkDir,
	})
	if err != nil {
		return "", err
	}
	if session == nil || session.ID == "" {
		return "", fmt.Errorf("empty session id")
	}
	c.mu.Lock()
	c.sessions[session.ID] = session
	c.mu.Unlock()
	return session.ID, nil
}

func (c *ArcticClient) SendText(ctx context.Context, sessionID string, text string) (string, error) {
	session, err := c.getSession(sessionID)
	if err != nil {
		return "", err
	}
	stream, err := session.SendText(ctx, text)
	if err != nil {
		return "", err
	}
	var (
		texts      []string
		resultText string
		streamErr  string
	)
	stream.OnText(func(textChunk string) {
		if strings.TrimSpace(textChunk) == "" {
			return
		}
		texts = append(texts, textChunk)
	}).OnResult(func(ev arcticclient.Event) {
		if strings.TrimSpace(ev.Text) != "" {
			resultText = ev.Text
		}
	}).OnError(func(errMsg string) {
		streamErr = errMsg
	})
	if err := stream.Run(); err != nil {
		return "", err
	}
	if streamErr != "" {
		return "", fmt.Errorf("session stream error: %s", streamErr)
	}
	return finalizeResponse(texts, resultText), nil
}

func (c *ArcticClient) TerminateSession(ctx context.Context, sessionID string) error {
	session, err := c.getSession(sessionID)
	if err != nil {
		return err
	}
	if err := session.Terminate(ctx); err != nil {
		return err
	}
	c.mu.Lock()
	delete(c.sessions, sessionID)
	c.mu.Unlock()
	return nil
}

func (c *ArcticClient) getSession(sessionID string) (*arcticclient.Session, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s, ok := c.sessions[sessionID]
	if !ok || s == nil {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return s, nil
}

var _ Client = (*ArcticClient)(nil)

func finalizeResponse(texts []string, resultText string) string {
	joined := strings.TrimSpace(strings.Join(texts, "\n"))
	if joined != "" {
		return joined
	}
	return strings.TrimSpace(resultText)
}
