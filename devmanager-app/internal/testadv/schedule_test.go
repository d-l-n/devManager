package testadv

import (
	"testing"
	"time"
)

// miércoles 2026-09-30 10:00 UTC (referencia fija para Next()).
var wed = time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)

func TestParseSpecEvery(t *testing.T) {
	spec, err := ParseSpec("every 15m")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if spec.Kind != KindEvery || spec.Every != 15*time.Minute {
		t.Errorf("spec = %+v", spec)
	}
	if got := spec.Next(wed); !got.Equal(wed.Add(15 * time.Minute)) {
		t.Errorf("next = %v", got)
	}
	if spec.String() != "every 15m0s" {
		t.Errorf("string = %q", spec.String())
	}

	if _, err := ParseSpec("every 1s"); err == nil {
		t.Error("intervalo < 1m debería rechazarse")
	}
	if _, err := ParseSpec("every abc"); err == nil {
		t.Error("intervalo inválido debería fallar")
	}
}

func TestParseSpecDaily(t *testing.T) {
	spec, err := ParseSpec("daily 03:30")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	next := spec.Next(wed)
	want := time.Date(2026, 10, 1, 3, 30, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Errorf("next = %v, want %v", next, want)
	}

	same, err := ParseSpec("daily 12:00")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got := same.Next(wed); !got.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("hora futura del mismo día = %v", got)
	}

	for _, bad := range []string{"daily", "daily 24:00", "daily 12:60", "daily 1200"} {
		if _, err := ParseSpec(bad); err == nil {
			t.Errorf("%q debería fallar", bad)
		}
	}
}

func TestParseSpecWeekly(t *testing.T) {
	spec, err := ParseSpec("weekly mon 09:00")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	next := spec.Next(wed)
	want := time.Date(2026, 10, 5, 9, 0, 0, 0, time.UTC)
	if !next.Equal(want) {
		t.Fatalf("next = %v, want %v", next, want)
	}
	if next.Weekday() != time.Monday {
		t.Errorf("weekday = %v", next.Weekday())
	}

	// Mismo día con hora futura → hoy mismo.
	same, err := ParseSpec("weekly wed 12:00")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got := same.Next(wed); !got.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) {
		t.Errorf("next = %v", got)
	}

	// Mismo día con hora pasada → miércoles siguiente.
	past, err := ParseSpec("weekly wed 09:00")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if got := past.Next(wed); !got.Equal(time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("next = %v", got)
	}

	// Alias en español + varios días (se normalizan y ordenan).
	multi, err := ParseSpec("weekly vie,lun 08:15")
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if len(multi.Weekdays) != 2 || multi.Weekdays[0] != time.Monday || multi.Weekdays[1] != time.Friday {
		t.Errorf("weekdays = %+v", multi.Weekdays)
	}
	if multi.String() != "weekly mon,fri 08:15" {
		t.Errorf("string = %q", multi.String())
	}

	for _, bad := range []string{"weekly", "weekly funday 09:00", "weekly mon", "semanal mon 09:00"} {
		if _, err := ParseSpec(bad); err == nil {
			t.Errorf("%q debería fallar", bad)
		}
	}
}

func TestScheduleValidateYRecompute(t *testing.T) {
	s := Schedule{Name: "nightly", Spec: "daily 03:00", Enabled: true}
	if errs := s.Validate(); len(errs) != 0 {
		t.Fatalf("schedule válido: %+v", errs)
	}
	s.Recompute(wed)
	if !s.NextRun.Equal(time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)) {
		t.Errorf("nextRun = %v", s.NextRun)
	}

	// LastRun posterior a now no debe reprogramar en el pasado.
	s.LastRun = time.Date(2026, 9, 30, 23, 0, 0, 0, time.UTC)
	s.Recompute(wed)
	if !s.NextRun.After(s.LastRun) {
		t.Errorf("nextRun %v debe ser posterior a lastRun %v", s.NextRun, s.LastRun)
	}

	s.Enabled = false
	s.Recompute(wed)
	if !s.NextRun.IsZero() {
		t.Errorf("deshabilitado no debe tener nextRun: %v", s.NextRun)
	}

	bad := Schedule{Spec: "cada rato"}
	if errs := bad.Validate(); len(errs) < 2 {
		t.Errorf("esperaba error de nombre y de spec: %+v", errs)
	}

	invalid := Schedule{Name: "x", Spec: "every 5s", Enabled: true}
	invalid.Recompute(wed)
	if !invalid.NextRun.IsZero() {
		t.Errorf("spec inválida no debe programar: %v", invalid.NextRun)
	}
}

func TestDue(t *testing.T) {
	now := wed
	overdue := Schedule{ID: "1", Name: "lento", Spec: "every 1h", Enabled: true,
		LastRun: now.Add(-2 * time.Hour)}
	overdue.Recompute(now.Add(-2 * time.Hour)) // nextRun = lastRun+1h → vencido

	future := Schedule{ID: "2", Name: "futuro", Spec: "daily 23:00", Enabled: true}
	future.Recompute(now)

	disabled := Schedule{ID: "3", Name: "off", Spec: "every 5m", Enabled: false}

	due := Due([]Schedule{overdue, future, disabled}, now)
	if len(due) != 1 || due[0].ID != "1" {
		t.Fatalf("due = %+v", due)
	}

	// Sin nextRun calculado se deriva del spec (nunca se queda sin programar).
	pending := Schedule{ID: "4", Name: "sin next", Spec: "every 1m", Enabled: true}
	if got := Due([]Schedule{pending}, now); len(got) != 0 {
		t.Errorf("no debe estar vencido antes de la primera ejecución: %+v", got)
	}
	first := Schedule{ID: "5", Name: "primera", Spec: "every 1m", Enabled: true}
	spec, _ := ParseSpec("every 1m")
	first.NextRun = spec.Next(now.Add(-10 * time.Minute))
	if got := Due([]Schedule{first}, now); len(got) != 1 {
		t.Errorf("nextRun pasado debería estar vencido: %+v", got)
	}
}

func TestSortByNext(t *testing.T) {
	enabled1 := Schedule{Name: "b", Enabled: true, NextRun: wed.Add(2 * time.Hour)}
	enabled2 := Schedule{Name: "a", Enabled: true, NextRun: wed.Add(1 * time.Hour)}
	off1 := Schedule{Name: "off-b", Enabled: false}
	off2 := Schedule{Name: "off-a", Enabled: false}
	list := []Schedule{off2, enabled1, off1, enabled2}
	SortByNext(list)
	want := []string{"a", "b", "off-a", "off-b"}
	for i, w := range want {
		if list[i].Name != w {
			t.Fatalf("orden = %+v", list)
		}
	}
}
