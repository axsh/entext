package imagemd

import (
	"context"

	"github.com/axsh/entext"
)

type Converter struct {
	config entext.ImageToMarkdownConfig
}

func New(cfg entext.ImageToMarkdownConfig) *Converter {
	return &Converter{config: cfg}
}

func (c *Converter) Convert(ctx context.Context, inputPath string, outputPath string, outputDir string, refs []string) (entext.MarkdownArtifact, error) {
	return entext.ConvertImageToMarkdown(ctx, entext.ImageToMarkdownJob{
		InputPath:   inputPath,
		OutputPath:  outputPath,
		OutputDir:   outputDir,
		RefPatterns: refs,
	}, c.config)
}
