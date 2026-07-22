package excelanalyze

import (
	"context"
	"fmt"
	"strings"

	"github.com/axsh/entext/internal/imagetomd/tern"
)

type SheetImage struct {
	SheetName string
	ImagePath string
}

type SemanticAnalyzer interface {
	AnalyzeSheets(ctx context.Context, images []SheetImage, hintText string) (map[string]string, error)
}

const TemplateStructurePrompt = `You are analyzing an Excel template page image.
Identify fixed labels vs fill-in fields, sections, and reading order.
Return concise Markdown describing the semantic structure only.
Do not invent cell addresses.`

const nonInteractiveSuffix = `

Respond with the final Markdown only. Do not ask questions. Do not wait for confirmation.`

type TernSemanticAnalyzer struct {
	Client tern.Client
	Agent  string
	Model  string
}

func (a *TernSemanticAnalyzer) AnalyzeSheets(ctx context.Context, images []SheetImage, hintText string) (map[string]string, error) {
	if a == nil || a.Client == nil {
		return nil, fmt.Errorf("tern semantic analyzer: nil client")
	}
	out := make(map[string]string, len(images))
	for _, img := range images {
		sessionID, err := a.Client.CreateSession(ctx, tern.CreateSessionRequest{
			Agent:   a.Agent,
			Model:   a.Model,
			WorkDir: ".",
		})
		if err != nil {
			return nil, fmt.Errorf("create session for sheet %q: %w", img.SheetName, err)
		}
		var b strings.Builder
		b.WriteString(TemplateStructurePrompt)
		b.WriteString("\n\nSheet name: ")
		b.WriteString(img.SheetName)
		if strings.TrimSpace(hintText) != "" {
			b.WriteString("\n\n")
			b.WriteString(hintText)
		}
		b.WriteString(nonInteractiveSuffix)
		text, err := a.Client.SendImagePrompt(ctx, sessionID, b.String(), img.ImagePath)
		if err != nil {
			return nil, fmt.Errorf("semantic analyze sheet %q: %w", img.SheetName, err)
		}
		out[img.SheetName] = strings.TrimSpace(text)
	}
	return out, nil
}

// StaticSemantic returns fixed semantic markdown per sheet (tests / offline).
type StaticSemantic struct {
	Out map[string]string
	Err error
}

func (s *StaticSemantic) AnalyzeSheets(ctx context.Context, images []SheetImage, hintText string) (map[string]string, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	if s.Out != nil {
		return s.Out, nil
	}
	out := make(map[string]string, len(images))
	for _, img := range images {
		out[img.SheetName] = "static semantic"
	}
	return out, nil
}
