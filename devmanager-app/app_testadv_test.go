package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/config"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/playwright"
	"github.com/d-l-n/devmanager/internal/scripts"
	"github.com/d-l-n/devmanager/internal/server"
	"github.com/d-l-n/devmanager/internal/testadv"
)

// newTestadvApp crea un App con settingsPath y proyecto en directorios
// temporales (nunca escribe en el CWD ni en %APPDATA% real).
func newTestadvApp(t *testing.T, projectPath string) *App {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "projects.json")
	cfg, err := config.NewManager(cfgPath, config.Options{})
	if err != nil {
		t.Fatalf("config manager: %v", err)
	}
	p := models.Project{Name: "Mi App", Path: projectPath, ActiveEnv: "dev",
		Envs: map[string]models.EnvConfig{"dev": {}, "staging": {}, "prod": {}}}
	if err := cfg.AddProject(p); err != nil {
		t.Fatalf("add project: %v", err)
	}
	return &App{
		cfg:                cfg,
		settingsPath:       filepath.Join(dir, "settings.json"),
		servers:            map[int]*server.Manager{},
		playwrightManagers: map[int]*playwright.Manager{},
		scriptManagers:     map[int]*scripts.Manager{},
	}
}

// pngFixture genera un PNG sólido para los tests de regresión visual.
func pngFixture(t *testing.T, w, h int, r, g, b uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("codificar PNG: %v", err)
	}
	return buf.Bytes()
}

func writeBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestTestadvSlugUnicoPorRuta(t *testing.T) {
	a := newTestadvApp(t, filepath.Join(t.TempDir(), "a"))
	other := newTestadvApp(t, filepath.Join(t.TempDir(), "b"))

	first := a.testadvStorePath(a.currentProject(0))
	second := other.testadvStorePath(other.currentProject(0))
	if first == second {
		t.Fatalf("mismo slug para proyectos distintos: %q", first)
	}
	slug := projectSlug("Mi App")
	if slug != "mi-app" {
		t.Errorf("slug = %q, want mi-app", slug)
	}
	if got := projectSlug("!!! ###"); got != "project" {
		t.Errorf("slug sin caracteres válidos = %q", got)
	}
	if got := projectSlug(strings.Repeat("a", 50)); len(got) > 32 {
		t.Errorf("slug demasiado largo: %d", len(got))
	}
	// Estable entre llamadas (misma ruta → mismo fichero).
	if a.testadvStorePath(a.currentProject(0)) != first {
		t.Error("el slug debe ser estable")
	}
}

func TestTestadvSchedulesCRUD(t *testing.T) {
	a := newTestadvApp(t, t.TempDir())

	// Alta inicial.
	if errs := a.SaveTestSchedule(0, testadv.Schedule{Name: "nightly", Spec: "daily 03:00", Command: "npx playwright test"}); len(errs) != 0 {
		t.Fatalf("guardar: %v", errs)
	}
	info := a.GetTestSchedules(0)
	if len(info.Schedules) != 1 {
		t.Fatalf("schedules = %+v", info.Schedules)
	}
	s := info.Schedules[0]
	if s.ID != "nightly" || !s.Enabled {
		t.Errorf("schedule = %+v", s)
	}
	if s.NextRun.IsZero() || !s.NextRun.After(time.Now()) {
		t.Errorf("nextRun debe ser futuro: %v", s.NextRun)
	}

	// Segundo con el mismo nombre → ID con sufijo.
	if errs := a.SaveTestSchedule(0, testadv.Schedule{Name: "nightly", Spec: "every 30m"}); len(errs) != 0 {
		t.Fatalf("guardar segundo: %v", errs)
	}
	names := []string{}
	for _, s := range a.GetTestSchedules(0).Schedules {
		names = append(names, s.ID)
	}
	if len(names) != 2 {
		t.Fatalf("IDs = %v", names)
	}

	// Validación.
	if errs := a.SaveTestSchedule(0, testadv.Schedule{Name: "", Spec: "every 5m"}); len(errs) == 0 {
		t.Error("sin nombre debería fallar")
	}
	if errs := a.SaveTestSchedule(0, testadv.Schedule{Name: "x", Spec: "every 5s"}); len(errs) == 0 {
		t.Error("spec inválida debería fallar")
	}
	if errs := a.SaveTestSchedule(0, testadv.Schedule{ID: "fantasma", Name: "x", Spec: "daily 01:00"}); len(errs) == 0 {
		t.Error("ID inexistente debería fallar")
	}

	// Marcar ejecución reprograma.
	if errs := a.MarkTestScheduleRun(0, "nightly", "passed"); len(errs) != 0 {
		t.Fatalf("mark: %v", errs)
	}
	if errs := a.MarkTestScheduleRun(0, "nightly", "explotó"); len(errs) == 0 {
		t.Error("resultado inválido debería fallar")
	}
	var marked testadv.Schedule
	for _, s := range a.GetTestSchedules(0).Schedules {
		if s.ID == "nightly" {
			marked = s
		}
	}
	if marked.LastResult != "passed" || marked.LastRun.IsZero() {
		t.Errorf("run no registrado: %+v", marked)
	}
	if !marked.NextRun.After(marked.LastRun) {
		t.Errorf("nextRun %v debe ser posterior a lastRun %v", marked.NextRun, marked.LastRun)
	}

	// Baja.
	if errs := a.DeleteTestSchedule(0, "nightly"); len(errs) != 0 {
		t.Fatalf("borrar: %v", errs)
	}
	if errs := a.DeleteTestSchedule(0, "nightly"); len(errs) == 0 {
		t.Error("borrar dos veces debería fallar")
	}
	for _, s := range a.GetTestSchedules(0).Schedules {
		if s.ID == "nightly" {
			t.Error("el schedule borrado sigue presente")
		}
	}
}

func TestTestadvRunsAlimentanMatrizYAnalitica(t *testing.T) {
	a := newTestadvApp(t, t.TempDir())
	base := time.Now().Add(-2 * time.Hour)

	recs := []testadv.RunRecord{
		{StartedAt: base, DurationMS: 1000, Passed: 5, Failed: 1, Browser: "Chromium", Env: "DEV", FailedTests: []string{"login"}},
		{StartedAt: base.Add(time.Hour), DurationMS: 500, Passed: 6, Failed: 0, Browser: "chromium", Env: "dev"},
		{StartedAt: base.Add(90 * time.Minute), DurationMS: 500, Passed: 6, Failed: 0, Browser: "firefox", Env: "prod"},
	}
	for _, rec := range recs {
		if errs := a.AppendTestRun(0, rec); len(errs) != 0 {
			t.Fatalf("append: %v", errs)
		}
	}

	cells := a.GetTestMatrix(0)
	// 3 navegadores por 3 entornos del proyecto.
	if len(cells) != 9 {
		t.Fatalf("cells = %d, want 9", len(cells))
	}
	byKey := map[string]testadv.MatrixCell{}
	for _, c := range cells {
		byKey[testadv.MatrixKey(c.Browser, c.Env)] = c
	}
	if got := byKey["chromium|dev"]; got.Status != "passed" || got.Runs != 2 {
		t.Errorf("chromium|dev = %+v (el último run pasa, se acumulan runs)", got)
	}
	if got := byKey["firefox|prod"]; got.Status != "passed" || got.Runs != 1 {
		t.Errorf("firefox|prod = %+v", got)
	}
	if got := byKey["webkit|dev"]; got.Status != "pending" {
		t.Errorf("webkit|dev = %+v", got)
	}

	analytics := a.GetTestAnalytics(0, 6)
	if analytics.TotalRuns != 3 || analytics.TotalPassed != 17 || analytics.TotalFailed != 1 {
		t.Errorf("analytics = %+v", analytics)
	}
	if len(analytics.FlakyTests) != 1 || analytics.FlakyTests[0].Name != "login" {
		t.Errorf("flaky = %+v", analytics.FlakyTests)
	}

	if errs := a.AppendTestRun(0, testadv.RunRecord{Passed: -1}); len(errs) == 0 {
		t.Error("contadores negativos deberían fallar")
	}
}

func TestTestadvCoverageDesdeElProyecto(t *testing.T) {
	projectPath := t.TempDir()
	a := newTestadvApp(t, projectPath)

	if got := a.GetTestCoverage(0, 0); got.Available {
		t.Fatalf("sin artefactos no debe haber cobertura: %+v", got)
	}

	writeFile(t, filepath.Join(projectPath, "coverage", "coverage-summary.json"), `{
      "total":{"lines":{"total":100,"covered":60},"statements":{"total":100,"covered":60},
               "functions":{"total":10,"covered":10},"branches":{"total":10,"covered":10}},
      "/src/flojo.js":{"lines":{"total":10,"covered":1},"statements":{"total":10,"covered":1},
                       "functions":{"total":1,"covered":1},"branches":{"total":1,"covered":1}}
    }`)
	got := a.GetTestCoverage(0, 0)
	if !got.Available || got.Summary.Lines.Pct != 60 {
		t.Fatalf("coverage = %+v", got)
	}
	if got.Summary.Threshold != testadvDefaultThreshold || got.Summary.Pass {
		t.Errorf("debe aplicar el umbral por defecto y fallar: %+v", got.Summary)
	}
	if len(got.Worst) != 1 || got.Worst[0].Path != "/src/flojo.js" {
		t.Errorf("worst = %+v", got.Worst)
	}

	loose := a.GetTestCoverage(0, 50)
	if !loose.Summary.Pass || loose.Summary.Threshold != 50 {
		t.Errorf("umbral 50 debería pasar: %+v", loose.Summary)
	}
}

func TestTestadvVisualYBaseline(t *testing.T) {
	projectPath := t.TempDir()
	a := newTestadvApp(t, projectPath)
	visualRoot := filepath.Join(projectPath, "test-results", "visual")
	writeBytes(t, filepath.Join(visualRoot, "baseline", "home.png"), pngFixture(t, 6, 6, 200, 200, 200))
	writeBytes(t, filepath.Join(visualRoot, "current", "home.png"), pngFixture(t, 6, 6, 200, 200, 200))
	writeBytes(t, filepath.Join(visualRoot, "current", "nueva.png"), pngFixture(t, 6, 6, 10, 10, 10))

	report := a.GetVisualReport(0, 0, 0.5)
	if !report.Available || report.Total != 2 {
		t.Fatalf("report = %+v", report)
	}
	if report.Same != 1 || report.New != 1 {
		t.Errorf("conteos = same %d, new %d", report.Same, report.New)
	}
	if report.Shots != 2 {
		t.Errorf("shots = %d", report.Shots)
	}

	copied, err := a.PromoteVisualBaseline(0)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if copied != 2 {
		t.Errorf("copiados = %d, want 2", copied)
	}
	after := a.GetVisualReport(0, 0, 0.5)
	if after.New != 0 || after.Same != 2 {
		t.Errorf("tras promover todo debería ser same: %+v", after)
	}
}

func TestTestadvFixturesYResumen(t *testing.T) {
	projectPath := t.TempDir()
	a := newTestadvApp(t, projectPath)
	writeFile(t, filepath.Join(projectPath, "test-data", "seed.json"), `{"a":1}`)

	fixtures := a.GetTestFixtures(0)
	if len(fixtures) != 1 || fixtures[0].Dir != "test-data" || fixtures[0].Files != 1 {
		t.Fatalf("fixtures = %+v", fixtures)
	}

	summary := a.GetTestAdvSummary(0, 0)
	if summary.Project != "Mi App" || summary.ActiveEnv != "dev" {
		t.Errorf("resumen = %+v", summary)
	}
	if summary.Threshold != testadvDefaultThreshold {
		t.Errorf("threshold = %v", summary.Threshold)
	}
	if len(summary.Envs) != 3 || len(summary.Browsers) != 3 || len(summary.Matrix) != 9 {
		t.Errorf("en matriz = envs %v, browsers %v, cells %d", summary.Envs, summary.Browsers, len(summary.Matrix))
	}
	if summary.Analytics.TotalRuns != 0 || summary.Analytics.Trend == nil {
		t.Errorf("analytics vacía mal formada: %+v", summary.Analytics)
	}
	if summary.Coverage.Available {
		t.Errorf("coverage no debe estar disponible: %+v", summary.Coverage)
	}
	if len(summary.Fixtures) != 1 || len(summary.FixtureDirs) == 0 {
		t.Errorf("fixtures en resumen = %+v", summary.Fixtures)
	}
	if summary.MatrixState.Total != 9 || summary.MatrixState.Done != 0 {
		t.Errorf("matrixProgress = %+v", summary.MatrixState)
	}
}

func TestTestadvIndexFueraDeRango(t *testing.T) {
	a := newTestadvApp(t, t.TempDir())
	if got := a.GetTestCoverage(7, 0); got.Available {
		t.Errorf("coverage fuera de rango = %+v", got)
	}
	if got := a.GetTestSchedules(7); len(got.Schedules) != 0 {
		t.Errorf("schedules fuera de rango = %+v", got)
	}
	if got := a.GetTestMatrix(7); len(got) != 3 {
		t.Errorf("matriz fuera de rango debe seguir siendo la default: %+v", got)
	}
	if errs := a.AppendTestRun(7, testadv.RunRecord{}); len(errs) == 0 {
		t.Error("append fuera de rango debería fallar")
	}
	if errs := a.SaveTestSchedule(7, testadv.Schedule{Name: "x", Spec: "daily 01:00"}); len(errs) == 0 {
		t.Error("save fuera de rango debería fallar")
	}
	if _, err := a.PromoteVisualBaseline(7); err == nil {
		t.Error("promote fuera de rango debería fallar")
	}
}
