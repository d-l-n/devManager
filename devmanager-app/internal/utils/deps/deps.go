// Package deps porta la gestión de dependencias (Issue #61/#62 - núcleo).
package deps

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// DetectManager infiere el gestor de paquetes del directorio.
// Prioridad: go.mod > pnpm-lock.yaml > yarn.lock > package.json.
func DetectManager(dir string) string {
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(dir, "pnpm-lock.yaml")); err == nil {
		return "pnpm"
	}
	if _, err := os.Stat(filepath.Join(dir, "yarn.lock")); err == nil {
		return "yarn"
	}
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		return "npm"
	}
	return ""
}

// Dep es una dependencia con su estado de actualización.
type Dep struct {
	Name     string `json:"name"`
	Current  string `json:"current"`
	Latest   string `json:"latest"`
	Outdated bool   `json:"outdated"`
}

type npmManifest struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParseNPM parsea package.json unificando deps y devDeps.
func ParseNPM(manifestPath string) ([]Dep, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, err
	}
	var m npmManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	var deps []Dep
	for name, ver := range m.Dependencies {
		deps = append(deps, Dep{Name: name, Current: ver})
	}
	for name, ver := range m.DevDependencies {
		deps = append(deps, Dep{Name: name, Current: ver})
	}
	return deps, nil
}

// ParseGoMod parsea bloques require de go.mod.
func ParseGoMod(goModPath string) ([]Dep, error) {
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return nil, err
	}
	var deps []Dep
	block := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "require (" {
			block = true
			continue
		}
		if block && line == ")" {
			block = false
			continue
		}
		if block && line != "" {
			if d, ok := parseGoRequire(line); ok {
				deps = append(deps, d)
			}
			continue
		}
		if strings.HasPrefix(line, "require ") && !strings.Contains(line, "(") {
			if d, ok := parseGoRequire(strings.TrimPrefix(line, "require ")); ok {
				deps = append(deps, d)
			}
		}
	}
	return deps, nil
}

// parseGoRequire extrae "name v1.2.3" de una línea require.
func parseGoRequire(line string) (Dep, bool) {
	fields := strings.Split(strings.TrimSpace(line), " ")
	var fields2 []string
	for _, f := range fields {
		if f != "" {
			fields2 = append(fields2, f)
		}
	}
	if len(fields2) < 2 {
		return Dep{}, false
	}
	return Dep{Name: fields2[0], Current: fields2[1]}, true
}

// ManagerResult agrupa el resultado del listado por gestor.
type ManagerResult struct {
	Manager string `json:"manager"`
	Deps    []Dep  `json:"deps"`
	Error   string `json:"error,omitempty"`
}

// ListDeps detecta el gestor y lista dependencias del manifest.
func ListDeps(dir string) ManagerResult {
	m := DetectManager(dir)
	if m == "" {
		return ManagerResult{}
	}
	var deps []Dep
	var err error
	switch m {
	case "npm", "yarn", "pnpm":
		deps, err = ParseNPM(filepath.Join(dir, "package.json"))
	case "go":
		deps, err = ParseGoMod(filepath.Join(dir, "go.mod"))
	}
	if err != nil {
		return ManagerResult{Manager: m, Error: err.Error()}
	}
	return ManagerResult{Manager: m, Deps: deps}
}

// Vuln es una vulnerabilidad reportada por el audit.
type Vuln struct {
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
}

// AuditResult agrupa el resultado del audit de seguridad.
type AuditResult struct {
	Manager string `json:"manager"`
	Vulns   []Vuln `json:"vulns"`
	Error   string `json:"error,omitempty"`
}

var errGovulncheckMissing = errors.New("govulncheck no está instalado: ejecutá 'go install golang.org/x/vuln/cmd/govulncheck@latest'")
