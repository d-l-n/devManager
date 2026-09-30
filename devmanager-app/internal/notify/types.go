// Package notify implementa el sistema de notificaciones avanzado (Issue #66):
// plataformas externas (Slack, Discord, Telegram, Teams), reglas por evento y
// prioridad, plantillas, rate limiting, historial con estado de entrega y
// reintentos con backoff.
package notify

import "time"

// Plataformas soportadas.
const (
	PlatformSlack    = "slack"
	PlatformDiscord  = "discord"
	PlatformTelegram = "telegram"
	PlatformTeams    = "teams"
)

// Prioridades (ordenadas de menor a mayor severidad).
const (
	PriorityInfo     = "info"
	PriorityWarning  = "warning"
	PriorityCritical = "critical"
)

// Tipos de evento notificable. "app" cubre cualquier notify genérico de la
// app (toasts/burbujas); los específicos permiten reglas finas.
const (
	EventApp            = "app"
	EventServerStarted  = "server_started"
	EventServerStopped  = "server_stopped"
	EventTestsFinished  = "tests_finished"
	EventBackupDone     = "backup_done"
	EventWorkflowStep   = "workflow_step"
	EventBuildFailed    = "build_failed"
	EventServerCrashed  = "server_crashed"
)

// KnownPlatforms lista las plataformas válidas (para validar config).
func KnownPlatforms() []string {
	return []string{PlatformSlack, PlatformDiscord, PlatformTelegram, PlatformTeams}
}

// KnownEvents lista los eventos sobre los que se pueden crear reglas.
func KnownEvents() []string {
	return []string{
		EventApp, EventServerStarted, EventServerStopped, EventTestsFinished,
		EventBackupDone, EventWorkflowStep, EventBuildFailed, EventServerCrashed,
	}
}

// KnownPriorities lista las prioridades válidas, de menor a mayor.
func KnownPriorities() []string {
	return []string{PriorityInfo, PriorityWarning, PriorityCritical}
}

// ValidPlatform reporta si p es una plataforma soportada.
func ValidPlatform(p string) bool {
	for _, k := range KnownPlatforms() {
		if k == p {
			return true
		}
	}
	return false
}

// ValidEvent reporta si e es un evento conocido.
func ValidEvent(e string) bool {
	for _, k := range KnownEvents() {
		if k == e {
			return true
		}
	}
	return false
}

// ValidPriority reporta si p es una prioridad conocida.
func ValidPriority(p string) bool {
	for _, k := range KnownPriorities() {
		if k == p {
			return true
		}
	}
	return false
}

// priorityRank ordena prioridades: info=0, warning=1, critical=2.
func priorityRank(p string) int {
	switch p {
	case PriorityWarning:
		return 1
	case PriorityCritical:
		return 2
	default:
		return 0
	}
}

// Event es una notificación a dispatchar.
type Event struct {
	Type     string `json:"type"`               // server_started, app, ...
	Project  string `json:"project,omitempty"`  // nombre del proyecto, si aplica
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority string `json:"priority"`           // info|warning|critical
	At       time.Time `json:"at"`
}

// PlatformConfig es un destino externo configurado por el usuario. Los
// secretos (tokens de Telegram) viajan en claro SOLO de app → servicio, y se
// persisten en notify.json junto a settings (mismo patrón que env secrets:
// masking solo aplica a lecturas de vars de entorno, aquí la config es local).
type PlatformConfig struct {
	ID         string `json:"id"`
	Name       string `json:"name"`                // etiqueta legible
	Platform   string `json:"platform"`            // slack|discord|telegram|teams
	WebhookURL string `json:"webhook_url"`         // slack/discord/teams
	BotToken   string `json:"bot_token,omitempty"` // telegram
	ChatID     string `json:"chat_id,omitempty"`   // telegram
	// MinPriority: no enviar eventos por debajo de esta prioridad.
	MinPriority string `json:"min_priority"`
	// Events vacío = todos los eventos (paridad tolerante).
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

// Validate devuelve errores de config (lista vacía = ok).
func (c PlatformConfig) Validate() []string {
	var errs []string
	if c.Name == "" {
		errs = append(errs, "name cannot be empty")
	}
	if !ValidPlatform(c.Platform) {
		errs = append(errs, "invalid platform "+c.Platform)
	}
	switch c.Platform {
	case PlatformTelegram:
		if c.BotToken == "" || c.ChatID == "" {
			errs = append(errs, "telegram needs bot_token and chat_id")
		}
	default:
		if !isHTTPURL(c.WebhookURL) {
			errs = append(errs, c.Platform+": webhook_url must be http(s)")
		}
	}
	if c.MinPriority != "" && !ValidPriority(c.MinPriority) {
		errs = append(errs, "invalid min_priority "+c.MinPriority)
	}
	for _, e := range c.Events {
		if !ValidEvent(e) {
			errs = append(errs, "invalid event "+e)
		}
	}
	return errs
}

// Rule enruta eventos: un evento dispara la regla si type coincide (o rule
// event="all") y la prioridad alcanza MinPriority.
type Rule struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Event       string `json:"event"`       // evento concreto o "all"
	MinPriority string `json:"min_priority"`
	Enabled     bool   `json:"enabled"`
}

// Validate devuelve errores de la regla.
func (r Rule) Validate() []string {
	var errs []string
	if r.Name == "" {
		errs = append(errs, "name cannot be empty")
	}
	if r.Event != "all" && !ValidEvent(r.Event) {
		errs = append(errs, "invalid event "+r.Event)
	}
	if !ValidPriority(r.MinPriority) {
		errs = append(errs, "invalid min_priority "+r.MinPriority)
	}
	return errs
}

// Delivery es una entrada del historial de envíos.
type Delivery struct {
	ID        string    `json:"id"`
	At        time.Time `json:"at"`
	Platform  string    `json:"platform"`
	Name      string    `json:"name"` // etiqueta de la plataforma destino
	Event     string    `json:"event"`
	Priority  string    `json:"priority"`
	Title     string    `json:"title"`
	Status    string    `json:"status"` // sent|failed|rate_limited|filtered
	Attempts  int       `json:"attempts"`
	Error     string    `json:"error,omitempty"`
}

// Estados de entrega.
const (
	StatusSent        = "sent"
	StatusFailed      = "failed"
	StatusRateLimited = "rate_limited"
	StatusFiltered    = "filtered"
)

func isHTTPURL(s string) bool {
	return len(s) > 8 && (s[:7] == "http://" || s[:8] == "https://")
}
