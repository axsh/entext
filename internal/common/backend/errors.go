package backend

import (
	"errors"
	"fmt"
	"strings"
)

type AttemptError struct {
	Backend string
	Err     error
}

func (e AttemptError) Error() string {
	return fmt.Sprintf("%s(%v)", e.Backend, e.Err)
}

type AggregateError struct {
	Attempts []AttemptError
}

func (e *AggregateError) Error() string {
	if e == nil || len(e.Attempts) == 0 {
		return "all backends failed"
	}
	parts := make([]string, 0, len(e.Attempts))
	for _, a := range e.Attempts {
		parts = append(parts, a.Error())
	}
	return fmt.Sprintf("all backends failed; tried: %s", strings.Join(parts, ", "))
}

func (e *AggregateError) Unwrap() error {
	if e == nil || len(e.Attempts) == 0 {
		return nil
	}
	return e.Attempts[len(e.Attempts)-1].Err
}

func NewAggregateError(attempts []AttemptError) error {
	if len(attempts) == 0 {
		return errors.New("all backends failed")
	}
	return &AggregateError{Attempts: attempts}
}
