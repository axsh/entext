package tern

import (
	"errors"
	"fmt"
	"testing"

	arcticclient "github.com/axsh/arctic-tern/client/v1"
)

func TestUnattendedInputHandler_AutoResponseFreeText(t *testing.T) {
	t.Parallel()

	h := NewUnattendedInputHandler(3, nil)
	got, err := h.Handle(arcticclient.UserInputRequiredEvent{Content: "Please confirm the column layout."})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if got != UnattendedAutoResponse {
		t.Fatalf("unexpected response: %q", got)
	}
}

func TestUnattendedInputHandler_PicksFirstChoice(t *testing.T) {
	t.Parallel()

	h := NewUnattendedInputHandler(3, nil)
	got, err := h.Handle(arcticclient.UserInputRequiredEvent{
		Content:  "Which column?",
		PromptID: "p1",
		Choices:  []string{"colA", "colB"},
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if got != "colA" {
		t.Fatalf("unexpected response: %q", got)
	}
	events := h.Events()
	if len(events) != 1 || events[0].PickedIndex != 0 || events[0].ChoicesCount != 2 {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestUnattendedInputHandler_ExceedsMaxReturnsError(t *testing.T) {
	t.Parallel()

	h := NewUnattendedInputHandler(3, nil)
	ev := arcticclient.UserInputRequiredEvent{Content: "confirm?"}
	for i := 0; i < 3; i++ {
		if _, err := h.Handle(ev); err != nil {
			t.Fatalf("Handle %d failed: %v", i+1, err)
		}
	}
	_, err := h.Handle(ev)
	if !errors.Is(err, ErrInteractiveInputRequired) {
		t.Fatalf("expected ErrInteractiveInputRequired, got %v", err)
	}
}

func TestUnattendedInputHandler_RecordsGuardEvents(t *testing.T) {
	t.Parallel()

	var logs []string
	h := NewUnattendedInputHandler(3, func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	})
	_, err := h.Handle(arcticclient.UserInputRequiredEvent{
		Content:  "Which?",
		PromptID: "pid-1",
		Choices:  []string{"A"},
	})
	if err != nil {
		t.Fatalf("Handle failed: %v", err)
	}
	if len(h.Events()) != 1 {
		t.Fatalf("expected one guard event")
	}
	if len(logs) != 1 || logs[0] == "" {
		t.Fatalf("expected progress log, got %v", logs)
	}
}
