package excelfill

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/axsh/entext/internal/excelfill/dialog"
	"github.com/axsh/entext/internal/imagetomd/tern"
)

type CellWrite struct {
	Sheet string
	Cell  string
	Value string
}

type FillContext struct {
	StructureMD    string
	HintText       string
	Answers        map[string]string
	History        []dialog.Message
	VisualFeedback []dialog.VisualIssue
}

type Filler interface {
	Plan(ctx context.Context, fc FillContext) (questions []dialog.FieldSpec, writes []CellWrite, err error)
}

type StaticFiller struct {
	Questions []dialog.FieldSpec
	Writes    []CellWrite
	Calls     int
}

func (s *StaticFiller) Plan(ctx context.Context, fc FillContext) ([]dialog.FieldSpec, []CellWrite, error) {
	s.Calls++
	if s.Calls == 1 && len(s.Questions) > 0 && len(fc.Answers) == 0 {
		return s.Questions, nil, nil
	}
	writes := s.Writes
	if len(fc.VisualFeedback) > 0 {
		// On visual retry, shorten values slightly for tests.
		out := make([]CellWrite, len(writes))
		copy(out, writes)
		for i := range out {
			if len(out[i].Value) > 3 {
				out[i].Value = out[i].Value[:len(out[i].Value)/2]
			}
		}
		return nil, out, nil
	}
	return nil, writes, nil
}

type TernFiller struct {
	Client tern.Client
	Agent  string
	Model  string
}

type planResponse struct {
	Need   []dialog.FieldSpec `json:"need"`
	Writes []CellWrite        `json:"writes"`
}

func (f *TernFiller) Plan(ctx context.Context, fc FillContext) ([]dialog.FieldSpec, []CellWrite, error) {
	if f == nil || f.Client == nil {
		return nil, nil, fmt.Errorf("nil tern filler")
	}
	sessionID, err := f.Client.CreateSession(ctx, tern.CreateSessionRequest{Agent: f.Agent, Model: f.Model, WorkDir: "."})
	if err != nil {
		return nil, nil, err
	}
	var b strings.Builder
	b.WriteString("Fill an Excel template. Reply with JSON only: {\"need\":[{\"id\",\"label\",\"required\"}],\"writes\":[{\"sheet\",\"cell\",\"value\"}]}.\n")
	b.WriteString("If information is missing, put fields in need and leave writes empty.\n")
	b.WriteString("Structure markdown:\n")
	b.WriteString(fc.StructureMD)
	b.WriteString("\n\n")
	if fc.HintText != "" {
		b.WriteString(fc.HintText)
		b.WriteString("\n\n")
	}
	if len(fc.Answers) > 0 {
		raw, _ := json.Marshal(fc.Answers)
		b.WriteString("Known answers: ")
		b.Write(raw)
		b.WriteString("\n")
	}
	if len(fc.VisualFeedback) > 0 {
		raw, _ := json.Marshal(fc.VisualFeedback)
		b.WriteString("Visual issues to fix: ")
		b.Write(raw)
		b.WriteString("\n")
	}
	b.WriteString("\nRespond with JSON only. Do not ask questions in prose.")
	text, err := f.Client.SendText(ctx, sessionID, b.String())
	if err != nil {
		return nil, nil, err
	}
	text = extractJSONObject(text)
	var resp planResponse
	if err := json.Unmarshal([]byte(text), &resp); err != nil {
		return nil, nil, fmt.Errorf("parse filler json: %w", err)
	}
	return resp.Need, resp.Writes, nil
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			return s[i : j+1]
		}
	}
	return s
}
