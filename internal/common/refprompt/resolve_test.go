package refprompt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/axsh/entext/internal/common/apperr"
)

func TestResolveHintsEmpty(t *testing.T) {
	b, err := Resolve(HintInput{})
	if err != nil {
		t.Fatalf("Resolve empty: %v", err)
	}
	if len(b.Refs) != 0 || b.Prompts != "" {
		t.Fatalf("expected empty bundle, got refs=%d prompts=%q", len(b.Refs), b.Prompts)
	}
}

func TestResolveHintsRefPatternsMatchAndDedupe(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "docs")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sub, "hint.md")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel := filepath.ToSlash(filepath.Join("docs", "hint.md"))

	b, err := Resolve(HintInput{
		Root:        root,
		RefPatterns: []string{`docs/.*\.md`, `docs/hint\.md`},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(b.Refs) != 1 {
		t.Fatalf("expected 1 ref after dedupe, got %d", len(b.Refs))
	}
	if !strings.HasSuffix(filepath.ToSlash(b.Refs[0].Path), rel) && !strings.Contains(filepath.ToSlash(b.Refs[0].Path), "hint.md") {
		t.Fatalf("unexpected path %q", b.Refs[0].Path)
	}
	if b.Refs[0].Content != "hello" {
		t.Fatalf("content=%q", b.Refs[0].Content)
	}
}

func TestResolveHintsRefDirRecursiveCollectsMarkdown(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "deep.md"), []byte("deep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a", "top.md"), []byte("top"), 0o644); err != nil {
		t.Fatal(err)
	}

	b, err := Resolve(HintInput{RefDirs: []string{filepath.Join(root, "a")}})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(b.Refs) != 2 {
		t.Fatalf("expected 2 markdown files, got %d", len(b.Refs))
	}
}

func TestResolveHintsPromptFilesConcatOrder(t *testing.T) {
	dir := t.TempDir()
	f1 := filepath.Join(dir, "p1.txt")
	f2 := filepath.Join(dir, "p2.txt")
	if err := os.WriteFile(f1, []byte("file1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f2, []byte("file2"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(HintInput{
		Prompts:     []string{"inline-a", "inline-b"},
		PromptFiles: []string{f1, f2},
	})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := "inline-a\n\ninline-b\n\nfile1\n\nfile2"
	if b.Prompts != want {
		t.Fatalf("prompts=\n%q\nwant\n%q", b.Prompts, want)
	}
}

func TestResolveHintsInlinePromptsConcatOrder(t *testing.T) {
	b, err := Resolve(HintInput{Prompts: []string{"one", "two"}})
	if err != nil {
		t.Fatal(err)
	}
	if b.Prompts != "one\n\ntwo" {
		t.Fatalf("got %q", b.Prompts)
	}
}

func TestResolveHintsMissingPromptFileReturnsValidationError(t *testing.T) {
	_, err := Resolve(HintInput{PromptFiles: []string{filepath.Join(t.TempDir(), "missing.txt")}})
	if err == nil {
		t.Fatal("expected error")
	}
	if !apperr.IsValidationError(err) {
		t.Fatalf("expected validation error, got %T %v", err, err)
	}
}

func TestResolveHintsInvalidRegexpReturnsError(t *testing.T) {
	_, err := Resolve(HintInput{Root: t.TempDir(), RefPatterns: []string{"[invalid"}})
	if err == nil {
		t.Fatal("expected regexp error")
	}
}

func TestResolveHintsSkipsNonMarkdownInRefDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("md"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := Resolve(HintInput{RefDirs: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Refs) != 1 {
		t.Fatalf("expected only .md, got %d", len(b.Refs))
	}
}

func TestFormatForPromptAnalyzeIncludesPolicy(t *testing.T) {
	text := FormatForPrompt(HintBundle{
		Refs: []RefDocument{{Path: "/tmp/a.md", Content: "ref-body"}},
		Prompts: "extra",
	}, ModeAnalyze)
	if !strings.Contains(text, HintPolicyAnalyze) {
		t.Fatal("missing analyze policy")
	}
	if !strings.Contains(text, "[Reference markdown context]") {
		t.Fatal("missing ref header")
	}
	if !strings.Contains(text, "[Additional prompt hints]") {
		t.Fatal("missing prompt header")
	}
	if !strings.Contains(text, "ref-body") || !strings.Contains(text, "extra") {
		t.Fatal("missing content")
	}
}
