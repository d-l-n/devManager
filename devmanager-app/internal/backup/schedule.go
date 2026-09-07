package backup

import (
	"time"
)

// Frecuencias soportadas por backup_frequency en settings.json.
const (
	FreqOff    = "off"
	FreqHourly = "hourly"
	Freq6h     = "6h"
	FreqDaily  = "daily"
	FreqWeekly = "weekly"
)

// ValidFrequency reporta si f es una frecuencia aceptada.
func ValidFrequency(f string) bool {
	switch f {
	case FreqOff, FreqHourly, Freq6h, FreqDaily, FreqWeekly:
		return true
	}
	return false
}

// Interval devuelve el intervalo de f; ok=false para "off" o inválido.
func Interval(f string) (time.Duration, bool) {
	switch f {
	case FreqHourly:
		return time.Hour, true
	case Freq6h:
		return 6 * time.Hour, true
	case FreqDaily:
		return 24 * time.Hour, true
	case FreqWeekly:
		return 7 * 24 * time.Hour, true
	}
	return 0, false
}

// IsDue reporta si toca backup: last (mtime del backup más nuevo; cero si no
// hay) es más viejo que el intervalo de frequency.
func IsDue(frequency string, last time.Time, now time.Time) bool {
	interval, ok := Interval(frequency)
	if !ok {
		return false // "off" o desconocida: nunca automático
	}
	if last.IsZero() {
		return true // sin backups previos + frecuencia activa → toca
	}
	return now.Sub(last) >= interval
}
