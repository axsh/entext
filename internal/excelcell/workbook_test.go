package excelcell

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xuri/excelize/v2"
)

func writeMinimalXLSX(t *testing.T, path string) {
	t.Helper()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	if err := f.SetCellValue(sheet, "A1", "Name"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "B1", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "A2", "Age"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func TestOpenListsSheetsAndCells(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	writeMinimalXLSX(t, path)

	wb, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = wb.Close() }()

	snaps, err := wb.Snapshots()
	if err != nil {
		t.Fatalf("Snapshots: %v", err)
	}
	if len(snaps) < 1 {
		t.Fatal("expected at least one sheet")
	}
	if snaps[0].Index != 1 {
		t.Fatalf("index=%d", snaps[0].Index)
	}
	foundName := false
	for _, c := range snaps[0].Cells {
		if c.Ref == "A1" && c.Value == "Name" {
			foundName = true
		}
	}
	if !foundName {
		t.Fatalf("A1 Name not found in %#v", snaps[0].Cells)
	}
}

func TestMergedCellsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "merge.xlsx")
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	if err := f.SetCellValue(sheet, "A1", "Title"); err != nil {
		t.Fatal(err)
	}
	if err := f.MergeCell(sheet, "A1", "C1"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()

	wb, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wb.Close() }()
	snaps, err := wb.Snapshots()
	if err != nil {
		t.Fatal(err)
	}
	if len(snaps[0].MergeRanges) == 0 {
		t.Fatal("expected merge ranges")
	}
}

func TestSetCellAndSaveAsDoesNotModifySource(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.xlsx")
	writeMinimalXLSX(t, src)
	before, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	wb, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	sheet := wb.SheetName(0)
	if err := wb.SetCellValue(sheet, "B1", "Alice"); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out.xlsx")
	if err := wb.SaveAs(dst); err != nil {
		t.Fatal(err)
	}
	_ = wb.Close()

	after, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("source file was modified")
	}

	out, err := Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	snaps, err := out.Snapshots()
	if err != nil {
		t.Fatal(err)
	}
	ok := false
	for _, c := range snaps[0].Cells {
		if c.Ref == "B1" && c.Value == "Alice" {
			ok = true
		}
	}
	if !ok {
		t.Fatal("B1 Alice not written")
	}
}

func TestCopyThenWritePreservesUnrelatedCells(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.xlsx")
	writeMinimalXLSX(t, src)
	dst := filepath.Join(t.TempDir(), "copy.xlsx")
	if err := CopyFile(src, dst); err != nil {
		t.Fatal(err)
	}
	wb, err := Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wb.Close() }()
	sheet := wb.SheetName(0)
	if err := wb.SetCellValue(sheet, "B1", "Bob"); err != nil {
		t.Fatal(err)
	}
	if err := wb.SaveAs(dst); err != nil {
		t.Fatal(err)
	}
	_ = wb.Close()

	wb2, err := Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wb2.Close() }()
	snaps, err := wb2.Snapshots()
	if err != nil {
		t.Fatal(err)
	}
	hasName, hasBob := false, false
	for _, c := range snaps[0].Cells {
		if c.Ref == "A1" && c.Value == "Name" {
			hasName = true
		}
		if c.Ref == "B1" && c.Value == "Bob" {
			hasBob = true
		}
	}
	if !hasName || !hasBob {
		t.Fatalf("preserve failed name=%v bob=%v cells=%#v", hasName, hasBob, snaps[0].Cells)
	}
}

func TestOpenMissingFileReturnsError(t *testing.T) {
	_, err := Open(filepath.Join(t.TempDir(), "missing.xlsx"))
	if err == nil {
		t.Fatal("expected error")
	}
}
