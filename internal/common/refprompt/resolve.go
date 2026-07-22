package refprompt

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/axsh/entext/internal/common/apperr"
	"github.com/axsh/entext/internal/imagetomd/refresolver"
)

type HintInput struct {
	RefPatterns []string
	RefDirs     []string
	Prompts     []string
	PromptFiles []string
	Root        string // default "."
}

type HintBundle struct {
	Refs    []refresolver.RefDocument
	Prompts string
}

// RefDocument is an alias for callers that do not import refresolver.
type RefDocument = refresolver.RefDocument

func Resolve(in HintInput) (HintBundle, error) {
	root := strings.TrimSpace(in.Root)
	if root == "" {
		root = "."
	}

	seen := map[string]struct{}{}
	docs := make([]refresolver.RefDocument, 0)

	fromPatterns, err := refresolver.ResolveRefs(in.RefPatterns, root)
	if err != nil {
		return HintBundle{}, err
	}
	for _, d := range fromPatterns {
		abs := filepath.Clean(d.Path)
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		docs = append(docs, refresolver.RefDocument{Path: abs, Content: d.Content})
	}

	for _, dir := range in.RefDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if strings.ToLower(filepath.Ext(path)) != ".md" {
				return nil
			}
			abs := filepath.Clean(path)
			if _, ok := seen[abs]; ok {
				return nil
			}
			content, readErr := os.ReadFile(abs)
			if readErr != nil {
				return readErr
			}
			seen[abs] = struct{}{}
			docs = append(docs, refresolver.RefDocument{Path: abs, Content: string(content)})
			return nil
		})
		if walkErr != nil {
			return HintBundle{}, walkErr
		}
	}

	sort.SliceStable(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})

	parts := make([]string, 0, len(in.Prompts)+len(in.PromptFiles))
	for _, p := range in.Prompts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts = append(parts, p)
	}
	for _, pf := range in.PromptFiles {
		pf = strings.TrimSpace(pf)
		if pf == "" {
			continue
		}
		raw, readErr := os.ReadFile(pf)
		if readErr != nil {
			return HintBundle{}, apperr.NewValidationError(fmt.Errorf("prompt file %q: %w", pf, readErr))
		}
		text := strings.TrimSpace(string(raw))
		if text == "" {
			continue
		}
		parts = append(parts, text)
	}

	return HintBundle{
		Refs:    docs,
		Prompts: strings.Join(parts, "\n\n"),
	}, nil
}
