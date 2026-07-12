package tern

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	arcticclient "github.com/axsh/arctic-tern/client/v1"
)

type Client interface {
	CreateSession(ctx context.Context, req CreateSessionRequest) (string, error)
	SendText(ctx context.Context, sessionID string, text string) (string, error)
	TerminateSession(ctx context.Context, sessionID string) error
	LastSendGuardEvents() []AgentGuardEvent
}

type CreateSessionRequest struct {
	Agent   string `json:"agent"`
	Model   string `json:"model"`
	WorkDir string `json:"work_dir"`
}

type ArcticClient struct {
	client          *arcticclient.Client
	mu              sync.Mutex
	sessions        map[string]*arcticclient.Session
	opts            SendOptions
	lastGuardEvents []AgentGuardEvent
}

func NewClient(baseURL string) *ArcticClient {
	return NewClientWithSendOptions(baseURL, DefaultSendOptions())
}

func NewClientWithHTTP(baseURL string, hc *http.Client) *ArcticClient {
	if hc == nil {
		hc = &http.Client{Timeout: 0}
	}
	return &ArcticClient{
		client: arcticclient.New(
			strings.TrimRight(baseURL, "/"),
			arcticclient.WithHTTPClient(hc),
		),
		sessions: make(map[string]*arcticclient.Session),
		opts:     DefaultSendOptions(),
	}
}

func NewClientWithSendOptions(baseURL string, opts SendOptions) *ArcticClient {
	if opts.MaxAutoResponses <= 0 {
		opts.MaxAutoResponses = DefaultSendOptions().MaxAutoResponses
	}
	if opts.TotalTimeout <= 0 {
		opts.TotalTimeout = DefaultSendOptions().TotalTimeout
	}
	return &ArcticClient{
		client: arcticclient.New(
			strings.TrimRight(baseURL, "/"),
			arcticclient.WithHTTPClient(&http.Client{Timeout: 0}),
		),
		sessions: make(map[string]*arcticclient.Session),
		opts:     opts,
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

	sendCtx, cancel := context.WithTimeout(ctx, c.opts.TotalTimeout)
	defer cancel()

	var (
		texts      []string
		textsMu    sync.Mutex
		lastEvent  atomic.Int64
		handler    = NewUnattendedInputHandler(c.opts.MaxAutoResponses, c.opts.Progress)
		idleStop   chan struct{}
	)

	lastEvent.Store(time.Now().UnixNano())
	if c.opts.IdleTimeout > 0 {
		idleCtx, idleCancel := context.WithCancel(sendCtx)
		sendCtx = idleCtx
		idleStop = make(chan struct{})
		go c.runIdleWatchdog(idleCtx, idleCancel, &lastEvent, c.opts.IdleTimeout, idleStop)
	}

	handlers := arcticclient.StreamHandlers{
		OnText: func(textChunk string) {
			lastEvent.Store(time.Now().UnixNano())
			if strings.TrimSpace(textChunk) == "" {
				return
			}
			textsMu.Lock()
			texts = append(texts, textChunk)
			textsMu.Unlock()
		},
		OnToolUse: func(toolName string) {
			lastEvent.Store(time.Now().UnixNano())
			if c.opts.Progress != nil {
				c.opts.Progress("step=stream_tool_use tool=%s", toolName)
			}
		},
		OnToolResult: func(content string) {
			lastEvent.Store(time.Now().UnixNano())
			_ = content
		},
		OnUserInputRequired: func(ev arcticclient.UserInputRequiredEvent) (string, error) {
			lastEvent.Store(time.Now().UnixNano())
			return handler.Handle(ev)
		},
		OnError: func(errMsg string) error {
			lastEvent.Store(time.Now().UnixNano())
			return fmt.Errorf("%s", errMsg)
		},
		OnResult: func() {
			lastEvent.Store(time.Now().UnixNano())
		},
	}

	err = session.SendTextWithHandlers(sendCtx, text, handlers)
	if idleStop != nil {
		close(idleStop)
	}

	c.mu.Lock()
	c.lastGuardEvents = handler.Events()
	c.mu.Unlock()

	if err != nil {
		if errors.Is(err, ErrInteractiveInputRequired) {
			return "", err
		}
		if errors.Is(sendCtx.Err(), context.DeadlineExceeded) || errors.Is(sendCtx.Err(), context.Canceled) {
			return "", ErrStreamStall
		}
		return "", err
	}
	if sendCtx.Err() != nil {
		return "", ErrStreamStall
	}

	textsMu.Lock()
	defer textsMu.Unlock()
	return finalizeResponse(texts, ""), nil
}

func (c *ArcticClient) LastSendGuardEvents() []AgentGuardEvent {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.lastGuardEvents) == 0 {
		return nil
	}
	out := make([]AgentGuardEvent, len(c.lastGuardEvents))
	copy(out, c.lastGuardEvents)
	return out
}

func (c *ArcticClient) runIdleWatchdog(ctx context.Context, cancel context.CancelFunc, lastEvent *atomic.Int64, idleTimeout time.Duration, stop <-chan struct{}) {
	tick := idleTimeout / 4
	if tick < 10*time.Millisecond {
		tick = 10 * time.Millisecond
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			last := time.Unix(0, lastEvent.Load())
			if time.Since(last) >= idleTimeout {
				cancel()
				return
			}
		}
	}
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
