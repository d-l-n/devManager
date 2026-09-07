package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/d-l-n/devmanager/internal/backup"
)

// Settings porta app/config/settings.py: preferencias de app persistidas
// en JSON (schema propio Go, keys snake_case — no es projects.json).
type Settings struct {
	Theme             string            `json:"theme"`
	Style             string            `json:"style"`
	MonitorPolling    bool              `json:"monitor_polling"`
	ToastsEnabled     bool              `json:"toasts_enabled"`
	AccentOverrides   map[string]string `json:"accent_overrides"`
	AccentGlobal      bool              `json:"accent_global"`
	AccentGlobalColor string            `json:"accent_global_color"`
	// Backup (Issue #71): frecuencia automática + retención del catálogo.
	BackupFrequency string `json:"backup_frequency"`
	BackupRetention int    `json:"backup_retention"`
}

func validStyle(s string) bool {
	switch s {
	case "standard", "brutalist", "glassmorphism", "retro", "dracula":
		return true
	}
	return false
}

// Temas válidos; cualquier otro valor sanea a "dark" (paridad tolerante).
func validTheme(t string) bool {
	switch t {
	case "light", "dark", "oled", "system":
		return true
	}
	return false
}

// DefaultSettings replica los defaults efectivos de Python:
// theme "dark", polling true, toasts true.
func DefaultSettings() Settings {
	return Settings{
		Theme:             "dark",
		Style:             "standard",
		MonitorPolling:    true,
		ToastsEnabled:     true,
		AccentOverrides:   make(map[string]string),
		AccentGlobal:      false,
		AccentGlobalColor: "",
		BackupFrequency:   backup.FreqOff,
		BackupRetention:   20,
	}
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
	if !validStyle(s.Style) {
		s.Style = "standard"
	}
	// Ensure AccentOverrides is never nil for JSON serialization.
	if s.AccentOverrides == nil {
		s.AccentOverrides = make(map[string]string)
	}
	// Backup: frecuencia inválida → off; retención fuera de rango → default.
	if !backup.ValidFrequency(s.BackupFrequency) {
		s.BackupFrequency = backup.FreqOff
	}
	if s.BackupRetention < 1 || s.BackupRetention > 500 {
		s.BackupRetention = 20
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
