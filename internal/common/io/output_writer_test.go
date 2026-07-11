package io

import (
	"bytes"
	"strings"
	"testing"
)

func TestResolveOutputMode(t *testing.T) {
	t.Parallel()

	mode, err := ResolveOutputMode("", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != OutputModePath {
		t.Fatalf("unexpected mode: %s", mode)
	}

	mode, err = ResolveOutputMode("json", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != OutputModeJSON {
		t.Fatalf("unexpected mode: %s", mode)
	}

	mode, err = ResolveOutputMode("path", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mode != OutputModeJSON {
		t.Fatalf("print-json should force json mode")
	}
}

func TestWriteResultPaths(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}
	err := WriteResultPaths(buf, OutputModePath, []string{"a.pdf", "b.pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := buf.String(); got != "a.pdf\nb.pdf\n" {
		t.Fatalf("unexpected path output: %q", got)
	}

	buf.Reset()
	err = WriteResultPaths(buf, OutputModeJSON, []string{"a.pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "[\"a.pdf\"]") {
		t.Fatalf("unexpected json output: %q", buf.String())
	}
}
