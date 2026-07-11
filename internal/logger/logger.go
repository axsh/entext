package logger

import (
	"io"
	"log/slog"
)

func New(stderr io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewTextHandler(stderr, &slog.HandlerOptions{
		Level: level,
	}))
}
