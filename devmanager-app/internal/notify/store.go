package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store persiste la config (plataformas + reglas) y el historial de entregas
// en JSON dentro del dir de settings (%APPDATA%/devManager/notify.json +
// notify-history.jsonl), siguiendo el patrón de internal/backup y testadv.
type Store struct {
	mu sync.Mutex

	path     string // notify.json
	histPath string // notify-history.jsonl

	Platforms []PlatformConfig `json:"platforms"`
	Rules     []Rule           `json:"rules"`

	// Rate limiting por plataforma (id): ventanas de 60s.
	sent    map[string][]time.Time
	history []Delivery
}

const (
	historyKeep  = 200
	rateWindow   = time.Minute
	defaultLimit = 10
	// MaxLimit techo del rate limit configurable.
	MaxLimit = 120
)

// NewStore crea el store con paths derivados de settingsDir.
func NewStore(settingsDir string) *Store {
	return &Store{
		path:      filepath.Join(settingsDir, "notify.json"),
		histPath:  filepath.Join(settingsDir, "notify-history.jsonl"),
		Platforms: []PlatformConfig{},
		Rules:     []Rule{},
		sent:      map[string][]time.Time{},
		history:   []Delivery{},
	}
}

// Load lee config e historial del disco. Ausente/corrupto → store vacío.
func (s *Store) Load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.loadLocked()
}

func (s *Store) loadLocked() {
	s.Platforms = []PlatformConfig{}
	s.Rules = []Rule{}
	s.history = []Delivery{}
	s.sent = map[string][]time.Time{}
	if data, err := os.ReadFile(s.path); err == nil {
		var st Store
		if json.Unmarshal(data, &st) == nil {
			if st.Platforms != nil {
				s.Platforms = st.Platforms
			}
			if st.Rules != nil {
				s.Rules = st.Rules
			}
		}
	}
	if data, err := os.ReadFile(s.histPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			var d Delivery
			if json.Unmarshal([]byte(line), &d) == nil {
				s.history = append(s.history, d)
			}
		}
		// Más nuevo primero al leer (las líneas se escriben en orden).
		sort.Slice(s.history, func(i, j int) bool {
			return s.history[i].At.After(s.history[j].At)
		})
	}
}

// Save persiste la config (plataformas + reglas).
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

// saveLocked escribe el JSON de config; requiere mu tomado.
func (s *Store) saveLocked() error {
	out, err := json.MarshalIndent(struct {
		Platforms []PlatformConfig `json:"platforms"`
		Rules     []Rule           `json:"rules"`
	}{s.Platforms, s.Rules}, "", "  ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(s.path, out, 0o644)
}

// SavePlatform inserta (ID vacío o inexistente) o reemplaza una plataforma
// (upsert) y persiste a disco. Devuelve errores de validación si la config
// no es válida.
func (s *Store) SavePlatform(c PlatformConfig) []string {
	if errs := c.Validate(); len(errs) > 0 {
		return errs
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c.MinPriority == "" {
		c.MinPriority = PriorityInfo
	}
	replaced := false
	if c.ID != "" {
		for i := range s.Platforms {
			if s.Platforms[i].ID == c.ID {
				s.Platforms[i] = c
				replaced = true
				break
			}
		}
	}
	if !replaced {
		if c.ID == "" {
			c.ID = newID(s.Platforms, func(p PlatformConfig) string { return p.ID }, "platform")
		}
		s.Platforms = append(s.Platforms, c)
	}
	if err := s.saveLocked(); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DeletePlatform elimina una plataforma por ID y persiste a disco.
func (s *Store) DeletePlatform(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]PlatformConfig, 0, len(s.Platforms))
	removed := false
	for _, p := range s.Platforms {
		if p.ID == id {
			removed = true
			continue
		}
		out = append(out, p)
	}
	if !removed {
		return []string{fmt.Sprintf("platform %q does not exist", id)}
	}
	s.Platforms = out
	if err := s.saveLocked(); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// SaveRule inserta (ID vacío o inexistente) o reemplaza una regla (upsert)
// y persiste a disco.
func (s *Store) SaveRule(r Rule) []string {
	if errs := r.Validate(); len(errs) > 0 {
		return errs
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Event == "" {
		r.Event = "all"
	}
	replaced := false
	if r.ID != "" {
		for i := range s.Rules {
			if s.Rules[i].ID == r.ID {
				s.Rules[i] = r
				replaced = true
				break
			}
		}
	}
	if !replaced {
		if r.ID == "" {
			r.ID = newID(s.Rules, func(x Rule) string { return x.ID }, "rule")
		}
		s.Rules = append(s.Rules, r)
	}
	if err := s.saveLocked(); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DeleteRule elimina una regla por ID y persiste a disco.
func (s *Store) DeleteRule(id string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Rule, 0, len(s.Rules))
	removed := false
	for _, r := range s.Rules {
		if r.ID == id {
			removed = true
			continue
		}
		out = append(out, r)
	}
	if !removed {
		return []string{fmt.Sprintf("rule %q does not exist", id)}
	}
	s.Rules = out
	if err := s.saveLocked(); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// Config devuelve una copia de plataformas y reglas (para bindings seguros).
func (s *Store) Config() ([]PlatformConfig, []Rule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ps := make([]PlatformConfig, len(s.Platforms))
	copy(ps, s.Platforms)
	rs := make([]Rule, len(s.Rules))
	copy(rs, s.Rules)
	return ps, rs
}

// targetsFor devuelve las plataformas habilitadas que aceptan el evento según
// su propia lista de eventos, min_priority y (opcionalmente) las reglas: si
// existe al menos una regla habilitada, solo las reglas matching enrutan.
func (s *Store) targetsFor(ev Event) []PlatformConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	rank := priorityRank(ev.Priority)

	// Si hay reglas habilitadas que matchean el evento, definen el floor.
	floor := 0
	rulesActive := false
	for _, r := range s.Rules {
		if !r.Enabled {
			continue
		}
		if r.Event != "all" && r.Event != ev.Type {
			continue
		}
		rulesActive = true
		if fr := priorityRank(r.MinPriority); fr > floor {
			floor = fr
		}
	}
	if rulesActive && rank < floor {
		return nil
	}

	var out []PlatformConfig
	for _, p := range s.Platforms {
		if !p.Enabled {
			continue
		}
		if rank < priorityRank(p.MinPriority) {
			continue
		}
		if len(p.Events) > 0 && !containsString(p.Events, ev.Type) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// History devuelve las últimas limit entregas (más nuevas primero).
func (s *Store) History(limit int) []Delivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = historyKeep
	}
	out := make([]Delivery, 0, len(s.history))
	for i, d := range s.history {
		if i >= limit {
			break
		}
		out = append(out, d)
	}
	return out
}

// appendHistory persiste una entrega al final del jsonl (y recorta en
// memoria; el archivo crece pero se relee acotado por Load).
func (s *Store) appendHistory(d Delivery) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append([]Delivery{d}, s.history...)
	if len(s.history) > historyKeep {
		s.history = s.history[:historyKeep]
	}
	if dir := filepath.Dir(s.histPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return
		}
	}
	f, err := os.OpenFile(s.histPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	if b, err := json.Marshal(d); err == nil {
		f.Write(append(b, '\n'))
	}
}

// rateLimited reporta si la plataforma id superó su límite por minuto y
// registra el intento cuando no lo está.
func (s *Store) rateLimited(id string, limit int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sent == nil {
		s.sent = map[string][]time.Time{}
	}
	now := time.Now()
	recent := s.sent[id][:0]
	for _, t := range s.sent[id] {
		if now.Sub(t) < rateWindow {
			recent = append(recent, t)
		}
	}
	s.sent[id] = recent
	if limit <= 0 || limit > MaxLimit {
		limit = defaultLimit
	}
	if len(recent) >= limit {
		return true
	}
	s.sent[id] = append(s.sent[id], now)
	return false
}

// newID genera un ID estable a partir de base con sufijo -2, -3…
func newID[T any](items []T, getID func(T) string, base string) string {
	taken := map[string]bool{}
	for _, it := range items {
		taken[getID(it)] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d", base, i)
		if !taken[c] {
			return c
		}
	}
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
