package backend

import (
	"errors"
	"strings"
	"testing"
)

func TestAggregateErrorFormatting(t *testing.T) {
	t.Parallel()
	err := NewAggregateError([]AttemptError{
		{Backend: "excel-com", Err: errors.New("not available")},
		{Backend: "libreoffice", Err: errors.New("not found")},
	})
	if err == nil {
		t.Fatalf("expected aggregate error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "all backends failed") {
		t.Fatalf("missing aggregate header: %s", msg)
	}
	if !strings.Contains(msg, "excel-com(") || !strings.Contains(msg, "libreoffice(") {
		t.Fatalf("missing backend attempts: %s", msg)
	}
}
