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
	// Dashboard (Issue #64): visibilidad de secciones (projects/alerts/uptime/
	// perf). Clave ausente o nil → sección visible (paridad tolerante).
	DashboardSections map[string]bool `json:"dashboard_sections"`
	// Workflows (Issue #65): listener HTTP entrante para webhooks.
	// Port 0/ausente → default 9876; Enabled ausente → true (struct zero
	// false se distingue por port==0 en Load: legacy sin campos → enabled).
	WorkflowWebhookPort    int  `json:"workflow_webhook_port"`
	WorkflowWebhookEnabled bool `json:"workflow_webhook_enabled"`
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
		DashboardSections: make(map[string]bool),
		// Workflows (Issue #65): listener entrante localhost:9876 activo.
		WorkflowWebhookPort:    9876,
		WorkflowWebhookEnabled: true,
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
	if s.DashboardSections == nil {
		s.DashboardSections = make(map[string]bool)
	}
	// Workflows (Issue #65): 0/ausente → 9876; legacy (port 0) → enabled true
	// salvo que el JSON traiga enabled:false explícito con port válido.
	if s.WorkflowWebhookPort == 0 {
		// Distingue legacy (sin campos) de "puerto 0 explícito": ambos caen
		// al default 9876; deshabilitar se hace con enabled:false.
		var raw struct {
			Port    *int  `json:"workflow_webhook_port"`
			Enabled *bool `json:"workflow_webhook_enabled"`
		}
		if err := json.Unmarshal(data, &raw); err == nil && raw.Port == nil && raw.Enabled == nil {
			s.WorkflowWebhookEnabled = true
		}
		s.WorkflowWebhookPort = 9876
	}
	if s.WorkflowWebhookPort < 1 || s.WorkflowWebhookPort > 65535 {
		s.WorkflowWebhookPort = 9876
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
