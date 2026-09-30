package workflow

import (
	"strconv"
	"strings"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

// MatchesSchedule reporta si el workflow schedule-enabled vence en now.
// lastRun es el finishedAt del último run (zero si nunca corrió):
//   - @every <d>: vence si pasó >= d desde lastRun (o nunca corrió).
//   - @hourly/@daily/cron: vence si now matchea el patrón y pasaron >= 50s
//     desde lastRun (evita doble disparo dentro del mismo minuto con el
//     ticker de 60s).
func MatchesSchedule(w models.Workflow, now, lastRun time.Time) bool {
	if !w.Enabled || w.Trigger != models.WorkflowTriggerSchedule {
		return false
	}
	s := strings.TrimSpace(w.Schedule)
	if strings.HasPrefix(s, "@every ") {
		d, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(s, "@every ")))
		if err != nil || d <= 0 {
			return false
		}
		return lastRun.IsZero() || !now.Before(lastRun.Add(d))
	}
	if !lastRun.IsZero() && now.Sub(lastRun) < 50*time.Second {
		return false
	}
	switch s {
	case "@hourly":
		return now.Minute() == 0
	case "@daily":
		return now.Hour() == 0 && now.Minute() == 0
	}
	return matchCron(s, now)
}

func matchCron(s string, now time.Time) bool {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return false
	}
	vals := []int{now.Minute(), now.Hour(), now.Day(), int(now.Month()), int(now.Weekday())}
	for i, f := range fields {
		if !matchCronField(f, vals[i]) {
			return false
		}
	}
	return true
}

func matchCronField(f string, v int) bool {
	for _, part := range strings.Split(f, ",") {
		if matchCronAtom(strings.TrimSpace(part), v) {
			return true
		}
	}
	return false
}

func matchCronAtom(a string, v int) bool {
	if a == "*" {
		return true
	}
	if strings.HasPrefix(a, "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(a, "*/"))
		if err != nil || n <= 0 {
			return false
		}
		return v%n == 0
	}
	if strings.Contains(a, "-") {
		se := strings.SplitN(a, "-", 2)
		lo, err1 := strconv.Atoi(se[0])
		hi, err2 := strconv.Atoi(se[1])
		if err1 != nil || err2 != nil {
			return false
		}
		return v >= lo && v <= hi
	}
	n, err := strconv.Atoi(a)
	return err == nil && v == n
}
