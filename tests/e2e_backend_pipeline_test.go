package tests

import (
	"bytes"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestE2EExcelToPDFInvalidBackendExitCode2(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "invalid-backend")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := toolCommand(
		t,
		"excel-to-pdf",
		"--backend", "invalid",
		"-i", filepath.Join(repoRootAbs(t), "samples", "R06_09.xlsx"),
		"-o", tmpDir,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure for invalid backend")
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 && !strings.Contains(stderr.String(), "exit status 2") {
		t.Fatalf("expected exit code 2, got %d, stderr=%s", exitErr.ExitCode(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "excel backend must be auto, libreoffice, or excel-com") {
		t.Fatalf("expected backend validation message, stderr=%s", stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected empty stdout on failure, got: %q", stdout.String())
	}
}

func TestE2EExcelToPDFAutoFailureShowsAttemptList(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "excel-auto-fail")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	missingInput := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "missing.xlsx")

	cmd := toolCommand(
		t,
		"excel-to-pdf",
		"--backend", "auto",
		"-i", missingInput,
		"-o", tmpDir,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected failure for missing input")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d, stderr=%s", exitErr.ExitCode(), stderr.String())
	}
	errText := stderr.String()
	if !strings.Contains(errText, "all backends failed") {
		t.Fatalf("expected aggregate failure message, stderr=%s", errText)
	}
	if !strings.Contains(errText, "excel-com(") {
		t.Fatalf("expected excel-com attempt in auto mode, stderr=%s", errText)
	}
	if !strings.Contains(errText, "libreoffice(") {
		t.Fatalf("expected libreoffice attempt in auto mode, stderr=%s", errText)
	}
}

func TestE2EPDFToImageAutoFailureShowsAttemptList(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "pdf-auto-fail")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	missingInput := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "missing.pdf")

	cmd := toolCommand(
		t,
		"pdf-to-image",
		"--backend", "auto",
		"-i", missingInput,
		"-o", tmpDir,
		"--format", "png",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected failure for missing input")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d, stderr=%s", exitErr.ExitCode(), stderr.String())
	}
	errText := stderr.String()
	if !strings.Contains(errText, "all backends failed") {
		t.Fatalf("expected aggregate failure message, stderr=%s", errText)
	}
	if !strings.Contains(errText, "pdftoppm(") {
		t.Fatalf("expected pdftoppm attempt in auto mode, stderr=%s", errText)
	}
	if !strings.Contains(errText, "magick(") {
		t.Fatalf("expected magick attempt in auto mode, stderr=%s", errText)
	}
}

func TestE2EExcelToPDFInvalidSheetsExitCode2(t *testing.T) {
	t.Parallel()
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "invalid-sheets")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cmd := toolCommand(
		t,
		"excel-to-pdf",
		"--engine", "go-native",
		"--sheets", "a,b",
		"-i", filepath.Join(repoRootAbs(t), "samples", "R06_09.xlsx"),
		"-o", tmpDir,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected command failure for invalid --sheets")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.ExitCode() != 2 && !strings.Contains(stderr.String(), "exit status 2") {
		t.Fatalf("expected exit code 2, got %d, stderr=%s", exitErr.ExitCode(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "invalid --sheets value") {
		t.Fatalf("expected invalid sheets error, stderr=%s", stderr.String())
	}
}

func TestE2EExcelToPDFGoNativeWritesRealSheetMap(t *testing.T) {
	tmpDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "gonative-sheetmap")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	pdfPath, sidecar := runExcelToPDFGoNative(t, tmpDir, "--sheets", "1,2")
	if _, err := os.Stat(pdfPath); err != nil {
		t.Fatalf("expected output pdf, got error: %v", err)
	}
	sm := readSheetMap(t, sidecar)
	if sm.Version < 1 {
		t.Fatalf("invalid sidecar version: %d", sm.Version)
	}
	if len(sm.SheetEntries) != 2 {
		t.Fatalf("expected 2 sheet entries, got %d", len(sm.SheetEntries))
	}
	if sm.SheetEntries[0].SheetIndex != 1 || sm.SheetEntries[1].SheetIndex != 2 {
		t.Fatalf("unexpected sheet indices: %#v", sm.SheetEntries)
	}
	successPages := 0
	for _, e := range sm.SheetEntries {
		if e.ExportStatus == "success" {
			if e.SheetName == "" {
				t.Fatalf("sheet name must not be empty: %#v", e)
			}
			if e.PageCount <= 0 {
				t.Fatalf("success page count must be positive: %#v", e)
			}
			successPages += e.PageCount
		}
	}
	if successPages == 0 {
		t.Fatalf("expected at least one successful sheet export: %#v", sm.SheetEntries)
	}
	if len(sm.PageSheetNames) != successPages {
		t.Fatalf("page_sheet_names length mismatch: got %d want %d", len(sm.PageSheetNames), successPages)
	}
}

func TestE2EPDFToImageGoNativeDPIAffectsSize(t *testing.T) {
	pdfDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "gonative-dpi-pdf")
	img200Dir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "gonative-dpi-200")
	img300Dir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "gonative-dpi-300")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.MkdirAll(img200Dir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.MkdirAll(img300Dir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	pdfPath, sidecar := runExcelToPDFGoNative(t, pdfDir, "--sheets", "1")
	out200 := runPDFToImageGoNative(t, pdfPath, img200Dir, 200, sidecar)
	out300 := runPDFToImageGoNative(t, pdfPath, img300Dir, 300, sidecar)

	w200 := imageWidth(t, out200[0])
	w300 := imageWidth(t, out300[0])
	if w300 <= w200 {
		t.Fatalf("expected 300dpi width > 200dpi width: %d <= %d", w300, w200)
	}
}

func TestE2EPDFToImageGoNativeUsesSheetNames(t *testing.T) {
	pdfDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "gonative-naming-pdf")
	imgDir := filepath.Join(repoRootAbs(t), "tmp", "tests-e2e", "gonative-naming-images")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	pdfPath, sidecar := runExcelToPDFGoNative(t, pdfDir, "--sheets", "1")
	paths := runPDFToImageGoNative(t, pdfPath, imgDir, 200, sidecar)
	sm := readSheetMap(t, sidecar)
	if len(sm.PageSheetNames) == 0 {
		t.Fatalf("expected page_sheet_names in sidecar")
	}
	wantName := sanitizeForFilename(sm.PageSheetNames[0])
	if wantName == "" {
		wantName = "page"
	}
	expected := "01_" + wantName + ".png"
	if filepath.Base(paths[0]) != expected {
		t.Fatalf("unexpected first image name: got %s want %s", filepath.Base(paths[0]), expected)
	}
}

func runExcelToPDFGoNative(t *testing.T, outputDir string, extraArgs ...string) (string, string) {
	t.Helper()
	args := []string{
		"--engine", "go-native",
		"-i", filepath.Join(repoRootAbs(t), "samples", "R06_09.xlsx"),
		"-o", outputDir,
	}
	args = append(args, extraArgs...)
	cmd := toolCommand(t, "excel-to-pdf", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("excel-to-pdf go-native failed: %v stderr=%s", err, stderr.String())
	}
	pdfPath := firstPath(stdout.String())
	if pdfPath == "" {
		t.Fatalf("excel-to-pdf stdout did not contain output path: %q", stdout.String())
	}
	pdfPath = resolveToolOutputPath(pdfPath)
	sidecar := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + ".sheet-map.json"
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("expected sidecar file %s: %v", sidecar, err)
	}
	return pdfPath, sidecar
}

func runPDFToImageGoNative(t *testing.T, inputPDF string, outputDir string, dpi int, sidecar string) []string {
	t.Helper()
	cmd := toolCommand(
		t,
		"pdf-to-image",
		"--engine", "go-native",
		"--dpi", strconv.Itoa(dpi),
		"--sheet-map", sidecar,
		"--format", "png",
		"-i", inputPDF,
		"-o", outputDir,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("pdf-to-image go-native failed: %v stderr=%s", err, stderr.String())
	}
	paths := splitPaths(stdout.String())
	if len(paths) == 0 {
		t.Fatalf("pdf-to-image stdout did not contain output paths: %q", stdout.String())
	}
	for i := range paths {
		paths[i] = resolveToolOutputPath(paths[i])
	}
	return paths
}

type sheetMapPayload struct {
	Version        int      `json:"version"`
	PageSheetNames []string `json:"page_sheet_names"`
	SheetEntries   []struct {
		SheetIndex   int    `json:"sheet_index"`
		SheetName    string `json:"sheet_name"`
		ExportStatus string `json:"export_status"`
		PageCount    int    `json:"page_count"`
	} `json:"sheet_entries"`
}

func readSheetMap(t *testing.T, path string) sheetMapPayload {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read sidecar: %v", err)
	}
	var payload sheetMapPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("failed to parse sidecar: %v", err)
	}
	return payload
}

func firstPath(stdout string) string {
	paths := splitPaths(stdout)
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

func splitPaths(stdout string) []string {
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return out
}

func resolveToolOutputPath(path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	resolved := filepath.Join("..", path)
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return filepath.Clean(resolved)
	}
	return abs
}

func imageWidth(t *testing.T, path string) int {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open image: %v", err)
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		t.Fatalf("failed to decode image config: %v", err)
	}
	return cfg.Width
}

var (
	forbiddenChars  = regexp.MustCompile(`[\\/:*?"<>|]`)
	spaceChars      = regexp.MustCompile(`\s+`)
	multiUnderscore = regexp.MustCompile(`_+`)
)

func sanitizeForFilename(name string) string {
	name = spaceChars.ReplaceAllString(name, "_")
	name = forbiddenChars.ReplaceAllString(name, "")
	name = multiUnderscore.ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	return name
}

func toolCommand(t *testing.T, name string, args ...string) *exec.Cmd {
	t.Helper()
	var pkg string
	switch name {
	case "excel-to-pdf":
		pkg = "./cmd/excel-to-pdf"
	case "pdf-to-image":
		pkg = "./cmd/pdf-to-image"
	default:
		t.Fatalf("unsupported tool name: %s", name)
	}
	goArgs := []string{"run", pkg}
	goArgs = append(goArgs, args...)
	cmd := exec.Command("go", goArgs...)
	cmd.Dir = filepath.Join("..")
	return cmd
}

func repoRootAbs(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	return root
}
