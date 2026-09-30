package testadv

import (
	"testing"
	"time"
)

var analyticsBase = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

func analyticsRuns() []RunRecord {
	return []RunRecord{
		{StartedAt: analyticsBase, DurationMS: 1000, Passed: 8, Failed: 2, FailedTests: []string{"A", "B"}},
		{StartedAt: analyticsBase.Add(30 * time.Minute), DurationMS: 2000, Passed: 9, Failed: 1, FailedTests: []string{"A"}},
		{StartedAt: analyticsBase.Add(90 * time.Minute), DurationMS: 3000, Passed: 10, Failed: 0, Skipped: 1},
	}
}

func TestAnalyzeVacio(t *testing.T) {
	got := Analyze(nil, 0)
	if got.TotalRuns != 0 || got.Trend == nil || got.FlakyTests == nil {
		t.Errorf("analytics vacío debe tener slices no-nil: %+v", got)
	}
	if got.LastFailure != nil {
		t.Error("sin runs no hay último fallo")
	}
}

func TestAnalyzeAgregaTotales(t *testing.T) {
	got := Analyze(analyticsRuns(), 2)
	if got.TotalRuns != 3 || got.TotalPassed != 27 || got.TotalFailed != 3 || got.TotalSkipped != 1 {
		t.Errorf("totales = %+v", got)
	}
	if got.PassRate != 90 {
		t.Errorf("pass rate = %v, want 90", got.PassRate)
	}
	if got.AvgDurationMS != 2000 {
		t.Errorf("avg duration = %d, want 2000", got.AvgDurationMS)
	}
	if got.StableRuns != 1 {
		t.Errorf("stable runs = %d, want 1", got.StableRuns)
	}
	if got.LastFailure == nil || !got.LastFailure.Equal(analyticsBase.Add(30*time.Minute)) {
		t.Errorf("last failure = %v", got.LastFailure)
	}
}

func TestAnalyzeDetectaFlaky(t *testing.T) {
	got := Analyze(analyticsRuns(), 2)
	if len(got.FlakyTests) != 2 {
		t.Fatalf("flaky = %+v, want 2 (A y B)", got.FlakyTests)
	}
	if got.FlakyTests[0].Name != "A" || got.FlakyTests[0].Failures != 2 || got.FlakyTests[0].Rate != 66.67 {
		t.Errorf("primer flaky = %+v", got.FlakyTests[0])
	}
	if got.FlakyTests[1].Name != "B" || got.FlakyTests[1].Rate != 33.33 {
		t.Errorf("segundo flaky = %+v", got.FlakyTests[1])
	}
}

func TestAnalyzeNoMarcaFlakyAlQueSiempreFalla(t *testing.T) {
	runs := []RunRecord{
		{StartedAt: analyticsBase, Passed: 1, Failed: 1, FailedTests: []string{"siempre"}},
		{StartedAt: analyticsBase.Add(time.Hour), Passed: 1, Failed: 1, FailedTests: []string{"siempre"}},
	}
	got := Analyze(runs, 1)
	if len(got.FlakyTests) != 0 {
		t.Errorf("un test que falla en todos los runs no es flaky: %+v", got.FlakyTests)
	}
	if got.PassRate != 50 {
		t.Errorf("pass rate = %v", got.PassRate)
	}
}

func TestAnalyzeTrend(t *testing.T) {
	got := Analyze(analyticsRuns(), 2)
	if len(got.Trend) != 2 {
		t.Fatalf("trend = %+v, want 2 buckets con datos", got.Trend)
	}
	if got.Trend[0].Runs != 2 || got.Trend[0].PassRate != 85 {
		t.Errorf("primer bucket = %+v", got.Trend[0])
	}
	if got.Trend[0].AvgDurationMS != 1500 {
		t.Errorf("avg duration primer bucket = %d", got.Trend[0].AvgDurationMS)
	}
	if got.Trend[1].Runs != 1 || got.Trend[1].PassRate != 100 {
		t.Errorf("segundo bucket = %+v", got.Trend[1])
	}
}

func TestAnalyzeUnSoloInstante(t *testing.T) {
	runs := []RunRecord{{StartedAt: analyticsBase, Passed: 1, Failed: 0}}
	got := Analyze(runs, 12)
	if len(got.Trend) != 1 || got.Trend[0].Runs != 1 {
		t.Errorf("trend de un solo run = %+v", got.Trend)
	}
}

func TestAnalyzeOrdenaPorFechaNoPorEntrada(t *testing.T) {
	runs := []RunRecord{
		{StartedAt: analyticsBase.Add(time.Hour), Passed: 1, Failed: 1, FailedTests: []string{"X"}},
		{StartedAt: analyticsBase, Passed: 1, Failed: 0},
	}
	got := Analyze(runs, 1)
	if got.LastFailure == nil || !got.LastFailure.Equal(analyticsBase.Add(time.Hour)) {
		t.Errorf("debe ordenar por fecha: %v", got.LastFailure)
	}
}
