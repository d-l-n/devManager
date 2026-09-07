// Package dashboard persiste el historial de uptime por proyecto (Issue #64):
// samples {ts, running, uptime_sec} tomados por el sampler del App y guardados
// en %APPDATA%/devManager/dashboard-history.json con retención de 24h.
package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	// SampleInterval es la frecuencia del sampler del App (no de este paquete).
	SampleInterval = 60 * time.Second
	// Retention es la ventana máxima de samples guardados en disco.
	Retention = 24 * time.Hour
	// Version del schema del archivo.
	Version = 1
)

// Sample es un punto del historial de un proyecto.
type Sample struct {
	Ts      int64   `json:"ts"`            // unix seconds
	Running bool    `json:"running"`       // server running en el momento del sample
	Uptime  int64   `json:"uptime_sec"`    // segundos corriendo (0 si stopped)
	CPU     float64 `json:"cpu,omitempty"` // % CPU del árbol (0 si no disponible)
	RSS     float64 `json:"rss,omitempty"` // MB RSS del árbol
}

// ProjectHistory agrupa los samples de un proyecto.
type ProjectHistory struct {
	Name    string   `json:"name"`
	Samples []Sample `json:"samples"`
}

// file es el schema en disco.
type file struct {
	Version  int                 `json:"version"`
	Projects map[string][]Sample `json:"projects"`
}

// Store carga/guarda el historial y aplica retención. Thread-safe.
type Store struct {
	mu   sync.Mutex
	path string
	data file
}

func NewStore(path string) *Store {
	s := &Store{path: path, data: file{Version: Version, Projects: map[string][]Sample{}}}
	s.load()
	return s
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return // ausente → vacío
	}
	var f file
	if json.Unmarshal(data, &f) != nil || f.Version != Version {
		return // corrupto → vacío (paridad LoadSettings)
	}
	if f.Projects == nil {
		f.Projects = map[string][]Sample{}
	}
	s.data = f
}

// Append agrega un sample para name, aplica retención y persiste (atómico).
func (s *Store) Append(name string, sample Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Unix(sample.Ts, 0)
	cutoff := now.Add(-Retention)

	series := append(s.data.Projects[name], sample)
	s.data.Projects[name] = prune(series, cutoff)
	s.data.Projects = pruneAll(s.data.Projects, cutoff)

	return s.saveLocked()
}

// Since devuelve el historial por proyecto recortado a la ventana pedida,
// ordenado por nombre (binding GetDashboardHistory).
func (s *Store) Since(window time.Duration) []ProjectHistory {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-window)
	out := make([]ProjectHistory, 0, len(s.data.Projects))
	for name, series := range s.data.Projects {
		windowed := prune(series, cutoff)
		if len(windowed) == 0 {
			continue
		}
		out = append(out, ProjectHistory{Name: name, Samples: windowed})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// prune filtra samples con Ts >= cutoff (asumiendo serie ordenada por Ts).
func prune(series []Sample, cutoff time.Time) []Sample {
	out := series[:0]
	for _, s := range series {
		if time.Unix(s.Ts, 0).After(cutoff) {
			out = append(out, s)
		}
	}
	return out
}

// pruneAll elimina series sin samples vivos (nombres renombrados/eliminados).
func pruneAll(all map[string][]Sample, cutoff time.Time) map[string][]Sample {
	out := make(map[string][]Sample, len(all))
	for name, series := range all {
		kept := prune(series, cutoff)
		if len(kept) > 0 {
			out[name] = kept
		}
	}
	return out
}

// saveLocked escribe atómicamente (tmp + rename). Caller tiene el lock.
func (s *Store) saveLocked() error {
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
