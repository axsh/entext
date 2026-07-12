package analyzer

import (
	"strings"
	"testing"
)

func TestWrapNonInteractivePrompt_AppendsSuffixOnce(t *testing.T) {
	t.Parallel()

	got := WrapNonInteractivePrompt("base prompt")
	if !strings.Contains(got, "base prompt") || !strings.Contains(got, "UNATTENDED BATCH MODE") {
		t.Fatalf("unexpected prompt: %q", got)
	}
}

func TestWrapNonInteractivePrompt_Idempotent(t *testing.T) {
	t.Parallel()

	once := WrapNonInteractivePrompt("base")
	twice := WrapNonInteractivePrompt(once)
	if once != twice {
		t.Fatalf("expected idempotent wrap")
	}
}

func TestBuildSimpleTextPrompt_IncludesImageAndSuffix(t *testing.T) {
	t.Parallel()

	got := BuildSimpleTextPrompt("", "/tmp/image.png")
	if !strings.Contains(got, "[Attached image: /tmp/image.png]") {
		t.Fatalf("missing attached image line: %q", got)
	}
	if !strings.Contains(got, "UNATTENDED BATCH MODE") {
		t.Fatalf("missing non-interactive suffix: %q", got)
	}
}

func TestAssessGapPrompt_IncludesNonInteractiveSuffix(t *testing.T) {
	t.Parallel()

	got := AssessGapPrompt(DefaultPhases[0], "known")
	if !strings.Contains(got, "UNATTENDED BATCH MODE") {
		t.Fatalf("missing suffix: %q", got)
	}
}

func TestBuildClassifyPrompt_IncludesImageAndSuffix(t *testing.T) {
	t.Parallel()

	got := BuildClassifyPrompt("ref", "/tmp/a.png")
	if !strings.Contains(got, "[Attached image:") || !strings.Contains(got, "UNATTENDED BATCH MODE") {
		t.Fatalf("unexpected classify prompt: %q", got)
	}
}
