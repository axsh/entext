//go:build windows

package exceltopdf

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/axsh/entext/internal/common/pathutil"
	"github.com/axsh/entext/internal/common/sheetmap"
	"github.com/axsh/entext/internal/pdfnative"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

type GoNativeBackend struct{}

func NewGoNativeBackend() *GoNativeBackend {
	return &GoNativeBackend{}
}

func (b *GoNativeBackend) Convert(ctx context.Context, input string, outputDir string, sheets []int) (string, sheetmap.SheetMap, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	absInput, err := filepath.Abs(input)
	if err != nil {
		return "", sheetmap.SheetMap{}, fmt.Errorf("go-native conversion failed: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", sheetmap.SheetMap{}, err
	}

	if err := ole.CoInitialize(0); err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Excel.Application")
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	defer unknown.Release()

	excel, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	defer excel.Release()
	defer oleutil.CallMethod(excel, "Quit")

	if _, err := oleutil.PutProperty(excel, "Visible", false); err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	if _, err := oleutil.PutProperty(excel, "DisplayAlerts", false); err != nil {
		return "", sheetmap.SheetMap{}, err
	}

	workbooksVar, err := oleutil.GetProperty(excel, "Workbooks")
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	workbooks := workbooksVar.ToIDispatch()
	defer workbooks.Release()

	wbVar, err := oleutil.CallMethod(workbooks, "Open", absInput)
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	wb := wbVar.ToIDispatch()
	defer wb.Release()
	defer oleutil.CallMethod(wb, "Close", false)

	worksheetsVar, err := oleutil.GetProperty(wb, "Worksheets")
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	worksheets := worksheetsVar.ToIDispatch()
	defer worksheets.Release()

	countVar, err := oleutil.GetProperty(worksheets, "Count")
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	totalSheets := int(countVar.Val)
	countVar.Clear()

	targetIndices, err := resolveTargetSheetIndices(totalSheets, sheets)
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}

	tempDir, err := os.MkdirTemp("", "entext-gonative-*")
	if err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	defer os.RemoveAll(tempDir)

	entries := make([]sheetmap.SheetEntry, 0, len(targetIndices))
	pageNames := make([]string, 0)
	tempPDFs := make([]string, 0, len(targetIndices))

	for _, idx := range targetIndices {
		select {
		case <-ctx.Done():
			return "", sheetmap.SheetMap{}, ctx.Err()
		default:
		}

		entry, tempPDF, err := exportSheetPDF(worksheets, idx, tempDir)
		if err != nil {
			return "", sheetmap.SheetMap{}, err
		}
		entries = append(entries, entry)
		if entry.ExportStatus != "success" {
			continue
		}
		tempPDFs = append(tempPDFs, tempPDF)
		pageNames = expandPageSheetNames(pageNames, entry.SheetName, entry.PageCount)
	}

	if len(tempPDFs) == 0 {
		return "", sheetmap.SheetMap{
			SheetEntries:   entries,
			PageSheetNames: pageNames,
		}, fmt.Errorf("go-native conversion failed: no sheet exported successfully (%s)", summarizeSheetErrors(entries))
	}

	outputPDF := filepath.Join(outputDir, pathutil.BaseNameWithoutExt(input)+".pdf")
	if err := pdfnative.MergePDFs(tempPDFs, outputPDF); err != nil {
		return "", sheetmap.SheetMap{}, err
	}
	return outputPDF, sheetmap.SheetMap{
		SheetEntries:   entries,
		PageSheetNames: pageNames,
	}, nil
}

func resolveTargetSheetIndices(total int, requested []int) ([]int, error) {
	if total <= 0 {
		return nil, fmt.Errorf("workbook has no sheets")
	}
	if len(requested) == 0 {
		out := make([]int, 0, total)
		for i := 1; i <= total; i++ {
			out = append(out, i)
		}
		return out, nil
	}
	out := make([]int, 0, len(requested))
	for _, idx := range requested {
		if idx <= 0 || idx > total {
			return nil, fmt.Errorf("sheet index out of range: %d", idx)
		}
		out = append(out, idx)
	}
	return out, nil
}

func expandPageSheetNames(current []string, sheetName string, pageCount int) []string {
	if pageCount <= 0 {
		return current
	}
	for i := 0; i < pageCount; i++ {
		current = append(current, sheetName)
	}
	return current
}

func exportSheetPDF(worksheets *ole.IDispatch, idx int, tempDir string) (sheetmap.SheetEntry, string, error) {
	entry := sheetmap.SheetEntry{
		SheetIndex:   idx,
		ExportStatus: "failed",
		PageCount:    0,
	}
	sheetVar, err := oleutil.GetProperty(worksheets, "Item", idx)
	if err != nil {
		entry.Error = err.Error()
		return entry, "", nil
	}
	sheet := sheetVar.ToIDispatch()
	defer sheet.Release()

	nameVar, err := oleutil.GetProperty(sheet, "Name")
	if err != nil {
		entry.Error = err.Error()
		return entry, "", nil
	}
	entry.SheetName = nameVar.ToString()
	nameVar.Clear()

	pageSetupVar, err := oleutil.GetProperty(sheet, "PageSetup")
	if err == nil {
		pageSetup := pageSetupVar.ToIDispatch()
		_, _ = oleutil.PutProperty(pageSetup, "Zoom", false)
		_, _ = oleutil.PutProperty(pageSetup, "FitToPagesWide", 1)
		_, _ = oleutil.PutProperty(pageSetup, "FitToPagesTall", 1)
		pageSetup.Release()
	}

	tempPDF := filepath.Join(tempDir, fmt.Sprintf("sheet_%03d.pdf", idx))
	if _, err := oleutil.CallMethod(sheet, "ExportAsFixedFormat", 0, tempPDF, 0); err != nil {
		entry.Error = err.Error()
		return entry, "", nil
	}

	pageCount, err := pdfnative.CountPages(tempPDF)
	if err != nil {
		entry.Error = err.Error()
		return entry, "", nil
	}
	entry.ExportStatus = "success"
	entry.PageCount = pageCount
	return entry, tempPDF, nil
}

func summarizeSheetErrors(entries []sheetmap.SheetEntry) string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.ExportStatus == "success" {
			continue
		}
		if e.Error == "" {
			out = append(out, fmt.Sprintf("sheet[%d]: unknown error", e.SheetIndex))
			continue
		}
		out = append(out, fmt.Sprintf("sheet[%d]: %s", e.SheetIndex, e.Error))
	}
	if len(out) == 0 {
		return "unknown reason"
	}
	return strings.Join(out, "; ")
}
