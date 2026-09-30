package help

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Progress es el estado persistido del sistema de ayuda.
type Progress struct {
	Tutorials       Progresses `json:"tutorials"`
	LastTopic       string     `json:"lastTopic,omitempty"`
	DismissedTips   []string   `json:"dismissedTips,omitempty"`
	SchemaVersion   int        `json:"schemaVersion"`
	LastInteraction time.Time  `json:"lastInteraction,omitempty"`
}

// CurrentProgressSchema es la versión del esquema persistido.
const CurrentProgressSchema = 1

// NewProgress devuelve un progreso vacío listo para usar.
func NewProgress() Progress {
	return Progress{Tutorials: Progresses{}, DismissedTips: []string{}, SchemaVersion: CurrentProgressSchema}
}

// LoadProgress lee el progreso de forma tolerante: fichero ausente, vacío o
// corrupto devuelve un progreso nuevo (la ayuda nunca bloquea la app).
func LoadProgress(path string) Progress {
	data, err := os.ReadFile(path)
	if err != nil {
		return NewProgress()
	}
	var p Progress
	if err := json.Unmarshal(data, &p); err != nil {
		return NewProgress()
	}
	if p.Tutorials == nil {
		p.Tutorials = Progresses{}
	}
	if p.DismissedTips == nil {
		p.DismissedTips = []string{}
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = CurrentProgressSchema
	}
	return p
}

// SaveProgress escribe el progreso de forma atómica.
func SaveProgress(path string, p Progress) error {
	if p.Tutorials == nil {
		p.Tutorials = Progresses{}
	}
	if p.DismissedTips == nil {
		p.DismissedTips = []string{}
	}
	if p.SchemaVersion == 0 {
		p.SchemaVersion = CurrentProgressSchema
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("serializar progreso de ayuda: %w", err)
	}
	return writeAtomic(path, data, 0o644)
}

// SetTutorialStep registra el último paso completado de un tutorial. El índice
// se acota al rango del tutorial (nunca se persiste un paso inexistente).
func (p Progress) SetTutorialStep(tutorialID string, step int) Progress {
	tutorial, ok := FindTutorial(tutorialID)
	if !ok {
		return p
	}
	if p.Tutorials == nil {
		p.Tutorials = Progresses{}
	}
	bounded := NormalizeStep(tutorial, step)
	p.Tutorials[tutorialID] = TutorialProgress{
		Step:      bounded,
		Total:     len(tutorial.Steps),
		Completed: IsComplete(tutorial, TutorialProgress{Step: bounded}),
		UpdatedAt: time.Now(),
	}
	p.LastInteraction = time.Now()
	return p
}

// DismissTip marca un tip/elemento como descartado (idempotente).
func (p Progress) DismissTip(id string) Progress {
	for _, existing := range p.DismissedTips {
		if existing == id {
			return p
		}
	}
	p.DismissedTips = append(p.DismissedTips, id)
	p.LastInteraction = time.Now()
	return p
}

// IsTipDismissed indica si un tip ya fue descartado.
func (p Progress) IsTipDismissed(id string) bool {
	for _, existing := range p.DismissedTips {
		if existing == id {
			return true
		}
	}
	return false
}

// SetLastTopic recuerda el último tema visto (para reabrir la ayuda donde estabas).
func (p Progress) SetLastTopic(topicID string) Progress {
	if _, ok := FindTopic(topicID); !ok {
		return p
	}
	p.LastTopic = topicID
	p.LastInteraction = time.Now()
	return p
}

// Reset limpia todo el progreso.
func (p Progress) Reset() Progress {
	return NewProgress()
}

func writeAtomic(path string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("crear directorio de ayuda: %w", err)
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
