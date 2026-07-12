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

func TestClassifyPrompt_ForbidsShellAndRequiresVision(t *testing.T) {
	t.Parallel()

	got := BuildClassifyPrompt("", "/tmp/a.png")
	for _, token := range []string{"shell", "Vision", "ファイル探索", "添付画像を Vision で直接"} {
		if !strings.Contains(got, token) {
			t.Fatalf("missing %q in classify prompt: %q", token, got)
		}
	}
}

func TestClassifyPrompt_MentionsSmallTableAsComplexTable(t *testing.T) {
	t.Parallel()

	got := BuildClassifyPrompt("", "/tmp/a.png")
	if !strings.Contains(got, "行数が少なくても") || !strings.Contains(got, "complex_table") {
		t.Fatalf("missing table heuristic: %q", got)
	}
}

func TestBuildClassifyRetryPrompt_IncludesReinforcement(t *testing.T) {
	t.Parallel()

	got := BuildClassifyRetryPrompt("", "/tmp/a.png")
	for _, token := range []string{"前回は計画文のみでした", "ファイル探索", "shell", "Vision で直接見て"} {
		if !strings.Contains(got, token) {
			t.Fatalf("missing %q in retry prompt: %q", token, got)
		}
	}
}

func TestSimpleTextPrompt_ForbidsShellAndRequiresVision(t *testing.T) {
	t.Parallel()

	got := BuildSimpleTextPrompt("", "/tmp/a.png")
	for _, token := range []string{"shell", "Vision", "ファイル探索", "添付画像を Vision で直接"} {
		if !strings.Contains(got, token) {
			t.Fatalf("missing %q in simple_text prompt: %q", token, got)
		}
	}
}
