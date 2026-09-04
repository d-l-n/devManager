package deps

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectManager(t *testing.T) {
	d := t.TempDir()
	if got := DetectManager(d); got != "" {
		t.Errorf("sin manifest debe ser empty, got %q", got)
	}
	write(t, filepath.Join(d, "package.json"), "{}")
	if got := DetectManager(d); got != "npm" {
		t.Errorf("package.json -> npm, got %q", got)
	}
	write(t, filepath.Join(d, "yarn.lock"), "")
	if got := DetectManager(d); got != "yarn" {
		t.Errorf("yarn.lock -> yarn, got %q", got)
	}
	d2 := t.TempDir()
	write(t, filepath.Join(d2, "pnpm-lock.yaml"), "")
	if got := DetectManager(d2); got != "pnpm" {
		t.Errorf("pnpm-lock.yaml -> pnpm, got %q", got)
	}
	d3 := t.TempDir()
	write(t, filepath.Join(d3, "go.mod"), "module x\n")
	if got := DetectManager(d3); got != "go" {
		t.Errorf("go.mod -> go, got %q", got)
	}
	write(t, filepath.Join(d3, "package.json"), "{}")
	if got := DetectManager(d3); got != "go" {
		t.Errorf("go.mod presente debe ganar sobre package.json, got %q", got)
	}
}

func TestParseNPM(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "package.json"), `{
  "dependencies": { "lodash": "^4.17.0", "react": "18.2.0" },
  "devDependencies": { "vite": "^5.0.0" }
}`)
	deps, err := ParseNPM(filepath.Join(d, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 3 {
		t.Fatalf("esperado 3 deps, got %d", len(deps))
	}
	names := map[string]bool{}
	for _, dp := range deps {
		names[dp.Name] = true
		if dp.Current == "" {
			t.Errorf("dep %s sin version current", dp.Name)
		}
	}
	for _, want := range []string{"lodash", "react", "vite"} {
		if !names[want] {
			t.Errorf("falta dep %s", want)
		}
	}
}

func TestParseGoMod(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "go.mod"), `module example.com/foo

go 1.25

require (
	github.com/google/uuid v1.3.0
	golang.org/x/sync v0.1.0
)
`)
	deps, err := ParseGoMod(filepath.Join(d, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 {
		t.Fatalf("esperado 2 deps, got %d", len(deps))
	}
	if deps[0].Name != "github.com/google/uuid" || deps[0].Current != "v1.3.0" {
		t.Errorf("primera dep mal, got %+v", deps[0])
	}
}

func TestParseNPMMissingFile(t *testing.T) {
	if _, err := ParseNPM(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Error("archivo inexistente debe dar error")
	}
}

func TestListDepsEmptyDir(t *testing.T) {
	r := ListDeps(t.TempDir())
	if r.Manager != "" || len(r.Deps) != 0 || r.Error != "" {
		t.Errorf("dir vacío: %+v", r)
	}
}

func TestListDepsNPM(t *testing.T) {
	d := t.TempDir()
	write(t, filepath.Join(d, "package.json"), `{"dependencies":{"lodash":"1.0.0"}}`)
	r := ListDeps(d)
	if r.Manager != "npm" || len(r.Deps) != 1 || r.Deps[0].Name != "lodash" {
		t.Errorf("ListDeps npm: %+v", r)
	}
}
