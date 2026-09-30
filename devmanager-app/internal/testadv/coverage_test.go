package testadv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const istanbulSample = `{
  "total": {"lines":{"total":100,"covered":85,"skipped":0,"pct":85},
            "statements":{"total":120,"covered":90,"skipped":0,"pct":75},
            "functions":{"total":20,"covered":20,"skipped":0,"pct":100},
            "branches":{"total":40,"covered":20,"skipped":0,"pct":50}},
  "/a/b.js": {"lines":{"total":10,"covered":1,"skipped":0,"pct":10},
              "statements":{"total":12,"covered":2,"skipped":0,"pct":16.67},
              "functions":{"total":2,"covered":2,"skipped":0,"pct":100},
              "branches":{"total":4,"covered":0,"skipped":0,"pct":0}},
  "/a/c.js": {"lines":{"total":90,"covered":84,"skipped":0,"pct":93.33},
              "statements":{"total":108,"covered":88,"skipped":0,"pct":81.48},
              "functions":{"total":18,"covered":18,"skipped":0,"pct":100},
              "branches":{"total":36,"covered":20,"skipped":0,"pct":55.56}},
  "branchesTrue": {"total":0,"covered":0,"skipped":0,"pct":100}
}`

func TestParseIstanbulSummary(t *testing.T) {
	got, err := ParseIstanbulSummary([]byte(istanbulSample))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Source != "istanbul" {
		t.Errorf("source = %q", got.Source)
	}
	if got.Files != 2 {
		t.Errorf("files = %d, want 2 (branchesTrue no cuenta)", got.Files)
	}
	if got.Lines.Pct != 85 || got.Lines.Covered != 85 || got.Lines.Total != 100 {
		t.Errorf("lines = %+v", got.Lines)
	}
	if got.Statements.Pct != 75 || got.Functions.Pct != 100 || got.Branches.Pct != 50 {
		t.Errorf("statements/functions/branches = %+v %+v %+v", got.Statements, got.Functions, got.Branches)
	}
}

func TestParseIstanbulSummaryInvalid(t *testing.T) {
	if _, err := ParseIstanbulSummary([]byte("{")); err == nil {
		t.Fatal("JSON roto debería fallar")
	}
	if _, err := ParseIstanbulSummary([]byte(`{"foo":{}}`)); err == nil {
		t.Fatal("sin clave total debería fallar")
	}
}

func TestCoverageThreshold(t *testing.T) {
	summary, _ := ParseIstanbulSummary([]byte(istanbulSample))
	if summary.WithThreshold(50).Pass != true {
		t.Error("umbral 50 debería pasar")
	}
	if summary.WithThreshold(80).Pass != false {
		t.Error("umbral 80 debería fallar (statements 75, branches 50)")
	}
	if summary.WithThreshold(100).Pass != false {
		t.Error("umbral 100 debería fallar")
	}
	// Métrica sin elementos no puede incumplir el umbral.
	empty := CoverageSummary{}.WithThreshold(90)
	if !empty.Pass {
		t.Error("summary vacío debería pasar cualquier umbral")
	}
}

func TestParseIstanbulFilesOrdenaPeorPrimero(t *testing.T) {
	files, err := ParseIstanbulFiles([]byte(istanbulSample))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %d, want 2", len(files))
	}
	if files[0].Path != "/a/b.js" {
		t.Errorf("primer fichero = %q, want /a/b.js (10%%)", files[0].Path)
	}
	if files[0].Lines.Pct != 10 || files[1].Lines.Pct != 93.33 {
		t.Errorf("orden inválido: %v", []float64{files[0].Lines.Pct, files[1].Lines.Pct})
	}
}

func TestParseIstanbulFinal(t *testing.T) {
	final := `{"/x/a.js":{"path":"/x/a.js",
      "statementMap":{"0":{"start":{"line":1,"column":0},"end":{"line":1,"column":10}},
                      "1":{"start":{"line":2,"column":0},"end":{"line":2,"column":10}}},
      "s":{"0":1,"1":0},
      "fnMap":{"0":{}},"f":{"0":1},
      "branchMap":{"0":{}},"b":{"0":[1,0]}}}`
	got, err := ParseIstanbulFinal([]byte(final))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Statements.Pct != 50 || got.Statements.Total != 2 {
		t.Errorf("statements = %+v", got.Statements)
	}
	if got.Lines.Pct != 50 || got.Lines.Total != 2 {
		t.Errorf("lines = %+v", got.Lines)
	}
	if got.Functions.Pct != 100 || got.Branches.Pct != 50 {
		t.Errorf("functions/branches = %+v %+v", got.Functions, got.Branches)
	}
	if got.Files != 1 {
		t.Errorf("files = %d", got.Files)
	}
}

func TestParseGoCoverProfile(t *testing.T) {
	profile := "mode: set\n" +
		"github.com/x/a.go:1.1,2.2 3 1\n" +
		"github.com/x/a.go:3.1,4.2 2 0\n" +
		"github.com/x/b.go:1.1,1.10 1 1\n"
	got, err := ParseGoCoverProfile([]byte(profile))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Files != 2 {
		t.Errorf("files = %d, want 2", got.Files)
	}
	if got.Statements.Covered != 4 || got.Statements.Total != 6 {
		t.Errorf("statements = %+v", got.Statements)
	}
	if got.Statements.Pct != 66.67 {
		t.Errorf("pct = %v, want 66.67", got.Statements.Pct)
	}
	if got.Lines != got.Statements {
		t.Error("lines debe derivarse de statements en coverprofile")
	}
}

func TestParseGoCoverProfileInvalid(t *testing.T) {
	if _, err := ParseGoCoverProfile([]byte("mode: set\nbasura\n")); err == nil {
		t.Fatal("línea inválida debería fallar")
	}
	if _, err := ParseGoCoverProfile([]byte("")); err == nil {
		t.Fatal("perfil vacío debería fallar")
	}
}

func TestParseCobertura(t *testing.T) {
	doc := `<?xml version="1.0"?>
<coverage line-rate="0.8" branch-rate="0.5" lines-covered="80" lines-valid="100"
          branches-covered="10" branches-valid="20">
  <packages><package name="p"><classes><class name="C" filename="c.js">
    <methods>
      <method name="m"><lines><line number="1" hits="1"/></lines></method>
      <method name="n"><lines><line number="2" hits="0"/></lines></method>
    </methods>
    <lines><line number="1" hits="1"/><line number="2" hits="0"/></lines>
  </class></classes></package></packages>
</coverage>`
	got, err := ParseCobertura([]byte(doc))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Source != "cobertura" || got.Files != 1 {
		t.Errorf("source/files = %q/%d", got.Source, got.Files)
	}
	if got.Lines.Covered != 80 || got.Lines.Total != 100 {
		t.Errorf("lines debe usar los atributos explícitos: %+v", got.Lines)
	}
	if got.Functions.Covered != 1 || got.Functions.Total != 2 {
		t.Errorf("functions = %+v", got.Functions)
	}
	if got.Branches.Covered != 10 || got.Branches.Total != 20 {
		t.Errorf("branches = %+v", got.Branches)
	}
}

func TestParseCoberturaSinConteosUsaRate(t *testing.T) {
	doc := `<coverage line-rate="0.75" branch-rate="0.4"></coverage>`
	got, err := ParseCobertura([]byte(doc))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Branches.Pct != 40 || got.Branches.Total != 0 {
		t.Errorf("branches desde rate = %+v", got.Branches)
	}
}

func TestLoadCoverageDetectaYParsea(t *testing.T) {
	dir := t.TempDir()
	if _, ok, err := LoadCoverage(dir); ok || err != nil {
		t.Fatalf("proyecto sin cobertura: ok=%v err=%v", ok, err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "coverage"), 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "coverage", "coverage-summary.json")
	if err := os.WriteFile(path, []byte(istanbulSample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadCoverage(dir)
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Path != path {
		t.Errorf("path = %q, want %q", got.Path, path)
	}
	if got.Lines.Pct != 85 {
		t.Errorf("lines pct = %v", got.Lines.Pct)
	}

	files := LoadCoverageFiles(dir, 1)
	if len(files) != 1 || files[0].Path != "/a/b.js" {
		t.Errorf("files limitados = %+v", files)
	}
}

func TestDetectCoverageFilePriorizaSummary(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte("mode: set\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "coverage"), 0o755); err != nil {
		t.Fatal(err)
	}
	summary := filepath.Join(dir, "coverage", "coverage-summary.json")
	if err := os.WriteFile(summary, []byte(istanbulSample), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := DetectCoverageFile(dir); got != summary {
		t.Errorf("got %q, want %q", got, summary)
	}
	if got := DetectCoverageFile(""); got != "" {
		t.Errorf("path vacío debería devolver vacío, got %q", got)
	}
	if !strings.HasSuffix(DetectCoverageFile(dir), "coverage-summary.json") {
		t.Error("debe priorizar coverage-summary.json")
	}
}
