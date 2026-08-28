package detection

import (
	"os"
	"path/filepath"
	"testing"
)

func mkProject(t *testing.T, pkgJSON string, lockfile string) string {
	t.Helper()
	dir := t.TempDir()
	if pkgJSON != "" {
		writeFile(t, filepath.Join(dir, "package.json"), pkgJSON)
	}
	if lockfile != "" {
		writeFile(t, filepath.Join(dir, lockfile), "")
	}
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func names(ss []Script) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Name
	}
	return out
}

func TestDetectPackageManager(t *testing.T) {
	cases := []struct {
		lock string
		want string
	}{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"bun.lockb", "bun"},
		{"", "npm"},
	}
	for _, tc := range cases {
		dir := mkProject(t, `{"name":"x"}`, tc.lock)
		if got := DetectPackageManager(dir); got != tc.want {
			t.Errorf("DetectPackageManager(lock=%q) = %q, want %q", tc.lock, got, tc.want)
		}
	}
	if got := DetectPackageManager(t.TempDir()); got != "npm" {
		t.Errorf("dir vac├¡o = %q, want npm", got)
	}
	if got := DetectPackageManager(""); got != "npm" {
		t.Errorf("ruta vac├¡a = %q, want npm", got)
	}
}

func TestGetProjectScriptsNPMOrderPreserved(t *testing.T) {
	// Literal crudo (no map+Marshal): json.Marshal de maps ordena claves
	// alfab├®ticamente y destruir├¡a el orden de declaraci├│n a probar.
	raw := `{"name":"x","scripts":{"dev":"vite","start":"node server.js","test":"vitest","build":"vite build"}}`
	dir := mkProject(t, raw, "")

	scripts := GetProjectScripts(dir)
	wantNames := []string{"dev", "start", "test", "build"}
	gotNames := names(scripts)
	if len(gotNames) != len(wantNames) {
		t.Fatalf("scripts = %v, want %v", gotNames, wantNames)
	}
	for i := range wantNames {
		if gotNames[i] != wantNames[i] {
			t.Fatalf("orden roto: got %v, want %v", gotNames, wantNames)
		}
	}
	wantCmds := map[string]string{
		"dev":   "npm run dev",
		"start": "npm start", // start/test sin "run"
		"test":  "npm test",
		"build": "npm run build",
	}
	for _, s := range scripts {
		if s.Command != wantCmds[s.Name] {
			t.Errorf("script %q = %q, want %q", s.Name, s.Command, wantCmds[s.Name])
		}
	}
}

func TestGetProjectScriptsPackageManagerPrefixes(t *testing.T) {
	cases := []struct {
		lock    string
		command string
	}{
		{"pnpm-lock.yaml", "pnpm dev"},
		{"yarn.lock", "yarn dev"},
		{"bun.lockb", "bun run dev"},
	}
	for _, tc := range cases {
		dir := mkProject(t, `{"scripts":{"dev":"vite"}}`, tc.lock)
		scripts := GetProjectScripts(dir)
		if len(scripts) != 1 || scripts[0].Command != tc.command {
			t.Errorf("lock=%q: got %+v, want [%s]", tc.lock, scripts, tc.command)
		}
	}
}

func TestGetProjectScriptsEmptyCases(t *testing.T) {
	if got := GetProjectScripts(""); len(got) != 0 {
		t.Errorf("ruta vac├¡a debe dar vac├¡o, got %v", got)
	}
	if got := GetProjectScripts(t.TempDir()); len(got) != 0 {
		t.Errorf("sin package.json debe dar vac├¡o, got %v", got)
	}
	noScripts := mkProject(t, `{"name":"x","dependencies":{"a":"1"}}`, "")
	if got := GetProjectScripts(noScripts); len(got) != 0 {
		t.Errorf("sin clave scripts debe dar vac├¡o, got %v", got)
	}
	broken := mkProject(t, `{invalid json`, "")
	if got := GetProjectScripts(broken); len(got) != 0 {
		t.Errorf("json corrupto debe dar vac├¡o (paridad try/except), got %v", got)
	}
}
