// Package detection porta app/utils/detection.py (Fase 1: solo logs).
package detection

import (
	"regexp"
	"strconv"
)

var portURLRe = regexp.MustCompile(`(?i)https?://(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\]|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})`)

// ExtractPortFromLog replica extract_port_from_log.
// Devuelve 0 si no hay match o el puerto está fuera de rango.
func ExtractPortFromLog(line string) int {
	if line == "" {
		return 0
	}
	m := portURLRe.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	port, err := strconv.Atoi(m[1])
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}
