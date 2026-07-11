package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSetCommandRequiresProvider(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
	cmd.SetArgs([]string{"set", "--name", "default", "--secret", "abc"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected provider required error")
	}
}

func TestSetCommandInvalidProvider(t *testing.T) {
	t.Parallel()
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd := newRootCmd(strings.NewReader(""), &out, &errOut)
	cmd.SetArgs([]string{"set", "--provider", "unknown", "--name", "default", "--secret", "abc"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected invalid provider error")
	}
	if !strings.Contains(err.Error(), "provider must be openai or anthropic") {
		t.Fatalf("unexpected error: %v", err)
	}
}
