package io

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
)

type ResolveInputArgs struct {
	InputPath string
	UseStdin  bool
	Stdin     io.Reader
}

func ValidateStdinReady(useStdin bool, stdin *os.File) error {
	if !useStdin {
		return nil
	}
	if stdin == nil {
		return apperr.NewValidationError(errors.New("stdin is unavailable"))
	}
	info, err := stdin.Stat()
	if err != nil {
		return err
	}
	if (info.Mode() & os.ModeCharDevice) != 0 {
		return apperr.NewValidationError(errors.New("stdin is not piped"))
	}
	return nil
}

func ResolveInputPaths(args ResolveInputArgs) ([]string, error) {
	input := strings.TrimSpace(args.InputPath)
	if input != "" && args.UseStdin {
		return nil, apperr.NewValidationError(apperr.ErrInvalidArgs)
	}
	if input == "" && !args.UseStdin {
		return nil, apperr.NewValidationError(apperr.ErrInputRequired)
	}
	if input != "" {
		return []string{filepath.Clean(input)}, nil
	}
	if args.Stdin == nil {
		return nil, apperr.NewValidationError(errors.New("stdin is required when --stdin is set"))
	}

	scanner := bufio.NewScanner(args.Stdin)
	seen := map[string]struct{}{}
	out := make([]string, 0)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		cleaned := filepath.Clean(line)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, apperr.NewValidationError(apperr.ErrInputRequired)
	}
	return out, nil
}
