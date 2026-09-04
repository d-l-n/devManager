// Package deps porta la gestión de dependencias (Issue #61/#62 - núcleo).
package deps

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
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

// CheckOutdated rellena Latest/Outdated de cada Dep invocando el gestor.
func CheckOutdated(dir string, deps []Dep) []Dep {
	switch DetectManager(dir) {
	case "npm", "yarn", "pnpm":
		return checkOutdatedNPM(dir, deps)
	case "go":
		return checkOutdatedGo(dir, deps)
	}
	return deps
}

// npmOutdated es la estructura de `npm outdated --json`.
type npmOutdated map[string]struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
}

func checkOutdatedNPM(dir string, deps []Dep) []Dep {
	cmd := exec.Command("npm", "outdated", "--json")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // sin red o sin outdated: no error de tool

	out := stdout.Bytes()
	if len(out) == 0 {
		out = stderr.Bytes()
	}
	if len(out) == 0 {
		// npm sin outdated: salida vacía (o no hay red). Sin cambios.
		return deps
	}
	var m npmOutdated
	if err := json.Unmarshal(out, &m); err != nil {
		return deps
	}
	for i := range deps {
		if o, ok := m[deps[i].Name]; ok && o.Latest != "" {
			deps[i].Latest = o.Latest
			deps[i].Outdated = true
		}
	}
	return deps
}

func checkOutdatedGo(dir string, deps []Dep) []Dep {
	// `go list -m -u -json all` devuelve por módulo: {... "Update": {...}}.
	cmd := exec.Command("go", "list", "-m", "-u", "-json", "all")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			return deps
		}
	}
	var latest map[string]string
	dec := json.NewDecoder(strings.NewReader(string(stdout.Bytes())))
	for {
		var mod struct {
			Path   string `json:"Path"`
			Update *struct {
				Version string `json:"Version"`
			} `json:"Update"`
		}
		if e := dec.Decode(&mod); e != nil {
			break
		}
		if mod.Update != nil {
			latest[mod.Path] = mod.Update.Version
		}
	}
	for i := range deps {
		if lv, ok := latest[deps[i].Name]; ok {
			deps[i].Latest = lv
			deps[i].Outdated = true
		}
	}
	return deps
}

// RunAudit corre audit de seguridad del gestor detectado.
// npm/yarn/pnpm: `npm audit --json`; go: `govulncheck` (si está instalado).
func RunAudit(dir string) AuditResult {
	m := DetectManager(dir)
	if m == "" {
		return AuditResult{}
	}
	switch m {
	case "npm", "yarn", "pnpm":
		return auditNPM(dir, m)
	case "go":
		return auditGo(dir)
	}
	return AuditResult{Manager: m}
}

// auditNPM parsea `npm audit --json`.
func auditNPM(dir, manager string) AuditResult {
	cmd := exec.Command("npm", "audit", "--json")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	_ = cmd.Run() // exit 1 esperado con vulns; no tratamos como fallo

	out := stdout.Bytes()
	if len(out) == 0 {
		out = stderr.Bytes()
	}
	var report struct {
		Vulnerabilities map[string]struct {
			Severity string            `json:"severity"`
			IsDirect bool              `json:"isDirect"`
			Via      []json.RawMessage `json:"via"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(out, &report); err != nil {
		return AuditResult{Manager: manager, Error: "npm audit: respuesta inesperada"}
	}
	var vulns []Vuln
	for name, v := range report.Vulnerabilities {
		if len(v.Via) == 0 {
			continue // hoja sin falla directa
		}
		title := name
		var first struct {
			Title string `json:"title"`
		}
		if json.Unmarshal(v.Via[0], &first) == nil && first.Title != "" {
			title = first.Title
		}
		vulns = append(vulns, Vuln{
			Name:     name,
			Severity: v.Severity,
			Title:    title,
		})
	}
	return AuditResult{Manager: manager, Vulns: vulns}
}

// auditGo corre govulncheck --json si está disponible.
func auditGo(dir string) AuditResult {
	cmd := exec.Command("govulncheck", "-json", "./...")
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return AuditResult{Manager: "go", Error: errGovulncheckMissing.Error()}
		}
		// con vulns govulncheck sale != 0 pero emite JSON: parseamos igual
	}
	var rep struct {
		Findings []struct {
			OSV *struct {
				ID      string `json:"id"`
				Summary string `json:"summary"`
			} `json:"osv"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		return AuditResult{Manager: "go", Error: "govulncheck: respuesta inesperada (o sin vulns)"}
	}
	var vulns []Vuln
	for _, f := range rep.Findings {
		if f.OSV == nil {
			continue
		}
		vulns = append(vulns, Vuln{
			Name:     f.OSV.ID,
			Severity: "high",
			Title:    firstLine(f.OSV.Summary),
		})
	}
	return AuditResult{Manager: "go", Vulns: vulns}
}

// firstLine trunca un texto en el primer salto de línea.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
