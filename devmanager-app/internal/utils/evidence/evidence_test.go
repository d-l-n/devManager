package evidence

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeFileAt(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func makeProject(t *testing.T) (root string, base time.Time) {
	t.Helper()
	root = t.TempDir()
	base = time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	writeFileAt(t, filepath.Join(root, "test-results", "a", "old.png"), base)
	writeFileAt(t, filepath.Join(root, "test-results", "b", "ignored.mp4"), base.Add(30*time.Second))
	writeFileAt(t, filepath.Join(root, "test-results", "d", "y.webm"), base.Add(40*time.Second))
	writeFileAt(t, filepath.Join(root, "test-results", "e", "f.zip"), base.Add(50*time.Second))
	writeFileAt(t, filepath.Join(root, "node_modules", "z.png"), base.Add(60*time.Second))
	return root, base
}

func TestScanClassifiesAndOrdersDesc(t *testing.T) {
	root, base := makeProject(t)

	got := Scan(root, 200)
	if len(got) != 3 {
		t.Fatalf("3 artefactos esperados, got %d: %+v", len(got), got)
	}

	want := []struct {
		relPath, kind string
		mtime         int64
	}{
		{"test-results/e/f.zip", "trace", base.Add(50 * time.Second).Unix()},
		{"test-results/d/y.webm", "video", base.Add(40 * time.Second).Unix()},
		{"test-results/a/old.png", "image", base.Unix()},
	}
	for i, w := range want {
		f := got[i]
		if f.RelPath != w.relPath {
			t.Errorf("[%d] RelPath = %q, want %q", i, f.RelPath, w.relPath)
		}
		if f.Kind != w.kind {
			t.Errorf("[%d] Kind = %q, want %q", i, f.Kind, w.kind)
		}
		if f.MTime != w.mtime {
			t.Errorf("[%d] MTime = %d, want %d", i, f.MTime, w.mtime)
		}
		wantPath := filepath.Join(root, filepath.FromSlash(w.relPath))
		if f.Path != wantPath {
			t.Errorf("[%d] Path = %q, want %q", i, f.Path, wantPath)
		}
		wantDir := filepath.Dir(wantPath)
		if f.TestDir != wantDir {
			t.Errorf("[%d] TestDir = %q, want %q", i, f.TestDir, wantDir)
		}
	}
}

func TestScanSkipsDirectories(t *testing.T) {
	root, base := makeProject(t)
	skip := []string{"test-results/.git/x.png", "test-results/venv/v.png", "test-results/.venv/w.png", "test-results/__pycache__/p.png"}
	for _, rel := range skip {
		writeFileAt(t, filepath.Join(root, filepath.FromSlash(rel)), base.Add(time.Second))
	}

	got := Scan(root, 200)
	for _, f := range got {
		for _, s := range []string{"\\.git\\", "venv\\", "__pycache__"} {
			if strings.Contains(f.Path, s) {
				t.Errorf("directorio a saltar no debe aparecer: %s en %s", s, f.Path)
			}
		}
	}
	if n := len(got); n != 3 {
		t.Errorf("solo los 3 originales deben quedar, got %d", n)
	}
}

func TestScanMaxItemsTrims(t *testing.T) {
	root, _ := makeProject(t)

	got := Scan(root, 2)
	if len(got) != 2 {
		t.Fatalf("recorte a 2 esperado, got %d", len(got))
	}
	if got[0].RelPath != "test-results/e/f.zip" || got[1].RelPath != "test-results/d/y.webm" {
		t.Errorf("deben quedar los 2 m├ís recientes, got %+v", got)
	}
}

func TestScanMissingDirReturnsEmpty(t *testing.T) {
	if got := Scan(filepath.Join(t.TempDir(), "sin-proyecto"), 200); got != nil {
		t.Errorf("dir ausente debe dar nil, got %+v", got)
	}
	empty := t.TempDir()
	if got := Scan(empty, 200); len(got) != 0 {
		t.Errorf("proyecto sin test-results debe dar vac├¡o, got %+v", got)
	}
}

func TestScanRelPathUsesForwardSlashes(t *testing.T) {
	root, _ := makeProject(t)

	for _, f := range Scan(root, 200) {
		if strings.ContainsRune(f.RelPath, '\\') {
			t.Errorf("RelPath con separador nativo %q; debe usar /", f.RelPath)
		}
	}
}

func TestScanExtCaseInsensitive(t *testing.T) {
	root, base := makeProject(t)
	writeFileAt(t, filepath.Join(root, "test-results", "g", "UPPER.JPG"), base.Add(70*time.Second))

	got := Scan(root, 200)
	if len(got) != 4 {
		t.Fatalf(".PNG/.JPG may├║scula debe clasificar, got %d: %+v", len(got), got)
	}
	first := got[0]
	if first.Kind != "image" || first.RelPath != "test-results/g/UPPER.JPG" {
		t.Errorf("extensi├│n may├║scula mal clasificada: %+v", first)
	}
}

func TestScanJPEGKind(t *testing.T) {
	root, base := makeProject(t)
	writeFileAt(t, filepath.Join(root, "test-results", "j", "shot.jpeg"), base.Add(80*time.Second))

	got := Scan(root, 200)
	var found bool
	for _, f := range got {
		if f.RelPath == "test-results/j/shot.jpeg" {
			found = true
			if f.Kind != "image" {
				t.Errorf(".jpeg debe ser image, got %q", f.Kind)
			}
		}
	}
	if !found {
		t.Error(".jpeg no encontrado")
	}
}

func TestFindHTMLReport(t *testing.T) {
	root, _ := makeProject(t)

	if got := FindHTMLReport(root); got != "" {
		t.Errorf("sin report debe dar cadena vac├¡a, got %q", got)
	}

	reportPath := filepath.Join(root, "playwright-report", "index.html")
	writeFileAt(t, reportPath, time.Now())

	if got := FindHTMLReport(root); got != reportPath {
		t.Errorf("report = %q, want %q", got, reportPath)
	}
}

func TestFindHTMLReportIndexIsDir(t *testing.T) {
	root := t.TempDir()
	dirIndex := filepath.Join(root, "playwright-report", "index.html")
	if err := os.MkdirAll(dirIndex, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := FindHTMLReport(root); got != "" {
		t.Errorf("index.html como directorio debe descartarse, got %q", got)
	}
}
