package refresolver

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type RefDocument struct {
	Path    string
	Content string
}

func ResolveRefs(patterns []string, root string) ([]RefDocument, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		r, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, r)
	}

	seen := map[string]struct{}{}
	docs := make([]RefDocument, 0)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slashRel := filepath.ToSlash(rel)
		for _, re := range compiled {
			if !re.MatchString(slashRel) {
				continue
			}
			abs := filepath.Clean(path)
			if _, ok := seen[abs]; ok {
				break
			}
			content, readErr := os.ReadFile(abs)
			if readErr != nil {
				return readErr
			}
			seen[abs] = struct{}{}
			docs = append(docs, RefDocument{
				Path:    abs,
				Content: string(content),
			})
			break
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(docs, func(i, j int) bool {
		return docs[i].Path < docs[j].Path
	})
	return docs, nil
}
