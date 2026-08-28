// Package ports porta app/utils/ports.py.
package ports

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

// IsPortOpen replica is_port_open: dial TCP con timeout 500ms.
func IsPortOpen(host string, port int) bool {
	if host == "" || port <= 0 || port > 65535 {
		return false
	}
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, fmt.Sprint(port)), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

var (
	hasPortFlag = regexp.MustCompile(`(?:--port|-p)\s+\d+`)
	vitePrefix  = regexp.MustCompile(`^(?:npx\s+)?vite\b`)
)

// BuildServerCommand replica build_server_command línea a línea:
// añade el flag de puerto si el comando no lo trae ya.
func BuildServerCommand(command string, port int) string {
	if command == "" || port <= 0 {
		return command
	}
	if hasPortFlag.MatchString(command) {
		return command
	}
	trimmed := strings.TrimSpace(command)
	lower := strings.ToLower(trimmed)

	switch {
	case strings.HasPrefix(lower, "pnpm ") || strings.HasPrefix(lower, "pnpm.cmd "):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	case strings.HasPrefix(lower, "npm run ") || lower == "npm start":
		return fmt.Sprintf("%s -- --port %d", trimmed, port)
	case strings.HasPrefix(lower, "yarn ") || strings.HasPrefix(lower, "yarn.cmd "):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	case strings.HasPrefix(lower, "bun run ") || strings.HasPrefix(lower, "bun dev"):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	case vitePrefix.MatchString(lower):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	case strings.HasPrefix(lower, "next dev") || strings.HasPrefix(lower, "npx next dev"):
		return fmt.Sprintf("%s -p %d", trimmed, port)
	case strings.HasPrefix(lower, "astro dev") || strings.HasPrefix(lower, "npx astro dev"):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	case strings.HasPrefix(lower, "python manage.py runserver"):
		return fmt.Sprintf("%s %d", trimmed, port)
	case strings.HasPrefix(lower, "uvicorn "):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	case strings.HasPrefix(lower, "flask run"):
		return fmt.Sprintf("%s --port %d", trimmed, port)
	}
	return trimmed
}

// WaitForPort sustituye PortChecker (QTimer): sondea cada interval hasta
// que el puerto abre, expira el timeout o se cancela el contexto.
func WaitForPort(ctx context.Context, host string, port int, timeout, interval time.Duration) error {
	deadline := time.After(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			return fmt.Errorf("timeout waiting for port %d on %s after %v", port, host, timeout)
		case <-ticker.C:
			if IsPortOpen(host, port) {
				return nil
			}
		}
	}
}
