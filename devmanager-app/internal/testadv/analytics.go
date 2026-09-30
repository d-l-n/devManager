package testadv

import (
	"sort"
	"time"
)

// RunRecord es una ejecución de tests registrada (histórico del proyecto).
type RunRecord struct {
	StartedAt   time.Time `json:"startedAt"`
	DurationMS  int64     `json:"durationMs"`
	Passed      int       `json:"passed"`
	Failed      int       `json:"failed"`
	Skipped     int       `json:"skipped"`
	Browser     string    `json:"browser,omitempty"`
	Env         string    `json:"env,omitempty"`
	Suite       string    `json:"suite,omitempty"`
	FailedTests []string  `json:"failedTests,omitempty"`
}

// TrendPoint es un punto agregado del histórico.
type TrendPoint struct {
	Bucket        time.Time `json:"bucket"`
	Runs          int       `json:"runs"`
	PassRate      float64   `json:"passRate"`
	AvgDurationMS int64     `json:"avgDurationMs"`
}

// FlakyTest es un test que a veces pasa y a veces falla.
type FlakyTest struct {
	Name     string  `json:"name"`
	Failures int     `json:"failures"`
	Runs     int     `json:"runs"`
	Rate     float64 `json:"rate"` // % de runs en que falló
}

// Analytics agrega el histórico de runs.
type Analytics struct {
	TotalRuns     int          `json:"totalRuns"`
	TotalPassed   int          `json:"totalPassed"`
	TotalFailed   int          `json:"totalFailed"`
	TotalSkipped  int          `json:"totalSkipped"`
	PassRate      float64      `json:"passRate"`
	AvgDurationMS int64        `json:"avgDurationMs"`
	StableRuns    int          `json:"stableRuns"`
	Trend         []TrendPoint `json:"trend"`
	FlakyTests    []FlakyTest  `json:"flakyTests"`
	LastFailure   *time.Time   `json:"lastFailure,omitempty"`
}

// Analyze agrega runs en buckets de tiempo equiespaciados. Detección de flaky:
// un test que falló en algunos runs y no en otros (se asume que la suite es la
// misma en todos los runs registrados).
func Analyze(runs []RunRecord, buckets int) Analytics {
	out := Analytics{Trend: []TrendPoint{}, FlakyTests: []FlakyTest{}}
	if len(runs) == 0 {
		return out
	}
	if buckets <= 0 {
		buckets = 12
	}
	sorted := make([]RunRecord, len(runs))
	copy(sorted, runs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].StartedAt.Before(sorted[j].StartedAt) })

	out.TotalRuns = len(sorted)
	totalDuration := int64(0)
	failuresByTest := map[string]int{}
	for _, r := range sorted {
		out.TotalPassed += r.Passed
		out.TotalFailed += r.Failed
		out.TotalSkipped += r.Skipped
		totalDuration += r.DurationMS
		if r.Failed == 0 {
			out.StableRuns++
		} else {
			t := r.StartedAt
			if out.LastFailure == nil || t.After(*out.LastFailure) {
				out.LastFailure = &t
			}
		}
		for _, name := range r.FailedTests {
			failuresByTest[name]++
		}
	}
	executed := out.TotalPassed + out.TotalFailed
	if executed > 0 {
		out.PassRate = round2(float64(out.TotalPassed) / float64(executed) * 100)
	} else {
		out.PassRate = 100
	}
	out.AvgDurationMS = totalDuration / int64(len(sorted))

	for name, failures := range failuresByTest {
		if failures > 0 && failures < out.TotalRuns {
			out.FlakyTests = append(out.FlakyTests, FlakyTest{
				Name:     name,
				Failures: failures,
				Runs:     out.TotalRuns,
				Rate:     round2(float64(failures) / float64(out.TotalRuns) * 100),
			})
		}
	}
	sort.Slice(out.FlakyTests, func(i, j int) bool {
		if out.FlakyTests[i].Rate != out.FlakyTests[j].Rate {
			return out.FlakyTests[i].Rate > out.FlakyTests[j].Rate
		}
		return out.FlakyTests[i].Name < out.FlakyTests[j].Name
	})
	out.Trend = buildTrend(sorted, buckets)
	return out
}

// buildTrend parte el histórico en buckets equiespaciados entre el primer y el
// último run (si todos los runs comparten instante, un único bucket).
func buildTrend(runs []RunRecord, buckets int) []TrendPoint {
	start := runs[0].StartedAt
	end := runs[len(runs)-1].StartedAt
	span := end.Sub(start)
	if span <= 0 {
		return []TrendPoint{bucketPoint(runs, start)}
	}
	width := span / time.Duration(buckets)
	if width <= 0 {
		width = span
	}
	points := make([]TrendPoint, 0, buckets+1)
	for i := 0; i <= buckets; i++ {
		from := start.Add(time.Duration(i) * width)
		to := from.Add(width)
		last := i == buckets
		group := make([]RunRecord, 0, len(runs))
		for _, r := range runs {
			if r.StartedAt.Before(from) {
				continue
			}
			if last {
				if !r.StartedAt.After(end) {
					group = append(group, r)
				}
				continue
			}
			if r.StartedAt.Before(to) {
				group = append(group, r)
			}
		}
		if len(group) == 0 {
			continue
		}
		points = append(points, bucketPoint(group, from))
	}
	return points
}

func bucketPoint(group []RunRecord, bucket time.Time) TrendPoint {
	passed, executed := 0, 0
	total := int64(0)
	for _, r := range group {
		passed += r.Passed
		executed += r.Passed + r.Failed
		total += r.DurationMS
	}
	rate := 100.0
	if executed > 0 {
		rate = round2(float64(passed) / float64(executed) * 100)
	}
	return TrendPoint{
		Bucket:        bucket,
		Runs:          len(group),
		PassRate:      rate,
		AvgDurationMS: total / int64(len(group)),
	}
}
