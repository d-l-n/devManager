package main

import (
	"crypto/sha1"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/testadv"
)

// ---- Advanced testing bindings (Issue #70) ----
//
// Este fichero NO toca app.go: los bindings nuevos viven aquí para no
// colisionar con el trabajo de #67 (ver nota de integración en el PR).
// Estado persistido en %APPDATA%/devManager/testadv/<proyecto>.json (patrón de
// settings.json/backups: estable entre installs, fuera del proyecto).
// Los artefactos que se leen/escriben SÍ viven en el proyecto
// (coverage/, test-results/visual/), junto al resto de evidencias.

const (
	testadvRunKeep                = 500
	testadvDefaultThreshold       = 80.0
	testadvVisualToleranceDefault = 12
	testadvVisualMaxPercent       = 0.5
)

// TestCoverageInfo es la cobertura del proyecto lista para pintar.
type TestCoverageInfo struct {
	Available bool                    `json:"available"`
	Path      string                  `json:"path,omitempty"`
	Summary   testadv.CoverageSummary `json:"summary"`
	Worst     []testadv.FileCoverage  `json:"worst"`
}

// VisualInfo es el estado de la regresión visual.
type VisualInfo struct {
	Available   bool                   `json:"available"`
	BaselineDir string                 `json:"baselineDir"`
	CurrentDir  string                 `json:"currentDir"`
	DiffDir     string                 `json:"diffDir"`
	Tolerance   int                    `json:"tolerance"`
	MaxPercent  float64                `json:"maxPercent"`
	Total       int                    `json:"total"`
	Same        int                    `json:"same"`
	Changed     int                    `json:"changed"`
	New         int                    `json:"new"`
	Missing     int                    `json:"missing"`
	Shots       int                    `json:"shots"`
	Results     []testadv.VisualResult `json:"results"`
}

// MatrixProgress es el avance de la matriz navegador x entorno.
type MatrixProgress struct {
	Done     int     `json:"done"`
	Total    int     `json:"total"`
	PassRate float64 `json:"passRate"`
}

// TestSchedulesInfo agrupa las tareas programadas y las vencidas.
type TestSchedulesInfo struct {
	Schedules []testadv.Schedule `json:"schedules"`
	Due       []testadv.Schedule `json:"due"`
}

// TestAdvSummary es el payload único del panel (una llamada por refresco).
type TestAdvSummary struct {
	Project       string                    `json:"project"`
	ActiveEnv     string                    `json:"activeEnv"`
	Envs          []string                  `json:"envs"`
	Browsers      []string                  `json:"browsers"`
	Threshold     float64                   `json:"threshold"`
	Coverage      TestCoverageInfo          `json:"coverage"`
	Perf          testadv.PerfMetrics       `json:"perf"`
	PerfAvailable bool                      `json:"perfAvailable"`
	PerfChecks    []testadv.ThresholdResult `json:"perfChecks"`
	Thresholds    testadv.Thresholds        `json:"thresholds"`
	Visual        VisualInfo                `json:"visual"`
	Analytics     testadv.Analytics         `json:"analytics"`
	Schedules     TestSchedulesInfo         `json:"schedules"`
	FixtureDirs   []string                  `json:"fixtureDirs"`
	Fixtures      []testadv.FixtureSet      `json:"fixtures"`
	Matrix        []testadv.MatrixCell      `json:"matrix"`
	MatrixState   MatrixProgress            `json:"matrixProgress"`
}

// ---- paths ----

// testadvDir es %APPDATA%/devManager/testadv (junto a settings.json).
func (a *App) testadvDir() string {
	return filepath.Join(filepath.Dir(a.settingsPath), "testadv")
}

func (a *App) testadvStorePath(p models.Project) string {
	return filepath.Join(a.testadvDir(), testadvSlug(p)+".json")
}

func (a *App) testadvRunsPath(p models.Project) string {
	return filepath.Join(a.testadvDir(), testadvSlug(p)+"-runs.jsonl")
}

// visualDirs devuelve baseline/current/diff dentro de test-results/ del
// proyecto (el tab Evidence ya escanea test-results/ → los diffs son visibles).
func visualDirs(p models.Project) (baseline, current, diff string) {
	root := filepath.Join(p.Path, "test-results", "visual")
	return filepath.Join(root, "baseline"), filepath.Join(root, "current"), filepath.Join(root, "diff")
}

// testadvSlug identifica el proyecto de forma estable: nombre legible + hash
// corto del path (dos proyectos pueden llamarse igual en rutas distintas).
func testadvSlug(p models.Project) string {
	sum := sha1.Sum([]byte(strings.ToLower(filepath.ToSlash(p.Path))))
	return fmt.Sprintf("%s-%x", projectSlug(p.Name), sum[:4])
}

// projectSlug normaliza un nombre a [a-z0-9-] (máx 32) con fallback "project".
func projectSlug(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "project"
	}
	if len(out) > 32 {
		out = strings.Trim(out[:32], "-")
	}
	return out
}

// projectEnvs devuelve los entornos del proyecto (fallback: el activo o dev).
func projectEnvs(p models.Project) []string {
	names := make([]string, 0, len(p.Envs))
	for name := range p.Envs {
		names = append(names, name)
	}
	if len(names) == 0 {
		if p.ActiveEnv != "" {
			names = append(names, p.ActiveEnv)
		} else {
			names = append(names, "dev")
		}
	}
	sort.Strings(names)
	return testadv.NormalizeEnvs(names)
}

func (a *App) testadvRuns(p models.Project) []testadv.RunRecord {
	runs, err := testadv.LoadRuns(a.testadvRunsPath(p), 0)
	if err != nil {
		return []testadv.RunRecord{}
	}
	return runs
}

// matrixResultsFromRuns deriva el estado de cada celda del histórico: gana el
// run más reciente (Status = passed|failed de la última ejecución), y Runs
// acumula cuántas veces se corrió esa combinación.
func matrixResultsFromRuns(runs []testadv.RunRecord) map[string]testadv.MatrixCell {
	out := map[string]testadv.MatrixCell{}
	for _, r := range runs {
		if r.Browser == "" || r.Env == "" {
			continue
		}
		key := testadv.MatrixKey(r.Browser, r.Env)
		cell := out[key]
		cell.Browser = strings.ToLower(r.Browser)
		cell.Env = strings.ToLower(r.Env)
		cell.Runs++
		if cell.LastRun.IsZero() || !r.StartedAt.Before(cell.LastRun) {
			if r.Failed > 0 {
				cell.Status = "failed"
			} else {
				cell.Status = "passed"
			}
			cell.LastRun = r.StartedAt
		}
		out[key] = cell
	}
	return out
}

// ---- coverage ----

// GetTestCoverage detecta y resume la cobertura del proyecto.
func (a *App) GetTestCoverage(index int, threshold float64) TestCoverageInfo {
	p := a.currentProject(index)
	info := TestCoverageInfo{Worst: []testadv.FileCoverage{}}
	if p.Path == "" {
		return info
	}
	summary, ok, err := testadv.LoadCoverage(p.Path)
	if err != nil || !ok {
		return info
	}
	info.Available = true
	info.Path = summary.Path
	info.Summary = summary.WithThreshold(normalizeThreshold(threshold))
	info.Worst = testadv.LoadCoverageFiles(p.Path, 8)
	return info
}

func normalizeThreshold(threshold float64) float64 {
	if threshold <= 0 || threshold > 100 {
		return testadvDefaultThreshold
	}
	return threshold
}

// ---- visual ----

// GetVisualReport compara baseline vs current de test-results/visual.
func (a *App) GetVisualReport(index int, tolerance int, maxPercent float64) VisualInfo {
	p := a.currentProject(index)
	baseline, current, diff := visualDirs(p)
	info := VisualInfo{
		BaselineDir: baseline,
		CurrentDir:  current,
		DiffDir:     diff,
		Tolerance:   tolerance,
		MaxPercent:  maxPercent,
		Results:     []testadv.VisualResult{},
	}
	if p.Path == "" {
		return info
	}
	if tolerance < 0 || tolerance > 255 {
		tolerance = testadvVisualToleranceDefault
	}
	if maxPercent < 0 {
		maxPercent = testadvVisualMaxPercent
	}
	info.Tolerance = tolerance
	info.MaxPercent = maxPercent
	report, err := testadv.CompareDirs(baseline, current, diff, uint8(tolerance), maxPercent)
	if err != nil {
		return info
	}
	info.Available = report.Available
	info.Total = report.Total
	info.Same = report.Same
	info.Changed = report.Changed
	info.New = report.New
	info.Missing = report.Missing
	info.Results = report.Results
	info.Shots = report.Same + report.Changed + report.New
	return info
}

// PromoteVisualBaseline acepta los screenshots actuales como nueva referencia.
func (a *App) PromoteVisualBaseline(index int) (int, error) {
	p := a.currentProject(index)
	if p.Path == "" {
		return 0, fmt.Errorf("project index %d out of range", index)
	}
	baseline, current, _ := visualDirs(p)
	return testadv.PromoteBaseline(current, baseline)
}

// ---- analytics ----

// GetTestAnalytics agrega el histórico de runs del proyecto.
func (a *App) GetTestAnalytics(index int, buckets int) testadv.Analytics {
	p := a.currentProject(index)
	return testadv.Analyze(a.testadvRuns(p), buckets)
}

// AppendTestRun registra una ejecución en el histórico (y alimenta la matriz).
func (a *App) AppendTestRun(index int, rec testadv.RunRecord) []string {
	p := a.currentProject(index)
	if p.Path == "" {
		return []string{fmt.Sprintf("project index %d out of range", index)}
	}
	if rec.StartedAt.IsZero() {
		rec.StartedAt = time.Now()
	}
	if rec.Passed < 0 || rec.Failed < 0 || rec.Skipped < 0 {
		return []string{"run counts cannot be negative"}
	}
	if rec.Env == "" {
		rec.Env = projectEnvs(p)[0]
	}
	if rec.Browser == "" {
		rec.Browser = testadv.DefaultBrowsers()[0]
	}
	rec.Browser = strings.ToLower(rec.Browser)
	rec.Env = strings.ToLower(rec.Env)
	if err := testadv.AppendRun(a.testadvRunsPath(p), rec, testadvRunKeep); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// ---- schedules ----

// GetTestSchedules lista las tareas programadas (recalculando NextRun).
func (a *App) GetTestSchedules(index int) TestSchedulesInfo {
	p := a.currentProject(index)
	info := TestSchedulesInfo{Schedules: []testadv.Schedule{}, Due: []testadv.Schedule{}}
	if p.Path == "" {
		return info
	}
	store := testadv.LoadStore(a.testadvStorePath(p))
	now := time.Now()
	for i := range store.Schedules {
		store.Schedules[i].Recompute(now)
	}
	testadv.SortByNext(store.Schedules)
	info.Schedules = store.Schedules
	info.Due = testadv.Due(store.Schedules, now)
	return info
}

// SaveTestSchedule crea (ID vacío) o actualiza una tarea programada.
func (a *App) SaveTestSchedule(index int, s testadv.Schedule) []string {
	p := a.currentProject(index)
	if p.Path == "" {
		return []string{fmt.Sprintf("project index %d out of range", index)}
	}
	s.Name = strings.TrimSpace(s.Name)
	s.Spec = strings.TrimSpace(s.Spec)
	s.Command = strings.TrimSpace(s.Command)
	if errs := s.Validate(); len(errs) > 0 {
		return errs
	}
	now := time.Now()
	store := testadv.LoadStore(a.testadvStorePath(p))
	if s.ID == "" {
		s.ID = newScheduleID(store.Schedules, s.Name, now)
		s.Enabled = true
	} else if s.ID != "" {
		found := false
		for _, existing := range store.Schedules {
			if existing.ID == s.ID {
				found = true
				break
			}
		}
		if !found {
			return []string{fmt.Sprintf("schedule %q does not exist", s.ID)}
		}
	}
	s.Recompute(now)
	replaced := false
	for i := range store.Schedules {
		if store.Schedules[i].ID == s.ID {
			store.Schedules[i] = s
			replaced = true
			break
		}
	}
	if !replaced {
		store.Schedules = append(store.Schedules, s)
	}
	testadv.SortByNext(store.Schedules)
	if err := testadv.SaveStore(a.testadvStorePath(p), store); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DeleteTestSchedule elimina una tarea por ID.
func (a *App) DeleteTestSchedule(index int, id string) []string {
	p := a.currentProject(index)
	if p.Path == "" {
		return []string{fmt.Sprintf("project index %d out of range", index)}
	}
	store := testadv.LoadStore(a.testadvStorePath(p))
	out := make([]testadv.Schedule, 0, len(store.Schedules))
	removed := false
	for _, s := range store.Schedules {
		if s.ID == id {
			removed = true
			continue
		}
		out = append(out, s)
	}
	if !removed {
		return []string{fmt.Sprintf("schedule %q does not exist", id)}
	}
	store.Schedules = out
	if err := testadv.SaveStore(a.testadvStorePath(p), store); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// MarkTestScheduleRun registra el resultado de una ejecución programada y
// reprograma la siguiente (el executor real vive en el runner de tests).
func (a *App) MarkTestScheduleRun(index int, id string, result string) []string {
	p := a.currentProject(index)
	if p.Path == "" {
		return []string{fmt.Sprintf("project index %d out of range", index)}
	}
	switch result {
	case "passed", "failed", "error":
	default:
		return []string{fmt.Sprintf("invalid run result %q (passed|failed|error)", result)}
	}
	store := testadv.LoadStore(a.testadvStorePath(p))
	now := time.Now()
	found := false
	for i := range store.Schedules {
		if store.Schedules[i].ID != id {
			continue
		}
		store.Schedules[i].LastRun = now
		store.Schedules[i].LastResult = result
		store.Schedules[i].Recompute(now)
		found = true
		break
	}
	if !found {
		return []string{fmt.Sprintf("schedule %q does not exist", id)}
	}
	testadv.SortByNext(store.Schedules)
	if err := testadv.SaveStore(a.testadvStorePath(p), store); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// newScheduleID genera un ID estable a partir del nombre con sufijo -2, -3…
func newScheduleID(existing []testadv.Schedule, name string, now time.Time) string {
	base := projectSlug(name)
	if base == "project" {
		base = "schedule-" + now.Format("150405")
	}
	taken := map[string]bool{}
	for _, s := range existing {
		taken[s.ID] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d", base, i)
		if !taken[candidate] {
			return candidate
		}
	}
}

// ---- matrix & fixtures ----

// GetTestMatrix devuelve la matriz navegador x entorno con el histórico aplicado.
func (a *App) GetTestMatrix(index int) []testadv.MatrixCell {
	p := a.currentProject(index)
	cells := testadv.BuildMatrix(testadv.DefaultBrowsers(), projectEnvs(p))
	return testadv.ApplyResults(cells, matrixResultsFromRuns(a.testadvRuns(p)))
}

// GetTestFixtures lista los sets de datos de test detectados en el proyecto.
func (a *App) GetTestFixtures(index int) []testadv.FixtureSet {
	p := a.currentProject(index)
	if p.Path == "" {
		return []testadv.FixtureSet{}
	}
	return testadv.ListFixtures(p.Path)
}

// ---- summary ----

// GetTestAdvSummary es el refresco completo del panel (una sola llamada).
func (a *App) GetTestAdvSummary(index int, threshold float64) TestAdvSummary {
	p := a.currentProject(index)
	summary := TestAdvSummary{
		Project:     p.Name,
		ActiveEnv:   p.ActiveEnv,
		Envs:        projectEnvs(p),
		Browsers:    testadv.DefaultBrowsers(),
		Threshold:   normalizeThreshold(threshold),
		Coverage:    TestCoverageInfo{Worst: []testadv.FileCoverage{}},
		PerfChecks:  []testadv.ThresholdResult{},
		Visual:      VisualInfo{Results: []testadv.VisualResult{}},
		Schedules:   TestSchedulesInfo{Schedules: []testadv.Schedule{}, Due: []testadv.Schedule{}},
		FixtureDirs: testadv.FixtureDirNames(),
		Fixtures:    []testadv.FixtureSet{},
		Matrix:      []testadv.MatrixCell{},
	}
	if p.Path == "" {
		summary.Analytics = testadv.Analyze(nil, 0)
		return summary
	}

	summary.Coverage = a.GetTestCoverage(index, threshold)

	perf, ok, err := testadv.LoadPerf(p.Path)
	if err == nil && ok {
		summary.PerfAvailable = true
		summary.Perf = perf
	}

	summary.Visual = a.GetVisualReport(index, testadvVisualToleranceDefault, testadvVisualMaxPercent)
	summary.Analytics = a.GetTestAnalytics(index, 12)
	summary.Schedules = a.GetTestSchedules(index)
	summary.Fixtures = a.GetTestFixtures(index)
	summary.Matrix = a.GetTestMatrix(index)
	done, total, rate := testadv.MatrixProgress(summary.Matrix)
	summary.MatrixState = MatrixProgress{Done: done, Total: total, PassRate: rate}
	return summary
}
