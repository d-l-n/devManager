package testadv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PerfMetrics son las métricas agregadas de un run de carga (k6, artillery).
type PerfMetrics struct {
	Source          string  `json:"source"` // k6 | artillery
	Path            string  `json:"path"`
	AvgMS           float64 `json:"avgMs"`
	P50MS           float64 `json:"p50Ms"`
	P90MS           float64 `json:"p90Ms"`
	P95MS           float64 `json:"p95Ms"`
	P99MS           float64 `json:"p99Ms"`
	MaxMS           float64 `json:"maxMs"`
	RPS             float64 `json:"rps"`
	Requests        int     `json:"requests"`
	ErrorRate       float64 `json:"errorRate"` // porcentaje 0..100
	DurationSeconds float64 `json:"durationSeconds"`
	VUs             int     `json:"vus"`
	ChecksPassRate  float64 `json:"checksPassRate"`
}

// Thresholds son los límites opcionales del usuario. Un límite a 0 se ignora.
type Thresholds struct {
	P95MaxMS        float64 `json:"p95MaxMs"`
	P99MaxMS        float64 `json:"p99MaxMs"`
	ErrorRateMaxPct float64 `json:"errorRateMaxPct"`
	RPSMin          float64 `json:"rpsMin"`
}

// ThresholdResult es el veredicto de un límite evaluado.
type ThresholdResult struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Limit float64 `json:"limit"`
	Pass  bool    `json:"pass"`
}

// Evaluate compara las métricas contra los límites activos. Un límite sin
// valor medido (ej. RPS con artillery sin rates) falla de forma explícita.
func (m PerfMetrics) Evaluate(t Thresholds) []ThresholdResult {
	out := []ThresholdResult{}
	if t.P95MaxMS > 0 {
		out = append(out, ThresholdResult{Name: "p95", Value: m.P95MS, Limit: t.P95MaxMS, Pass: m.P95MS > 0 && m.P95MS <= t.P95MaxMS})
	}
	if t.P99MaxMS > 0 {
		out = append(out, ThresholdResult{Name: "p99", Value: m.P99MS, Limit: t.P99MaxMS, Pass: m.P99MS > 0 && m.P99MS <= t.P99MaxMS})
	}
	if t.ErrorRateMaxPct > 0 {
		out = append(out, ThresholdResult{Name: "error_rate", Value: m.ErrorRate, Limit: t.ErrorRateMaxPct, Pass: m.ErrorRate <= t.ErrorRateMaxPct})
	}
	if t.RPSMin > 0 {
		out = append(out, ThresholdResult{Name: "rps", Value: m.RPS, Limit: t.RPSMin, Pass: m.RPS >= t.RPSMin})
	}
	return out
}

// k6Summary es el shape de `k6 run --summary-export out.json`.
type k6Summary struct {
	Metrics map[string]map[string]json.RawMessage `json:"metrics"`
	State   struct {
		TestRunDurationMS float64 `json:"testRunDurationMs"`
	} `json:"state"`
}

// ParseK6Summary parsea un summary-export de k6. Los percentiles llegan como
// claves dinámicas ("p(95)"), así que se buscan por nombre.
func ParseK6Summary(data []byte) (PerfMetrics, error) {
	var doc k6Summary
	if err := json.Unmarshal(data, &doc); err != nil {
		return PerfMetrics{}, fmt.Errorf("k6 summary: %w", err)
	}
	if len(doc.Metrics) == 0 {
		return PerfMetrics{}, fmt.Errorf("k6 summary sin bloque metrics")
	}
	out := PerfMetrics{Source: "k6"}
	dur := doc.Metrics["http_req_duration"]
	out.AvgMS = rawFloat(dur, "avg")
	out.P50MS = rawFloat(dur, "med", "p(50)")
	out.P90MS = rawFloat(dur, "p(90)")
	out.P95MS = rawFloat(dur, "p(95)")
	out.P99MS = rawFloat(dur, "p(99)")
	out.MaxMS = rawFloat(dur, "max")

	reqs := doc.Metrics["http_reqs"]
	out.RPS = rawFloat(reqs, "rate")
	if v, ok := rawNumber(reqs, "count"); ok {
		out.Requests = int(v)
	}
	if failed, ok := rawNumber(doc.Metrics["http_req_failed"], "rate"); ok {
		out.ErrorRate = round2(failed * 100)
	}
	out.DurationSeconds = round2(doc.State.TestRunDurationMS / 1000)
	if v, ok := rawNumber(doc.Metrics["vus_max"], "max", "value"); ok {
		out.VUs = int(v)
	}
	if checks := doc.Metrics["checks"]; checks != nil {
		if rate, ok := rawNumber(checks, "rate"); ok {
			out.ChecksPassRate = round2(rate * 100)
		}
	}
	return out, nil
}

// artilleryReport es el shape del reporte JSON de artillery.
type artilleryReport struct {
	Aggregate struct {
		Latency struct {
			Min    float64 `json:"min"`
			Max    float64 `json:"max"`
			Median float64 `json:"median"`
			P95    float64 `json:"p95"`
			P99    float64 `json:"p99"`
		} `json:"latency"`
		RPS struct {
			Mean  float64 `json:"mean"`
			Count int     `json:"count"`
		} `json:"rps"`
		Codes              map[string]int `json:"codes"`
		Errors             map[string]int `json:"errors"`
		ScenariosCompleted int            `json:"scenariosCompleted"`
		ScenariosCreated   int            `json:"scenariosCreated"`
	} `json:"aggregate"`
}

// ParseArtillery parsea el JSON de `artillery run -o report.json`. El error
// rate se deriva de los códigos HTTP >= 400 más los errores de red.
func ParseArtillery(data []byte) (PerfMetrics, error) {
	var doc artilleryReport
	if err := json.Unmarshal(data, &doc); err != nil {
		return PerfMetrics{}, fmt.Errorf("artillery report: %w", err)
	}
	agg := doc.Aggregate
	if agg.ScenariosCreated == 0 && agg.ScenariosCompleted == 0 && agg.RPS.Count == 0 && len(agg.Codes) == 0 {
		return PerfMetrics{}, fmt.Errorf("artillery report sin aggregate")
	}
	out := PerfMetrics{
		Source:   "artillery",
		AvgMS:    agg.Latency.Median,
		P50MS:    agg.Latency.Median,
		P95MS:    agg.Latency.P95,
		P99MS:    agg.Latency.P99,
		MaxMS:    agg.Latency.Max,
		RPS:      agg.RPS.Mean,
		Requests: agg.RPS.Count,
	}
	if out.Requests == 0 {
		out.Requests = agg.ScenariosCompleted
	}
	total, bad := 0, 0
	for code, n := range agg.Codes {
		total += n
		if c, err := parseInt(code); err == nil && c >= 400 {
			bad += n
		}
	}
	for _, n := range agg.Errors {
		bad += n
		total += n
	}
	if total > 0 {
		out.ErrorRate = round2(float64(bad) / float64(total) * 100)
	}
	return out, nil
}

// perfCandidates son los artefactos de performance soportados.
var perfCandidates = []string{
	filepath.Join("test-results", "k6-summary.json"),
	"k6-summary.json",
	filepath.Join("perf", "k6-summary.json"),
	"k6.json",
	filepath.Join("test-results", "artillery-report.json"),
	"artillery-report.json",
	filepath.Join("perf", "artillery-report.json"),
	"artillery.json",
}

// DetectPerfFile devuelve el primer artefacto de performance existente.
func DetectPerfFile(projectPath string) string {
	if strings.TrimSpace(projectPath) == "" {
		return ""
	}
	for _, rel := range perfCandidates {
		p := filepath.Join(projectPath, rel)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// LoadPerf detecta y parsea las métricas de performance del proyecto.
func LoadPerf(projectPath string) (PerfMetrics, bool, error) {
	path := DetectPerfFile(projectPath)
	if path == "" {
		return PerfMetrics{}, false, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return PerfMetrics{}, false, fmt.Errorf("leer performance: %w", err)
	}
	var metrics PerfMetrics
	switch {
	case strings.Contains(strings.ToLower(filepath.Base(path)), "artillery"):
		metrics, err = ParseArtillery(data)
	default:
		metrics, err = ParseK6Summary(data)
	}
	if err != nil {
		return PerfMetrics{}, false, err
	}
	metrics.Path = path
	return metrics, true, nil
}

// ---- helpers ----

func rawNumber(metric map[string]json.RawMessage, keys ...string) (float64, bool) {
	for _, k := range keys {
		raw, ok := metric[k]
		if !ok {
			continue
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func rawFloat(metric map[string]json.RawMessage, keys ...string) float64 {
	if metric == nil {
		return 0
	}
	f, _ := rawNumber(metric, keys...)
	return round2(f)
}

func parseInt(s string) (int, error) {
	n := 0
	seen := false
	for _, r := range s {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
		seen = true
	}
	if !seen {
		return 0, fmt.Errorf("no numérico: %q", s)
	}
	return n, nil
}

// PerfThresholdResult ordena los veredictos de umbral para la UI.
func PerfThresholdResult(results []ThresholdResult) []ThresholdResult {
	out := make([]ThresholdResult, len(results))
	copy(out, results)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
