package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestExternalImportRootModule(t *testing.T) {
	t.Parallel()
	root := repoRootAbs(t)
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "main.go"), `package main

import (
	"context"
	"github.com/axsh/entext"
)

func main() {
	_, _ = entext.ConvertExcelToPDFWithOptions(context.Background(), entext.FileJob{
		InputPath: "dummy.xlsx",
		OutputDir: ".",
	}, entext.ExcelPDFOptions{
		Backend: "auto",
		Engine:  "legacy",
	})
}
`)

	run(t, workDir, "go", "mod", "init", "example.com/ext")
	run(t, workDir, "go", "mod", "edit", "-replace", "github.com/axsh/entext="+root)
	run(t, workDir, "go", "get", "github.com/axsh/entext")
	run(t, workDir, "go", "build", "./...")
}

func TestExternalImportSubPackages(t *testing.T) {
	t.Parallel()
	root := repoRootAbs(t)
	workDir := t.TempDir()
	writeFile(t, filepath.Join(workDir, "main.go"), `package main

import (
	"github.com/axsh/entext"
	"github.com/axsh/entext/excelpdf"
	"github.com/axsh/entext/imagemd"
	"github.com/axsh/entext/pdfimage"
)

func main() {
	_ = excelpdf.New()
	_ = pdfimage.New("png")
	_ = imagemd.New(entext.DefaultImageToMarkdownConfig())
}
`)

	run(t, workDir, "go", "mod", "init", "example.com/ext2")
	run(t, workDir, "go", "mod", "edit", "-replace", "github.com/axsh/entext="+root)
	run(t, workDir, "go", "get", "github.com/axsh/entext")
	run(t, workDir, "go", "build", "./...")
}

func run(t *testing.T, workDir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = workDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, string(out))
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}
