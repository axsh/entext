package excelfill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/axsh/entext/internal/excelfill/dialog"
	"github.com/axsh/entext/internal/imagetomd/tern"
)

type VisualChecker interface {
	Check(ctx context.Context, imagePaths []string, hintText string) ([]dialog.VisualIssue, error)
}

type StaticVisual struct {
	Sequence [][]dialog.VisualIssue
	Calls    int
}

func (v *StaticVisual) Check(ctx context.Context, imagePaths []string, hintText string) ([]dialog.VisualIssue, error) {
	if v.Calls >= len(v.Sequence) {
		return nil, nil
	}
	issues := v.Sequence[v.Calls]
	v.Calls++
	return issues, nil
}

const VisualCheckPrompt = `Inspect the filled Excel page image(s).
Report ONLY visible text cutoff, overflow into neighbors, or broken layout.
Reply with JSON array of objects: kind, sheet, cell_hint, description, suggestion.
If none, reply [].`

type TernVisualChecker struct {
	Client tern.Client
	Agent  string
	Model  string
}

func (v *TernVisualChecker) Check(ctx context.Context, imagePaths []string, hintText string) ([]dialog.VisualIssue, error) {
	if v == nil || v.Client == nil {
		return nil, fmt.Errorf("nil visual checker")
	}
	var all []dialog.VisualIssue
	for _, img := range imagePaths {
		sessionID, err := v.Client.CreateSession(ctx, tern.CreateSessionRequest{Agent: v.Agent, Model: v.Model, WorkDir: "."})
		if err != nil {
			return nil, err
		}
		prompt := VisualCheckPrompt
		if strings.TrimSpace(hintText) != "" {
			prompt += "\n\n" + hintText
		}
		prompt += "\nRespond with JSON array only."
		text, err := v.Client.SendImagePrompt(ctx, sessionID, prompt, img)
		if err != nil {
			return nil, err
		}
		arr := extractJSONArray(text)
		var issues []dialog.VisualIssue
		if err := json.Unmarshal([]byte(arr), &issues); err != nil {
			return nil, fmt.Errorf("parse visual issues: %w", err)
		}
		all = append(all, issues...)
	}
	return all, nil
}

func extractJSONArray(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "["); i >= 0 {
		if j := strings.LastIndex(s, "]"); j > i {
			return s[i : j+1]
		}
	}
	return s
}
