package io

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/logger"
)

type OutputMode string

const (
	OutputModePath OutputMode = "path"
	OutputModeJSON OutputMode = "json"
)

func ResolveOutputMode(outputMode string, printJSON bool) (OutputMode, error) {
	if printJSON {
		return OutputModeJSON, nil
	}
	mode := OutputMode(strings.TrimSpace(outputMode))
	if mode == "" {
		return OutputModePath, nil
	}
	switch mode {
	case OutputModePath, OutputModeJSON:
		return mode, nil
	default:
		return "", apperr.NewValidationError(fmt.Errorf("unsupported output mode: %s", outputMode))
	}
}

func WriteResultPaths(w io.Writer, mode OutputMode, paths []string) error {
	switch mode {
	case OutputModePath:
		for _, p := range paths {
			if _, err := fmt.Fprintln(w, p); err != nil {
				return err
			}
		}
		return nil
	case OutputModeJSON:
		enc := json.NewEncoder(w)
		return enc.Encode(paths)
	default:
		return apperr.NewValidationError(fmt.Errorf("unsupported output mode: %s", mode))
	}
}

func NewLogger(stderr io.Writer, verbose bool, quiet bool) *slog.Logger {
	level := slog.LevelWarn
	if quiet {
		level = slog.LevelWarn
	} else if verbose {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}
	return logger.New(stderr, level)
}
