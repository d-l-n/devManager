package workflow

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// MaxRunsPerWorkflow capa el historial en memoria y en disco.
const MaxRunsPerWorkflow = 100

// Store guarda runs por workflowId en memoria (cap 100) con persistencia
// best-effort en JSON. Nunca devuelve error al caller (log interno nulo:
// el caller decide si notifica).
type Store struct {
	mu   sync.Mutex
	path string
	runs map[string][]Run
}

// NewStore crea el store; Load() debe llamarse para hidratar desde disco.
func NewStore(path string) *Store {
	return &Store{path: path, runs: map[string][]Run{}}
}

// Load lee el archivo si existe; corrupto/ausente → vacío (best-effort).
func (s *Store) Load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var file struct {
		Runs map[string][]Run `json:"runs"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return
	}
	for id, list := range file.Runs {
		if len(list) > MaxRunsPerWorkflow {
			list = list[:MaxRunsPerWorkflow]
		}
		s.runs[id] = append([]Run{}, list...)
	}
}

func (s *Store) saveLocked() {
	if s.path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	out, err := json.MarshalIndent(map[string]interface{}{"runs": s.runs}, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, out, 0o644)
}

// Add antepone el run y capa a 100; persiste best-effort.
func (s *Store) Add(run Run) {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := append([]Run{run}, s.runs[run.WorkflowID]...)
	if len(list) > MaxRunsPerWorkflow {
		list = list[:MaxRunsPerWorkflow]
	}
	s.runs[run.WorkflowID] = list
	s.saveLocked()
}

// Get devuelve copia de los runs del workflow (newest primero).
func (s *Store) Get(workflowID string) []Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Run{}, s.runs[workflowID]...)
}

// All devuelve todos los runs (newest primero global por StartedAt desc).
func (s *Store) All() []Run {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Run
	for _, list := range s.runs {
		out = append(out, list...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	if out == nil {
		return []Run{}
	}
	return out
}

// LastFinishedAt devuelve el FinishedAt más reciente del workflow
// ("" si no hay runs terminados). Para el scheduler @every.
func (s *Store) LastFinishedAt(workflowID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.runs[workflowID] {
		if r.Status == "success" || r.Status == "failed" {
			return r.FinishedAt
		}
	}
	return ""
}

// Clear borra el historial de un workflow (al borrar el workflow).
func (s *Store) Clear(workflowID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, workflowID)
	s.saveLocked()
}
