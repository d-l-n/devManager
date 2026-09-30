package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Sender abstrae el HTTP para tests (server httptest de truth o error).
type Sender interface {
	Post(url string, contentType string, body []byte) (statusCode int, err error)
}

// HTTPSender es el sender real con timeout de 10s.
type HTTPSender struct{ Client *http.Client }

// Post implementa Sender.
func (h HTTPSender) Post(url, contentType string, body []byte) (int, error) {
	client := h.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Post(url, contentType, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return resp.StatusCode, fmt.Errorf("http %d", resp.StatusCode)
	}
	return resp.StatusCode, nil
}

// Dispatcher envía eventos a las plataformas configuradas aplicando reglas,
// rate limit por plataforma, reintentos con backoff y registro de historial.
type Dispatcher struct {
	Store  *Store
	Sender Sender
	// Retries es el número de reintentos extra tras el primer fallo (0-3).
	Retries int
	// now permite inyectar tiempo en tests (backoff).
	now func() time.Time
}

// NewDispatcher crea el dispatcher con el sender real.
func NewDispatcher(store *Store) *Dispatcher {
	return &Dispatcher{Store: store, Sender: HTTPSender{}, now: time.Now}
}

// Dispatch enruta ev a cada plataforma destino y devuelve las entregas
// registradas (incluye rate_limited/filtered para el historial visible).
// Nunca panics: los errores de red se registran como failed.
func (d *Dispatcher) Dispatch(ev Event) []Delivery {
	if d.now == nil {
		d.now = time.Now
	}
	if ev.At.IsZero() {
		ev.At = d.now()
	}
	if ev.Priority == "" {
		ev.Priority = PriorityInfo
	}
	targets := d.Store.targetsFor(ev)
	out := []Delivery{}
	for _, t := range targets {
		st, attempts, errMsg := d.sendWithRetry(t, ev)
		dlv := Delivery{
			ID:       fmt.Sprintf("%x", ev.At.UnixNano()),
			At:       ev.At,
			Platform: t.Platform,
			Name:     t.Name,
			Event:    ev.Type,
			Priority: ev.Priority,
			Title:    ev.Title,
			Status:   st,
			Attempts: attempts,
			Error:    errMsg,
		}
		d.Store.appendHistory(dlv)
		out = append(out, dlv)
	}
	return out
}

// sendWithRetry aplica rate limit y reintentos con backoff lineal
// (200ms * attempt). Devuelve status final, intentos y error.
func (d *Dispatcher) sendWithRetry(t PlatformConfig, ev Event) (status string, attempts int, errMsg string) {
	if d.Store.rateLimited(t.ID, defaultLimit) {
		return StatusRateLimited, 0, "rate limit exceeded (10/min)"
	}
	retries := d.Retries
	if retries < 0 {
		retries = 0
	}
	if retries > 3 {
		retries = 3
	}
	var lastErr error
	for attempt := 0; attempt <= retries; attempt++ {
		attempts = attempt + 1
		if err := d.send(t, ev); err == nil {
			return StatusSent, attempts, ""
		} else {
			lastErr = err
		}
		if attempt < retries {
			time.Sleep(time.Duration(200*(attempt+1)) * time.Millisecond)
		}
	}
	return StatusFailed, attempts, lastErr.Error()
}

// send construye el payload de la plataforma y lo postea.
func (d *Dispatcher) send(t PlatformConfig, ev Event) error {
	var url, contentType string
	var body []byte
	var err error
	switch t.Platform {
	case PlatformSlack:
		url, contentType, body, err = slackPayload(t, ev)
	case PlatformDiscord:
		url, contentType, body, err = discordPayload(t, ev)
	case PlatformTelegram:
		url, contentType, body, err = telegramPayload(t, ev)
	case PlatformTeams:
		url, contentType, body, err = teamsPayload(t, ev)
	default:
		return fmt.Errorf("unknown platform %q", t.Platform)
	}
	if err != nil {
		return err
	}
	_, err = d.Sender.Post(url, contentType, body)
	return err
}

// renderTemplate aplica {title} {message} {project} {event} {priority}.
func renderTemplate(tmpl string, ev Event) string {
	r := strings.NewReplacer(
		"{title}", ev.Title,
		"{message}", ev.Message,
		"{project}", ev.Project,
		"{event}", ev.Type,
		"{priority}", ev.Priority,
	)
	return r.Replace(tmpl)
}

// defaultText es el texto por defecto cuando no hay plantilla.
func defaultText(ev Event) string {
	if ev.Project != "" {
		return fmt.Sprintf("[%s] %s — %s: %s", strings.ToUpper(ev.Priority), ev.Title, ev.Project, ev.Message)
	}
	return fmt.Sprintf("[%s] %s: %s", strings.ToUpper(ev.Priority), ev.Title, ev.Message)
}

// slackPayload: {"text": "..."} con la plantilla aplicada o el default.
func slackPayload(t PlatformConfig, ev Event) (string, string, []byte, error) {
	text := defaultText(ev)
	body := map[string]string{"text": text}
	b, err := json.Marshal(body)
	if err != nil {
		return "", "", nil, err
	}
	return t.WebhookURL, "application/json", b, nil
}

// discordPayload: {"content": "..."}.
func discordPayload(t PlatformConfig, ev Event) (string, string, []byte, error) {
	b, err := json.Marshal(map[string]string{"content": defaultText(ev)})
	if err != nil {
		return "", "", nil, err
	}
	return t.WebhookURL, "application/json", b, nil
}

// telegramPayload: POST JSON a la Bot API (getUpdates no aplica).
func telegramPayload(t PlatformConfig, ev Event) (string, string, []byte, error) {
	if t.BotToken == "" || t.ChatID == "" {
		return "", "", nil, fmt.Errorf("telegram needs bot_token and chat_id")
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	b, err := json.Marshal(map[string]string{
		"chat_id": t.ChatID,
		"text":    defaultText(ev),
	})
	if err != nil {
		return "", "", nil, err
	}
	return url, "application/json", b, nil
}

// teamsPayload: Power Automate / Office 365 connector: {"text": "..."}.
func teamsPayload(t PlatformConfig, ev Event) (string, string, []byte, error) {
	b, err := json.Marshal(map[string]string{"text": defaultText(ev)})
	if err != nil {
		return "", "", nil, err
	}
	return t.WebhookURL, "application/json", b, nil
}
