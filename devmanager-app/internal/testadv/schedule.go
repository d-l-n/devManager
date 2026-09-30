package testadv

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tipos de spec soportados por el scheduler.
const (
	KindEvery  = "every"
	KindDaily  = "daily"
	KindWeekly = "weekly"
)

// Spec es una programación parseada. Todo se evalúa en hora local.
type Spec struct {
	Kind     string
	Every    time.Duration
	Hour     int
	Minute   int
	Weekdays []time.Weekday
}

// ParseSpec acepta: "every 15m", "every 2h", "daily 03:30",
// "weekly mon,wed,fri 09:00". Nunca acepta intervalos < 1m (evita spamear
// suites de tests por un typo: "every 1s").
func ParseSpec(spec string) (Spec, error) {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(spec)))
	if len(fields) < 2 {
		return Spec{}, fmt.Errorf(`spec inválida %q (usa "every 15m", "daily 03:30" o "weekly mon,fri 09:00")`, spec)
	}
	switch fields[0] {
	case KindEvery:
		if len(fields) != 2 {
			return Spec{}, fmt.Errorf("spec inválida %q: 'every' espera un intervalo", spec)
		}
		d, err := time.ParseDuration(fields[1])
		if err != nil {
			return Spec{}, fmt.Errorf("intervalo inválido %q: %v", fields[1], err)
		}
		if d < time.Minute {
			return Spec{}, fmt.Errorf("intervalo mínimo 1m (recibido %s)", d)
		}
		return Spec{Kind: KindEvery, Every: d}, nil
	case KindDaily:
		if len(fields) != 2 {
			return Spec{}, fmt.Errorf("spec inválida %q: 'daily' espera HH:MM", spec)
		}
		h, m, err := parseClock(fields[1])
		if err != nil {
			return Spec{}, err
		}
		return Spec{Kind: KindDaily, Hour: h, Minute: m}, nil
	case KindWeekly:
		if len(fields) != 3 {
			return Spec{}, fmt.Errorf("spec inválida %q: 'weekly' espera DÍAS HH:MM", spec)
		}
		days, err := parseWeekdays(fields[1])
		if err != nil {
			return Spec{}, err
		}
		h, m, err := parseClock(fields[2])
		if err != nil {
			return Spec{}, err
		}
		return Spec{Kind: KindWeekly, Hour: h, Minute: m, Weekdays: days}, nil
	default:
		return Spec{}, fmt.Errorf(`spec inválida %q: prefijo desconocido (usa "every", "daily" o "weekly")`, spec)
	}
}

// Next devuelve la siguiente ejecución estrictamente posterior a after.
// Devuelve el zero time si el spec no puede programar nada.
func (s Spec) Next(after time.Time) time.Time {
	base := after.Truncate(time.Second)
	switch s.Kind {
	case KindEvery:
		if s.Every <= 0 {
			return time.Time{}
		}
		return base.Add(s.Every)
	case KindDaily:
		candidate := time.Date(base.Year(), base.Month(), base.Day(), s.Hour, s.Minute, 0, 0, base.Location())
		if !candidate.After(base) {
			candidate = candidate.AddDate(0, 0, 1)
		}
		return candidate
	case KindWeekly:
		if len(s.Weekdays) == 0 {
			return time.Time{}
		}
		for i := 0; i < 8; i++ {
			day := base.AddDate(0, 0, i)
			candidate := time.Date(day.Year(), day.Month(), day.Day(), s.Hour, s.Minute, 0, 0, base.Location())
			if !candidate.After(base) {
				continue
			}
			if containsWeekday(s.Weekdays, candidate.Weekday()) {
				return candidate
			}
		}
	}
	return time.Time{}
}

// String devuelve una forma canónica del spec (paridad con ParseSpec).
func (s Spec) String() string {
	switch s.Kind {
	case KindEvery:
		return "every " + s.Every.String()
	case KindDaily:
		return fmt.Sprintf("daily %02d:%02d", s.Hour, s.Minute)
	case KindWeekly:
		names := make([]string, 0, len(s.Weekdays))
		for _, d := range s.Weekdays {
			names = append(names, weekdayName(d))
		}
		return fmt.Sprintf("weekly %s %02d:%02d", strings.Join(names, ","), s.Hour, s.Minute)
	}
	return ""
}

// Schedule es una tarea de testing programada del proyecto.
type Schedule struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Spec       string    `json:"spec"`
	Command    string    `json:"command"`
	Enabled    bool      `json:"enabled"`
	LastRun    time.Time `json:"lastRun,omitempty"`
	LastResult string    `json:"lastResult,omitempty"` // passed | failed | error
	NextRun    time.Time `json:"nextRun,omitempty"`
}

// Validate devuelve los errores del schedule (mismo contrato que
// models.Project.Validate: lista vacía = válido).
func (s Schedule) Validate() []string {
	var errs []string
	if strings.TrimSpace(s.Name) == "" {
		errs = append(errs, "schedule name is required")
	}
	if strings.TrimSpace(s.Spec) == "" {
		errs = append(errs, "schedule spec is required")
	} else if _, err := ParseSpec(s.Spec); err != nil {
		errs = append(errs, err.Error())
	}
	return errs
}

// Recompute recalcula NextRun desde el spec (o lo limpia si está deshabilitado).
func (s *Schedule) Recompute(now time.Time) {
	if !s.Enabled {
		s.NextRun = time.Time{}
		return
	}
	spec, err := ParseSpec(s.Spec)
	if err != nil {
		s.NextRun = time.Time{}
		return
	}
	base := now
	if s.LastRun.After(base) {
		base = s.LastRun
	}
	s.NextRun = spec.Next(base)
}

// Due devuelve los schedules habilitados que ya deberían haber corrido.
func Due(list []Schedule, now time.Time) []Schedule {
	out := []Schedule{}
	for _, s := range list {
		if !s.Enabled {
			continue
		}
		next := s.NextRun
		if next.IsZero() {
			if spec, err := ParseSpec(s.Spec); err == nil {
				next = spec.Next(now)
			}
		}
		if !next.IsZero() && !next.After(now) {
			out = append(out, s)
		}
	}
	return out
}

// SortByNext ordena habilitados primero (por próxima ejecución) y deja los
// deshabilitados al final ordenados por nombre.
func SortByNext(list []Schedule) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		switch {
		case a.Enabled && !b.Enabled:
			return true
		case !a.Enabled && b.Enabled:
			return false
		case a.Enabled && b.Enabled:
			if a.NextRun.Equal(b.NextRun) {
				return a.Name < b.Name
			}
			return a.NextRun.Before(b.NextRun)
		default:
			return a.Name < b.Name
		}
	})
}

// ---- helpers ----

func parseClock(raw string) (int, int, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("hora inválida %q (usa HH:MM)", raw)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return 0, 0, fmt.Errorf("hora inválida %q (0-23)", raw)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return 0, 0, fmt.Errorf("minuto inválido %q (0-59)", raw)
	}
	return h, m, nil
}

var weekdayAliases = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday, "dom": time.Sunday,
	"mon": time.Monday, "monday": time.Monday, "lun": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday, "mar": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday, "mie": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday, "jue": time.Thursday,
	"fri": time.Friday, "friday": time.Friday, "vie": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday, "sab": time.Saturday,
}

func parseWeekdays(raw string) ([]time.Weekday, error) {
	out := []time.Weekday{}
	seen := map[time.Weekday]bool{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.TrimSpace(part)
		if key == "" {
			continue
		}
		day, ok := weekdayAliases[key]
		if !ok {
			return nil, fmt.Errorf("día inválido %q (usa mon,tue,wed,thu,fri,sat,sun)", part)
		}
		if seen[day] {
			continue
		}
		seen[day] = true
		out = append(out, day)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("weekly requiere al menos un día")
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

func containsWeekday(days []time.Weekday, day time.Weekday) bool {
	for _, d := range days {
		if d == day {
			return true
		}
	}
	return false
}

func weekdayName(d time.Weekday) string {
	switch d {
	case time.Sunday:
		return "sun"
	case time.Monday:
		return "mon"
	case time.Tuesday:
		return "tue"
	case time.Wednesday:
		return "wed"
	case time.Thursday:
		return "thu"
	case time.Friday:
		return "fri"
	default:
		return "sat"
	}
}
