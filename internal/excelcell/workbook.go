package excelcell

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/xuri/excelize/v2"
)

type Cell struct {
	Sheet string
	Ref   string
	Value string
}

type SheetSnapshot struct {
	Name        string
	Index       int
	Cells       []Cell
	MergeRanges []string
}

type Workbook struct {
	file *excelize.File
	path string
}

func Open(path string) (*Workbook, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	return &Workbook{file: f, path: path}, nil
}

func (w *Workbook) Close() error {
	if w == nil || w.file == nil {
		return nil
	}
	return w.file.Close()
}

func (w *Workbook) SheetName(index0 int) string {
	return w.file.GetSheetName(index0)
}

func (w *Workbook) Snapshots() ([]SheetSnapshot, error) {
	names := w.file.GetSheetList()
	out := make([]SheetSnapshot, 0, len(names))
	for i, name := range names {
		rows, err := w.file.GetRows(name)
		if err != nil {
			return nil, fmt.Errorf("sheet %q rows: %w", name, err)
		}
		cells := make([]Cell, 0)
		for r, row := range rows {
			for c, val := range row {
				if val == "" {
					continue
				}
				ref, err := excelize.CoordinatesToCellName(c+1, r+1)
				if err != nil {
					return nil, err
				}
				cells = append(cells, Cell{Sheet: name, Ref: ref, Value: val})
			}
		}
		merges := make([]string, 0)
		mergeCells, err := w.file.GetMergeCells(name)
		if err != nil {
			return nil, fmt.Errorf("sheet %q merges: %w", name, err)
		}
		for _, m := range mergeCells {
			merges = append(merges, m.GetStartAxis()+":"+m.GetEndAxis())
		}
		out = append(out, SheetSnapshot{
			Name:        name,
			Index:       i + 1,
			Cells:       cells,
			MergeRanges: merges,
		})
	}
	return out, nil
}

func (w *Workbook) SetCellValue(sheet, cellRef, value string) error {
	return w.file.SetCellValue(sheet, cellRef, value)
}

func (w *Workbook) SaveAs(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return w.file.SaveAs(path)
}

func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	_, err = io.Copy(out, in)
	return err
}
