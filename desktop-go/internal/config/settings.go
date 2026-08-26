package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Settings porta app/config/settings.py: preferencias de app persistidas
// en JSON (schema propio Go, keys snake_case — no es projects.json).
type Settings struct {
	Theme          string `json:"theme"`
	MonitorPolling bool   `json:"monitor_polling"`
	ToastsEnabled  bool   `json:"toasts_enabled"`
}

// Temas válidos; cualquier otro valor sanea a "dark" (paridad tolerante).
func validTheme(t string) bool {
	switch t {
	case "light", "dark", "oled":
		return true
	}
	return false
}

// DefaultSettings replica los defaults efectivos de Python:
// theme "dark", polling true, toasts true.
func DefaultSettings() Settings {
	return Settings{Theme: "dark", MonitorPolling: true, ToastsEnabled: true}
}

// LoadSettings lee el archivo; ausente/corrupto → defaults sin backup.
func LoadSettings(path string) Settings {
	s := DefaultSettings()
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return DefaultSettings()
	}
	if !validTheme(s.Theme) {
		s.Theme = "dark"
	}
	return s
}

// SaveSettings escribe MarshalIndent 2 espacios, creando el directorio padre.
func SaveSettings(path string, s Settings) error {
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, out, 0o644)
}
