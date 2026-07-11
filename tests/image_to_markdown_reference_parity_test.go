package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReferenceParity_ChangeHistoryGoldenContract(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "01_変更履歴.md", []string{
		"# 変更履歴",
		"| No. | 変更箇所 | 変更内容(変更理由) |",
		"| 43 |",
		"| 44 |",
	}, []string{
		"Q:",
		"A:",
		"### Phase",
		"Phase 1",
	})
}

func TestReferenceParity_RewriteRulesGoldenContract(t *testing.T) {
	t.Parallel()
	assertReferenceMarkdownContract(t, "02_書き換えルール.md", []string{
		"# 書き換えルール",
		"| 見出し行（列名定義） |",
		"[SUB_TABLE_P01]",
	}, []string{
		"Q:",
		"A:",
		"### Phase",
		"Phase 1",
	})
}

func assertReferenceMarkdownContract(t *testing.T, name string, required []string, forbidden []string) {
	t.Helper()
	path := filepath.Join("testdata", "reference_parity", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read golden markdown %s: %v", path, err)
	}
	text := strings.TrimSpace(string(raw))
	if text == "" {
		t.Fatalf("golden markdown is empty: %s", path)
	}
	for _, token := range required {
		if !strings.Contains(text, token) {
			t.Fatalf("missing required token %q in %s", token, path)
		}
	}
	for _, token := range forbidden {
		if strings.Contains(text, token) {
			t.Fatalf("forbidden token %q found in %s", token, path)
		}
	}
}
