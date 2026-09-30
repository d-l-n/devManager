package help

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const changelogSample = `# Changelog

All notable changes to devManager will be documented in this file.

## [Unreleased]

## [2.1.0] - 2026-09-07

### 🚀 New Features
- **Built-in Updater (#58)**: version check against GitHub Releases
- **Dependency Inspector (#61)**: manifest parse + audit

### 🐛 Bug Fixes
- Fix issues #43-#56: validation, duplicate project handling

## [2.0.1] - 2026-09-01

### New Features
- 3 new UI themes

---

## [2.0.0] - 2026-08-20

### Breaking
- projects.json schema v2
`

func TestParseChangelog(t *testing.T) {
	releases := ParseChangelog(changelogSample)
	if len(releases) != 4 {
		t.Fatalf("releases = %d, want 4 (%+v)", len(releases), releases)
	}
	first := releases[0]
	if first.Version != "Unreleased" || first.Date != "" {
		t.Errorf("primera release = %+v", first)
	}
	if len(first.Sections) != 0 {
		t.Errorf("Unreleased sin secciones = %+v", first.Sections)
	}

	second := releases[1]
	if second.Version != "2.1.0" || second.Date != "2026-09-07" {
		t.Errorf("segunda release = %+v", second)
	}
	if len(second.Sections) != 2 {
		t.Fatalf("secciones = %+v", second.Sections)
	}
	if !strings.Contains(second.Sections[0].Title, "New Features") {
		t.Errorf("título sección = %q", second.Sections[0].Title)
	}
	if len(second.Sections[0].Items) != 2 {
		t.Errorf("items = %+v", second.Sections[0].Items)
	}
	if !strings.Contains(second.Sections[0].Items[0], "Built-in Updater") {
		t.Errorf("item = %q", second.Sections[0].Items[0])
	}
	if len(second.Sections[1].Items) != 1 {
		t.Errorf("bug fixes = %+v", second.Sections[1].Items)
	}

	// El separador "---" no debe convertirse en item ni romper la siguiente release.
	last := releases[3]
	if last.Version != "2.0.0" || last.Date != "2026-08-20" {
		t.Errorf("última release = %+v", last)
	}
	if len(last.Sections) != 1 || len(last.Sections[0].Items) != 1 {
		t.Errorf("última sección = %+v", last.Sections)
	}
}

func TestParseChangelogTolerante(t *testing.T) {
	if got := ParseChangelog(""); len(got) != 0 {
		t.Errorf("vacío = %+v", got)
	}
	// Contenido antes de la primera release se ignora.
	got := ParseChangelog("# Changelog\n\nTexto suelto\n\n## [1.0.0]\n\n- item sin sección\n")
	if len(got) != 1 {
		t.Fatalf("releases = %+v", got)
	}
	if got[0].Version != "1.0.0" || got[0].Date != "" {
		t.Errorf("release = %+v", got[0])
	}
	if len(got[0].Sections) != 0 {
		t.Errorf("sin sección ### no deben crearse secciones: %+v", got[0].Sections)
	}
}

func TestLatest(t *testing.T) {
	releases := ParseChangelog(changelogSample)
	got := Latest(releases, 2)
	if len(got) != 2 || got[0].Version != "Unreleased" {
		t.Errorf("latest 2 = %+v", got)
	}
	if all := Latest(releases, 0); len(all) != len(releases) {
		t.Errorf("limit 0 = %d releases", len(all))
	}
	if over := Latest(releases, 99); len(over) != len(releases) {
		t.Errorf("limit mayor al total = %d", len(over))
	}
	// No debe mutar el slice original.
	got[0].Version = "mutado"
	if releases[0].Version != "Unreleased" {
		t.Error("Latest debe copiar, no aliasar")
	}
}

func TestLoadChangelogYPaths(t *testing.T) {
	dir := t.TempDir()
	if got := LoadChangelog(dir); len(got) != 0 {
		t.Errorf("sin CHANGELOG.md = %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(changelogSample), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadChangelog(dir)
	if len(got) != 4 {
		t.Fatalf("cargado = %d releases", len(got))
	}
	if got[1].Version != "2.1.0" {
		t.Errorf("primera versión real = %q", got[1].Version)
	}

	paths := ChangelogCandidatePaths(dir)
	if len(paths) < 3 {
		t.Fatalf("paths = %v", paths)
	}
	if paths[0] != filepath.Join(dir, "CHANGELOG.md") {
		t.Errorf("primer path = %q", paths[0])
	}
	seen := map[string]bool{}
	for _, p := range paths {
		if seen[p] {
			t.Errorf("path duplicado: %q", p)
		}
		seen[p] = true
	}
	if len(ChangelogCandidatePaths("")) == 0 {
		t.Error("sin exeDir debería caer al CWD")
	}
}

// TestChangelogRealDelRepo valida el parser contra el CHANGELOG.md real: es el
// que el usuario verá en la app, así que no puede parsearse a medias.
func TestChangelogRealDelRepo(t *testing.T) {
	path := filepath.Join("..", "..", "..", "CHANGELOG.md")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("CHANGELOG.md del repo no accesible desde el test: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer CHANGELOG.md: %v", err)
	}
	releases := ParseChangelog(string(data))
	if len(releases) < 3 {
		t.Fatalf("releases parseadas = %d", len(releases))
	}
	// La primera entrada puede ser "[Unreleased]" (sin fecha) si hay trabajo en
	// curso; en ese caso validamos contra la primera release fechada.
	first := releases[0]
	if first.Version == "Unreleased" && first.Date == "" {
		if len(releases) < 2 {
			t.Fatalf("sólo hay [Unreleased], ninguna release fechada")
		}
		first = releases[1]
	}
	if first.Version == "" || first.Date == "" {
		t.Errorf("primera release fechada sin versión/fecha: %+v", first)
	}
	totalItems := 0
	for _, r := range releases {
		if len(r.Sections) == 0 {
			t.Errorf("release %q sin secciones", r.Version)
		}
		for _, s := range r.Sections {
			if s.Title == "" {
				t.Errorf("release %q con sección sin título", r.Version)
			}
			totalItems += len(s.Items)
		}
	}
	if totalItems < 20 {
		t.Errorf("sólo %d items parseados (parece un parseo parcial)", totalItems)
	}
}
