package help

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ReleaseSection es un bloque "### Titulo" del changelog.
type ReleaseSection struct {
	Title string   `json:"title"`
	Items []string `json:"items"`
}

// Release es una versión del changelog.
type Release struct {
	Version  string           `json:"version"`
	Date     string           `json:"date,omitempty"`
	Sections []ReleaseSection `json:"sections"`
}

// ParseChangelog parsea un changelog Keep a Changelog. Es tolerante: ignora
// líneas sueltas, acepta secciones vacías y no exige fecha.
func ParseChangelog(md string) []Release {
	out := []Release{}
	var current *Release
	var sectionIdx = -1

	scanner := bufio.NewScanner(strings.NewReader(md))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			version, date := parseReleaseHeading(strings.TrimPrefix(trimmed, "## "))
			out = append(out, Release{Version: version, Date: date, Sections: []ReleaseSection{}})
			current = &out[len(out)-1]
			sectionIdx = -1
			continue
		}
		if current == nil {
			continue // cabecera del documento
		}
		if strings.HasPrefix(trimmed, "### ") {
			title := strings.TrimSpace(strings.TrimPrefix(trimmed, "### "))
			current.Sections = append(current.Sections, ReleaseSection{Title: title, Items: []string{}})
			sectionIdx = len(current.Sections) - 1
			continue
		}
		if sectionIdx < 0 {
			continue
		}
		if item, ok := changelogItem(trimmed); ok {
			current.Sections[sectionIdx].Items = append(current.Sections[sectionIdx].Items, item)
		}
	}
	return out
}

// parseReleaseHeading separa "[2.1.0] - 2026-09-07" en versión y fecha.
func parseReleaseHeading(raw string) (string, string) {
	version := raw
	date := ""
	if idx := strings.LastIndex(raw, " - "); idx >= 0 {
		version = raw[:idx]
		date = strings.TrimSpace(raw[idx+3:])
	}
	version = strings.TrimSpace(strings.Trim(version, "[]"))
	return version, date
}

// changelogItem acepta "- item" y "* item", descartando separadores.
func changelogItem(line string) (string, bool) {
	if strings.HasPrefix(line, "---") {
		return "", false
	}
	for _, prefix := range []string{"- ", "* "} {
		if strings.HasPrefix(line, prefix) {
			item := strings.TrimSpace(strings.TrimPrefix(line, prefix))
			if item == "" {
				return "", false
			}
			return item, true
		}
	}
	return "", false
}

// Latest devuelve las n versiones más recientes (n<=0 → todas).
func Latest(releases []Release, n int) []Release {
	if n <= 0 || n >= len(releases) {
		out := make([]Release, len(releases))
		copy(out, releases)
		return out
	}
	out := make([]Release, n)
	copy(out, releases[:n])
	return out
}

// ChangelogCandidatePaths lista dónde buscar CHANGELOG.md, en orden: junto al
// ejecutable, un nivel arriba (repo), el CWD y un nivel arriba del CWD.
func ChangelogCandidatePaths(exeDir string) []string {
	paths := []string{}
	if exeDir != "" {
		paths = append(paths,
			filepath.Join(exeDir, "CHANGELOG.md"),
			filepath.Join(exeDir, "..", "CHANGELOG.md"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths,
			filepath.Join(cwd, "CHANGELOG.md"),
			filepath.Join(cwd, "..", "CHANGELOG.md"),
		)
	}
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		clean := filepath.Clean(p)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return out
}

// LoadChangelog lee y parsea el primer CHANGELOG.md encontrado. Devuelve una
// lista vacía si no hay ninguno (nunca es un error de UI).
func LoadChangelog(exeDir string) []Release {
	for _, path := range ChangelogCandidatePaths(exeDir) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		return ParseChangelog(string(data))
	}
	return []Release{}
}
