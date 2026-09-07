package backup

import (
	"testing"
	"time"
)

func TestInterval(t *testing.T) {
	cases := []struct {
		freq  string
		want  time.Duration
		valid bool
	}{
		{FreqOff, 0, false},
		{FreqHourly, time.Hour, true},
		{Freq6h, 6 * time.Hour, true},
		{FreqDaily, 24 * time.Hour, true},
		{FreqWeekly, 7 * 24 * time.Hour, true},
		{"bogus", 0, false},
	}
	for _, c := range cases {
		got, ok := Interval(c.freq)
		if ok != c.valid || got != c.want {
			t.Errorf("Interval(%q) = (%v, %v), want (%v, %v)", c.freq, got, ok, c.want, c.valid)
		}
	}
}

func TestIsDue(t *testing.T) {
	now := testNow

	if !IsDue(FreqDaily, time.Time{}, now) {
		t.Error("sin backups previos + frecuencia activa → due")
	}
	if IsDue(FreqOff, time.Time{}, now) {
		t.Error("off nunca es due")
	}
	if IsDue("bogus", time.Time{}, now) {
		t.Error("frecuencia inválida nunca es due")
	}
	if IsDue(FreqDaily, now.Add(-time.Hour), now) {
		t.Error("hace 1h no toca daily")
	}
	if !IsDue(FreqDaily, now.Add(-25*time.Hour), now) {
		t.Error("hace 25h toca daily")
	}
	if !IsDue(FreqHourly, now.Add(-time.Hour), now) {
		t.Error("hace exactamente 1h toca hourly")
	}
}
