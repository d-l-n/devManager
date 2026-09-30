package testadv

import (
	"os"
	"path/filepath"
	"testing"
)

const k6Sample = `{
  "metrics": {
    "http_req_duration": {"avg":12.5,"min":1,"med":10,"max":200,"p(90)":50,"p(95)":80.5,"p(99)":150},
    "http_reqs": {"count":1000,"rate":50.5},
    "http_req_failed": {"rate":0.012},
    "vus_max": {"max":20,"value":20},
    "checks": {"rate":0.98}
  },
  "state": {"testRunDurationMs":20000}
}`

func TestParseK6Summary(t *testing.T) {
	got, err := ParseK6Summary([]byte(k6Sample))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Source != "k6" {
		t.Errorf("source = %q", got.Source)
	}
	if got.AvgMS != 12.5 || got.P50MS != 10 || got.P90MS != 50 || got.P95MS != 80.5 || got.P99MS != 150 || got.MaxMS != 200 {
		t.Errorf("latencias = %+v", got)
	}
	if got.RPS != 50.5 || got.Requests != 1000 {
		t.Errorf("rps/requests = %v/%d", got.RPS, got.Requests)
	}
	if got.ErrorRate != 1.2 {
		t.Errorf("error rate = %v, want 1.2", got.ErrorRate)
	}
	if got.DurationSeconds != 20 || got.VUs != 20 || got.ChecksPassRate != 98 {
		t.Errorf("duration/vus/checks = %v/%d/%v", got.DurationSeconds, got.VUs, got.ChecksPassRate)
	}
}

func TestParseK6SummaryInvalid(t *testing.T) {
	if _, err := ParseK6Summary([]byte(`{"metrics":{}}`)); err == nil {
		t.Fatal("summary sin metrics debería fallar")
	}
	if _, err := ParseK6Summary([]byte("no json")); err == nil {
		t.Fatal("JSON inválido debería fallar")
	}
}

const artillerySample = `{
  "aggregate": {
    "latency": {"min":1,"max":900,"median":20,"p95":300,"p99":700},
    "rps": {"mean":30,"count":600},
    "codes": {"200":580,"404":10,"500":10},
    "errors": {"ECONNRESET":5},
    "scenariosCompleted":300,
    "scenariosCreated":300
  }
}`

func TestParseArtillery(t *testing.T) {
	got, err := ParseArtillery([]byte(artillerySample))
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got.Source != "artillery" || got.P50MS != 20 || got.P95MS != 300 || got.P99MS != 700 || got.MaxMS != 900 {
		t.Errorf("latencias = %+v", got)
	}
	if got.RPS != 30 || got.Requests != 600 {
		t.Errorf("rps/requests = %v/%d", got.RPS, got.Requests)
	}
	// 25 malos de 605 totales (600 códigos + 5 errores de red).
	if got.ErrorRate != 4.13 {
		t.Errorf("error rate = %v, want 4.13", got.ErrorRate)
	}
}

func TestParseArtilleryInvalid(t *testing.T) {
	if _, err := ParseArtillery([]byte(`{"aggregate":{}}`)); err == nil {
		t.Fatal("aggregate vacío debería fallar")
	}
}

func TestPerfEvaluate(t *testing.T) {
	metrics, _ := ParseK6Summary([]byte(k6Sample))

	checks := metrics.Evaluate(Thresholds{P95MaxMS: 81, ErrorRateMaxPct: 2, RPSMin: 50})
	if len(checks) != 3 {
		t.Fatalf("checks = %d, want 3", len(checks))
	}
	for _, c := range checks {
		if !c.Pass {
			t.Errorf("%s debería pasar: %+v", c.Name, c)
		}
	}

	failures := metrics.Evaluate(Thresholds{P95MaxMS: 80, P99MaxMS: 100, ErrorRateMaxPct: 1, RPSMin: 60})
	if len(failures) != 4 {
		t.Fatalf("checks = %d, want 4", len(failures))
	}
	for _, c := range failures {
		if c.Pass {
			t.Errorf("%s debería fallar: %+v", c.Name, c)
		}
	}

	// Un límite sin métrica medida falla de forma explícita (nunca "pass" mudo).
	empty := PerfMetrics{}.Evaluate(Thresholds{P95MaxMS: 1000})
	if len(empty) != 1 || empty[0].Pass {
		t.Errorf("p95 sin datos debe fallar: %+v", empty)
	}
}

func TestLoadPerfDetectaK6YArtillery(t *testing.T) {
	dir := t.TempDir()
	if _, ok, err := LoadPerf(dir); ok || err != nil {
		t.Fatalf("sin artefactos: ok=%v err=%v", ok, err)
	}
	k6Path := filepath.Join(dir, "test-results", "k6-summary.json")
	if err := os.MkdirAll(filepath.Dir(k6Path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(k6Path, []byte(k6Sample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadPerf(dir)
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Path != k6Path || got.Source != "k6" {
		t.Errorf("got %q/%q", got.Path, got.Source)
	}
}

func TestLoadPerfArtillery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "artillery-report.json")
	if err := os.WriteFile(path, []byte(artillerySample), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok, err := LoadPerf(dir)
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.Source != "artillery" {
		t.Errorf("source = %q (el nombre del fichero debe elegir el parser)", got.Source)
	}
	if DetectPerfFile("") != "" {
		t.Error("path vacío debería devolver vacío")
	}
}

func TestPerfThresholdResultOrdena(t *testing.T) {
	results := PerfThresholdResult([]ThresholdResult{
		{Name: "rps"}, {Name: "error_rate"}, {Name: "p95"},
	})
	want := []string{"error_rate", "p95", "rps"}
	for i, w := range want {
		if results[i].Name != w {
			t.Fatalf("orden = %+v", results)
		}
	}
}
