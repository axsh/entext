package pdfnative

import (
	"bytes"
	"fmt"
	"image/jpeg"
	"image/png"
	"strings"

	fitz "github.com/gen2brain/go-fitz"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

type RenderedPage struct {
	PageIndex int
	Width     int
	Height    int
	Bytes     []byte
	Format    string
}

func CountPages(pdfPath string) (int, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return 0, err
	}
	defer doc.Close()
	return doc.NumPage(), nil
}

func MergePDFs(inputs []string, output string) error {
	if len(inputs) == 0 {
		return fmt.Errorf("no pdf inputs to merge")
	}
	return api.MergeCreateFile(inputs, output, false, nil)
}

func RenderPages(pdfPath string, dpi int, format string) ([]RenderedPage, error) {
	format = normalizeFormat(format)
	if format != "png" && format != "jpg" {
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	if dpi <= 0 {
		dpi = 200
	}

	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, err
	}
	defer doc.Close()

	out := make([]RenderedPage, 0, doc.NumPage())
	for i := 0; i < doc.NumPage(); i++ {
		img, err := doc.ImageDPI(i, float64(dpi))
		if err != nil {
			return nil, err
		}
		var buf bytes.Buffer
		switch format {
		case "png":
			if err := png.Encode(&buf, img); err != nil {
				return nil, err
			}
		case "jpg":
			if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
				return nil, err
			}
		}
		out = append(out, RenderedPage{
			PageIndex: i,
			Width:     img.Bounds().Dx(),
			Height:    img.Bounds().Dy(),
			Bytes:     buf.Bytes(),
			Format:    format,
		})
	}
	return out, nil
}

func normalizeFormat(format string) string {
	switch strings.ToLower(format) {
	case "jpeg":
		return "jpg"
	default:
		return strings.ToLower(format)
	}
}
