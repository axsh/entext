package tern

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	arcticserver "github.com/axsh/arctic-tern/server"
	arcticconfig "github.com/axsh/arctic-tern/shared/libs/go/config"
)

type Mode string

const (
	ModeAuto     Mode = "auto"
	ModeExternal Mode = "external"
	ModeInProc   Mode = "inproc"
)

type RuntimeRequest struct {
	Mode           Mode
	ExternalServer string
	ConfigPath     string
	Agent          string
	Model          string
	WorkingDir     string
}

type Runtime struct {
	Client   Client
	Endpoint string
	ModeUsed Mode

	shutdownOnce sync.Once
	shutdownFn   func(context.Context) error
	shutdownErr  error
}

func BuildRuntime(ctx context.Context, req RuntimeRequest) (*Runtime, error) {
	mode, err := resolveMode(req.Mode)
	if err != nil {
		return nil, err
	}
	serverURL := strings.TrimSpace(req.ExternalServer)
	if serverURL == "" {
		serverURL = "http://localhost:3100"
	}
	switch mode {
	case ModeExternal:
		return buildExternalRuntime(ctx, serverURL)
	case ModeInProc:
		return buildInProcRuntime(ctx, req)
	case ModeAuto:
		extRuntime, err := buildExternalRuntime(ctx, serverURL)
		if err == nil {
			return extRuntime, nil
		}
		return buildInProcRuntime(ctx, req)
	default:
		return nil, fmt.Errorf("invalid tern mode: %s", mode)
	}
}

func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil || r.shutdownFn == nil {
		return nil
	}
	r.shutdownOnce.Do(func() {
		r.shutdownErr = r.shutdownFn(ctx)
	})
	return r.shutdownErr
}

func resolveMode(mode Mode) (Mode, error) {
	if mode == "" {
		return ModeAuto, nil
	}
	switch mode {
	case ModeAuto, ModeExternal, ModeInProc:
		return mode, nil
	default:
		return "", fmt.Errorf("tern mode must be auto, external, or inproc")
	}
}

func buildExternalRuntime(ctx context.Context, serverURL string) (*Runtime, error) {
	client := NewClient(serverURL)
	if err := client.Health(ctx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConnectFailed, err)
	}
	return &Runtime{
		Client:   client,
		Endpoint: serverURL,
		ModeUsed: ModeExternal,
		shutdownFn: func(context.Context) error {
			return nil
		},
	}, nil
}

func buildInProcRuntime(ctx context.Context, req RuntimeRequest) (*Runtime, error) {
	cfg, err := LoadInProcessConfig(LoadConfigInput{
		ExplicitPath: req.ConfigPath,
		WorkingDir:   req.WorkingDir,
	})
	if err != nil {
		return nil, err
	}
	appCfg, err := arcticconfig.Load(cfg.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBootFailed, err)
	}
	appCfg.LLMGateway.Port = cfg.Port
	appCfg.LLMGateway.ModelProfilesPath = cfg.ModelProfilesPath

	options := []arcticserver.Option{
		arcticserver.WithConfig(appCfg),
	}
	if strings.EqualFold(cfg.VaultBackend, "keyring") {
		options = append(options, arcticserver.WithKeyringVault())
	}
	srv, err := arcticserver.New(options...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrBootFailed, err)
	}
	launchCtx, cancel := context.WithCancel(context.Background())
	launchErrCh := make(chan error, 1)
	go func() {
		launchErrCh <- srv.Launch(launchCtx)
	}()
	agentPort, err := waitForAgentPort(ctx, srv, 20*time.Second)
	if err != nil {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = srv.Shutdown(stopCtx)
		return nil, fmt.Errorf("%w: %v", ErrBootFailed, err)
	}
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", agentPort)
	if err := waitForHealth(ctx, endpoint, 20*time.Second); err != nil {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer stopCancel()
		_ = srv.Shutdown(stopCtx)
		select {
		case launchErr := <-launchErrCh:
			if launchErr != nil && !errors.Is(launchErr, context.Canceled) {
				return nil, fmt.Errorf("%w: %v", ErrBootFailed, launchErr)
			}
		default:
		}
		return nil, fmt.Errorf("%w: %v", ErrBootFailed, err)
	}
	return &Runtime{
		Client:   NewClient(endpoint),
		Endpoint: endpoint,
		ModeUsed: ModeInProc,
		shutdownFn: func(ctx context.Context) error {
			cancel()
			shutdownErr := srv.Shutdown(ctx)
			launchErr := <-launchErrCh
			if shutdownErr != nil {
				return shutdownErr
			}
			if launchErr != nil && !errors.Is(launchErr, context.Canceled) {
				return launchErr
			}
			return nil
		},
	}, nil
}

func waitForAgentPort(ctx context.Context, srv *arcticserver.Server, timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
		}
		port := srv.AgentService().Port()
		if port > 0 {
			return port, nil
		}
		if time.Now().After(deadline) {
			return 0, errors.New("agent service port was not assigned")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForHealth(ctx context.Context, endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	httpClient := &http.Client{Timeout: 2 * time.Second}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		c := NewClientWithHTTP(endpoint, httpClient)
		_, err := c.client.Health(ctx)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check timeout for %s", endpoint)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
