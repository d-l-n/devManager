package testadv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Store es el estado persistido de testing avanzado de un proyecto.
type Store struct {
	Schedules []Schedule `json:"schedules"`
}

// LoadStore lee el store de forma tolerante: fichero ausente, vacío o corrupto
// devuelve un store vacío (nunca bloquea la UI por un JSON roto).
func LoadStore(path string) Store {
	empty := Store{Schedules: []Schedule{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return empty
	}
	var st Store
	if err := json.Unmarshal(data, &st); err != nil {
		return empty
	}
	if st.Schedules == nil {
		st.Schedules = []Schedule{}
	}
	return st
}

// SaveStore escribe el store de forma atómica.
func SaveStore(path string, st Store) error {
	if st.Schedules == nil {
		st.Schedules = []Schedule{}
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar store: %w", err)
	}
	return WriteAtomic(path, data, 0o644)
}

// WriteAtomic escribe con rename para no dejar el fichero a medias si la app
// se cierra durante el guardado.
func WriteAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("crear directorio: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return fmt.Errorf("escribir temporal: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renombrar temporal: %w", err)
	}
	return nil
}

// AppendRun añade un run al histórico JSONL y lo recorta a keep entradas
// (keep<=0 → sin límite). Las líneas corruptas se descartan al reescribir.
func AppendRun(path string, rec RunRecord, keep int) error {
	runs, err := LoadRuns(path, 0)
	if err != nil {
		return err
	}
	runs = append(runs, rec)
	if keep > 0 && len(runs) > keep {
		runs = runs[len(runs)-keep:]
	}
	var sb strings.Builder
	for _, r := range runs {
		line, err := json.Marshal(r)
		if err != nil {
			return fmt.Errorf("serializar run: %w", err)
		}
		sb.Write(line)
		sb.WriteByte('\n')
	}
	return WriteAtomic(path, []byte(sb.String()), 0o644)
}

// LoadRuns lee el histórico JSONL (limit>0 → últimos limit).
func LoadRuns(path string, limit int) ([]RunRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []RunRecord{}, nil
		}
		return nil, fmt.Errorf("leer histórico de runs: %w", err)
	}
	out := []RunRecord{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rec RunRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			continue // línea corrupta: se ignora, el resto del histórico sirve
		}
		out = append(out, rec)
	}
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}
