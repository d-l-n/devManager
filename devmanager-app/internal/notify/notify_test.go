package notify

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s := NewStore(dir)
	s.Load()
	return s, dir
}

// ---- Validate ----

func TestPlatformValidate(t *testing.T) {
	c := PlatformConfig{Name: "Slack", Platform: PlatformSlack, WebhookURL: "https://hooks.slack.com/x", MinPriority: PriorityWarning}
	if errs := c.Validate(); len(errs) != 0 {
		t.Fatalf("errores inesperados: %v", errs)
	}
	c.WebhookURL = "ftp://nope"
	if errs := c.Validate(); len(errs) == 0 {
		t.Fatal("webhook no-http debe fallar")
	}
	tg := PlatformConfig{Platform: PlatformTelegram, Name: "tg"}
	if errs := tg.Validate(); len(errs) == 0 {
		t.Fatal("telegram sin token/chat debe fallar")
	}
	tg.BotToken, tg.ChatID = "tok", "123"
	if errs := tg.Validate(); len(errs) != 0 {
		t.Fatalf("telegram completo debe pasar: %v", errs)
	}
}

func TestRuleValidate(t *testing.T) {
	r := Rule{Name: "solo críticos", Event: "all", MinPriority: PriorityCritical}
	if errs := r.Validate(); len(errs) != 0 {
		t.Fatalf("errores inesperados: %v", errs)
	}
	r.Event = "nope"
	if errs := r.Validate(); len(errs) == 0 {
		t.Fatal("evento inválido debe fallar")
	}
}

// ---- Store CRUD + persistencia ----

func TestPlatformCRUDYPersistencia(t *testing.T) {
	s, dir := newTestStore(t)
	if errs := s.SavePlatform(PlatformConfig{Name: "Slack canal", Platform: PlatformSlack, WebhookURL: "https://x", Enabled: true}); len(errs) != 0 {
		t.Fatalf("save: %v", errs)
	}
	ps, _ := s.Config()
	if len(ps) != 1 || ps[0].ID != "platform" {
		t.Fatalf("plataforma no guardada: %+v", ps)
	}
	// Update por ID.
	ps[0].Name = "Renombrada"
	if errs := s.SavePlatform(ps[0]); len(errs) != 0 {
		t.Fatalf("update: %v", errs)
	}
	// Recarga desde disco.
	s2 := NewStore(dir)
	s2.Load()
	ps2, _ := s2.Config()
	if len(ps2) != 1 || ps2[0].Name != "Renombrada" {
		t.Fatalf("persistencia rota: %+v", ps2)
	}
	if errs := s2.DeletePlatform("platform"); len(errs) != 0 {
		t.Fatalf("delete: %v", errs)
	}
	if errs := s2.DeletePlatform("platform"); len(errs) == 0 {
		t.Fatal("segundo delete debe fallar")
	}
}

func TestRuleCRUD(t *testing.T) {
	s, _ := newTestStore(t)
	if errs := s.SaveRule(Rule{Name: "r1", Event: EventServerCrashed, MinPriority: PriorityWarning}); len(errs) != 0 {
		t.Fatalf("save: %v", errs)
	}
	_, rules := s.Config()
	if len(rules) != 1 || rules[0].ID != "rule" {
		t.Fatalf("regla no guardada: %+v", rules)
	}
	if errs := s.DeleteRule("rule"); len(errs) != 0 {
		t.Fatalf("delete: %v", errs)
	}
}

// ---- Enrutado ----

type recordingSender struct {
	mu       sync.Mutex
	calls    int
	fail     bool
	lastURL  string
	lastBody map[string]string
}

func (r *recordingSender) Post(url, contentType string, body []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	r.lastURL = url
	json.Unmarshal(body, &r.lastBody)
	if r.fail {
		return 500, errFalso{}
	}
	return 200, nil
}

type errFalso struct{}

func (errFalso) Error() string { return "boom" }

func TestDispatchFiltraPorPrioridadYEvento(t *testing.T) {
	s, _ := newTestStore(t)
	s.SavePlatform(PlatformConfig{ID: "p-warn", Name: "warn", Platform: PlatformSlack, WebhookURL: "https://w", MinPriority: PriorityWarning, Enabled: true})
	s.SavePlatform(PlatformConfig{ID: "p-info", Name: "info", Platform: PlatformSlack, WebhookURL: "https://i", MinPriority: PriorityInfo, Enabled: true})
	d := NewDispatcher(s)

	// info: solo p-info.
	got := d.Dispatch(Event{Type: EventApp, Title: "t", Message: "m", Priority: PriorityInfo})
	if len(got) != 1 || got[0].Name != "info" {
		t.Fatalf("info debe llegar solo a p-info: %+v", got)
	}
	// warning: ambas (warning >= info).
	got = d.Dispatch(Event{Type: EventApp, Title: "t", Message: "m", Priority: PriorityWarning})
	if len(got) != 2 {
		t.Fatalf("warning debe llegar a ambas: %+v", got)
	}
}

func TestDispatchReglasDefinenFloor(t *testing.T) {
	s, _ := newTestStore(t)
	s.SavePlatform(PlatformConfig{ID: "p", Name: "p", Platform: PlatformSlack, WebhookURL: "https://w", Enabled: true})
	s.SaveRule(Rule{Name: "solo críticos", Event: "all", MinPriority: PriorityCritical, Enabled: true})
	d := NewDispatcher(s)

	got := d.Dispatch(Event{Type: EventApp, Title: "t", Priority: PriorityInfo})
	if len(got) != 0 {
		t.Fatalf("regla critical debe filtrar info: %+v", got)
	}
	got = d.Dispatch(Event{Type: EventApp, Title: "t", Priority: PriorityCritical})
	if len(got) != 1 {
		t.Fatalf("critical debe pasar: %+v", got)
	}
	// Regla deshabilitada no filtra.
	s.SaveRule(Rule{ID: "rule", Name: "solo críticos", Event: "all", MinPriority: PriorityCritical, Enabled: false})
	got = d.Dispatch(Event{Type: EventApp, Title: "t", Priority: PriorityInfo})
	if len(got) != 1 {
		t.Fatalf("regla off no debe filtrar: %+v", got)
	}
}

func TestDispatchHistorialYEstados(t *testing.T) {
	s, _ := newTestStore(t)
	s.SavePlatform(PlatformConfig{ID: "p", Name: "p", Platform: PlatformDiscord, WebhookURL: "https://w", Enabled: true})
	sender := &recordingSender{fail: true}
	d := &Dispatcher{Store: s, Sender: sender, Retries: 1}
	d.Dispatch(Event{Type: EventServerCrashed, Title: "crash", Priority: PriorityCritical})

	hist := s.History(10)
	if len(hist) != 1 {
		t.Fatalf("historial vacío: %+v", hist)
	}
	if hist[0].Status != StatusFailed || hist[0].Attempts != 2 {
		t.Fatalf("estado/attempts incorrectos: %+v", hist[0])
	}
}

func TestRateLimitPorMinuto(t *testing.T) {
	s, _ := newTestStore(t)
	s.SavePlatform(PlatformConfig{ID: "p", Name: "p", Platform: PlatformSlack, WebhookURL: "https://w", Enabled: true})
	sender := &recordingSender{}
	d := &Dispatcher{Store: s, Sender: sender}
	for i := 0; i < defaultLimit+2; i++ {
		d.Dispatch(Event{Type: EventApp, Title: "t"})
	}
	hist := s.History(50)
	limited := 0
	sent := 0
	for _, h := range hist {
		switch h.Status {
		case StatusRateLimited:
			limited++
		case StatusSent:
			sent++
		}
	}
	if sent != defaultLimit || limited != 2 {
		t.Fatalf("sent=%d limited=%d, want 10/2", sent, limited)
	}
}

func TestSendersPayloads(t *testing.T) {
	ev := Event{Type: EventServerStarted, Project: "Alpha", Title: "Servidor arriba", Message: "listo", Priority: PriorityWarning}
	slack, _, body, _ := slackPayload(PlatformConfig{WebhookURL: "https://s"}, ev)
	if slack != "https://s" {
		t.Fatalf("url slack: %s", slack)
	}
	var m map[string]string
	json.Unmarshal(body, &m)
	if m["text"] != "[WARNING] Servidor arriba — Alpha: listo" {
		t.Fatalf("texto slack: %q", m["text"])
	}
	disc, _, body, _ := discordPayload(PlatformConfig{WebhookURL: "https://d"}, ev)
	if disc != "https://d" {
		t.Fatalf("url discord: %s", disc)
	}
	tgURL, _, body, err := telegramPayload(PlatformConfig{BotToken: "T", ChatID: "7"}, ev)
	if err != nil || tgURL != "https://api.telegram.org/botT/sendMessage" {
		t.Fatalf("url telegram: %s %v", tgURL, err)
	}
	json.Unmarshal(body, &m)
	if m["chat_id"] != "7" {
		t.Fatalf("chat_id: %v", m)
	}
	teams, _, _, err := teamsPayload(PlatformConfig{WebhookURL: "https://t"}, ev)
	if err != nil || teams != "https://t" {
		t.Fatalf("url teams: %s %v", teams, err)
	}
}

func TestHTTPSenderContraServerReal(t *testing.T) {
	var gotCode int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCode = http.StatusOK
		w.WriteHeader(200)
	}))
	defer srv.Close()
	s := HTTPSender{}
	code, err := s.Post(srv.URL, "application/json", []byte(`{}`))
	if err != nil || code != 200 || gotCode != 200 {
		t.Fatalf("post: code=%d err=%v", code, err)
	}
}
