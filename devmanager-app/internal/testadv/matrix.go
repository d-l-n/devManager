package testadv

import (
	"sort"
	"strings"
	"time"
)

// MatrixCell es una combinación navegador x entorno. Status: pending | passed |
// failed.
type MatrixCell struct {
	Browser string    `json:"browser"`
	Env     string    `json:"env"`
	Status  string    `json:"status"`
	Runs    int       `json:"runs"`
	LastRun time.Time `json:"lastRun,omitempty"`
}

// DefaultBrowsers son los proyectos Playwright por defecto.
func DefaultBrowsers() []string { return []string{"chromium", "firefox", "webkit"} }

// browserAliases normaliza nombres comunes a los projects de Playwright.
var browserAliases = map[string]string{
	"chrome":                  "chromium",
	"googlechrome":            "chromium",
	"edge":                    "chromium",
	"msedge":                  "chromium",
	"gecko":                   "firefox",
	"safari":                  "webkit",
	"chromium-headless-shell": "chromium",
}

// NormalizeBrowsers limpia duplicados, normaliza alias y aplica los defaults
// cuando la lista viene vacía (nunca devuelve una matriz vacía por descuido).
func NormalizeBrowsers(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range in {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if canonical, ok := browserAliases[name]; ok {
			name = canonical
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return DefaultBrowsers()
	}
	return out
}

// NormalizeEnvs limpia, ordena y aplica "dev" como default.
func NormalizeEnvs(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range in {
		name := strings.ToLower(strings.TrimSpace(raw))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	if len(out) == 0 {
		return []string{"dev"}
	}
	sort.Strings(out)
	return out
}

// MatrixKey es la clave estable de una celda (para mapear resultados).
func MatrixKey(browser, env string) string {
	return strings.ToLower(browser) + "|" + strings.ToLower(env)
}

// BuildMatrix genera la matriz completa (browser-major) en estado pending.
func BuildMatrix(browsers, envs []string) []MatrixCell {
	bs := NormalizeBrowsers(browsers)
	es := NormalizeEnvs(envs)
	out := make([]MatrixCell, 0, len(bs)*len(es))
	for _, b := range bs {
		for _, e := range es {
			out = append(out, MatrixCell{Browser: b, Env: e, Status: "pending"})
		}
	}
	return out
}

// ApplyResults vuelca resultados conocidos (clave MatrixKey) sobre la matriz.
func ApplyResults(cells []MatrixCell, results map[string]MatrixCell) []MatrixCell {
	for i := range cells {
		res, ok := results[MatrixKey(cells[i].Browser, cells[i].Env)]
		if !ok {
			continue
		}
		if res.Status != "" {
			cells[i].Status = res.Status
		}
		if res.Runs > 0 {
			cells[i].Runs = res.Runs
		}
		if !res.LastRun.IsZero() {
			cells[i].LastRun = res.LastRun
		}
	}
	return cells
}

// MatrixProgress resume cuántas combinaciones se ejecutaron y el pass rate.
func MatrixProgress(cells []MatrixCell) (done, total int, passRate float64) {
	total = len(cells)
	passed := 0
	for _, c := range cells {
		if c.Status == "passed" || c.Status == "failed" {
			done++
		}
		if c.Status == "passed" {
			passed++
		}
	}
	if done > 0 {
		passRate = round2(float64(passed) / float64(done) * 100)
	} else {
		passRate = 0
	}
	return done, total, passRate
}
