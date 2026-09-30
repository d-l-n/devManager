package testadv

import (
	"reflect"
	"testing"
	"time"
)

func TestNormalizeBrowsers(t *testing.T) {
	if got := NormalizeBrowsers(nil); !reflect.DeepEqual(got, DefaultBrowsers()) {
		t.Errorf("sin entrada debe usar defaults: %+v", got)
	}
	got := NormalizeBrowsers([]string{"Chrome", "edge", "chrome", " firefox ", "", "safari"})
	want := []string{"chromium", "firefox", "webkit"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	custom := NormalizeBrowsers([]string{"chromium", "android-chrome"})
	if len(custom) != 2 || custom[1] != "android-chrome" {
		t.Errorf("proyectos custom deben pasar tal cual: %+v", custom)
	}
}

func TestNormalizeEnvs(t *testing.T) {
	if got := NormalizeEnvs(nil); !reflect.DeepEqual(got, []string{"dev"}) {
		t.Errorf("sin envs debe usar dev: %+v", got)
	}
	got := NormalizeEnvs([]string{"prod", "dev", "PROD", " "})
	want := []string{"dev", "prod"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestBuildMatrix(t *testing.T) {
	cells := BuildMatrix([]string{"chrome", "firefox"}, []string{"prod", "dev"})
	if len(cells) != 4 {
		t.Fatalf("cells = %d, want 4", len(cells))
	}
	// browser-major, envs ordenados alfabéticamente.
	if cells[0].Browser != "chromium" || cells[0].Env != "dev" || cells[0].Status != "pending" {
		t.Errorf("primera celda = %+v", cells[0])
	}
	if cells[1].Env != "prod" || cells[2].Browser != "firefox" {
		t.Errorf("orden = %+v", cells)
	}
	empty := BuildMatrix(nil, nil)
	if len(empty) != 3 {
		t.Errorf("matriz por defecto = %+v", empty)
	}
}

func TestApplyResultsYProgress(t *testing.T) {
	cells := BuildMatrix([]string{"chromium", "firefox"}, []string{"dev", "prod"})
	last := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	results := map[string]MatrixCell{
		MatrixKey("chromium", "dev"):  {Status: "passed", Runs: 3, LastRun: last},
		MatrixKey("chromium", "prod"): {Status: "failed", Runs: 1},
	}
	cells = ApplyResults(cells, results)
	pass, total, rate := MatrixProgress(cells)
	if pass != 2 || total != 4 {
		t.Errorf("progress = %d/%d (rate %v)", pass, total, rate)
	}
	if rate != 50 {
		t.Errorf("pass rate = %v, want 50", rate)
	}
	if cells[0].Runs != 3 || !cells[0].LastRun.Equal(last) {
		t.Errorf("resultado no aplicado: %+v", cells[0])
	}
	if cells[2].Status != "pending" {
		t.Errorf("celda sin resultado debe seguir pending: %+v", cells[2])
	}
	if MatrixKey("Chromium", "DEV") != "chromium|dev" {
		t.Errorf("key = %q", MatrixKey("Chromium", "DEV"))
	}
}

func TestMatrixProgressSinResultados(t *testing.T) {
	_, total, rate := MatrixProgress(BuildMatrix(nil, nil))
	if total != 3 || rate != 0 {
		t.Errorf("progress inicial = %d celdas, rate %v", total, rate)
	}
}
