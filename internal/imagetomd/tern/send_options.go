package tern

import "time"

type ProgressFunc func(format string, args ...any)

type SendOptions struct {
	TotalTimeout     time.Duration
	IdleTimeout      time.Duration
	MaxAutoResponses int
	Progress         ProgressFunc
}

func DefaultSendOptions() SendOptions {
	return SendOptions{
		TotalTimeout:     600 * time.Second,
		IdleTimeout:      120 * time.Second,
		MaxAutoResponses: 3,
	}
}
