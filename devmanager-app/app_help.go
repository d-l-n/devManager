package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/d-l-n/devmanager/internal/help"
)

// ---- Help & documentation bindings (Issue #72) ----
//
// Igual que app_testadv.go, este fichero evita tocar app.go para no colisionar
// con el trabajo de #67. El progreso de la ayuda se persiste en
// %APPDATA%/devManager/help.json (patrón de settings.json).

// helpProgressPath devuelve el fichero de progreso (tutoriales, tips vistos).
func (a *App) helpProgressPath() string {
	return filepath.Join(filepath.Dir(a.settingsPath), "help.json")
}

// helpExeDir es el directorio del binario: primer sitio donde se busca
// CHANGELOG.md (en dev el repo está un nivel por encima del CWD).
func (a *App) helpExeDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(exePath)
}

// HelpProgressSummary resume el avance de tutoriales para la UI.
type HelpProgressSummary struct {
	Completed int     `json:"completed"`
	Total     int     `json:"total"`
	Pct       float64 `json:"pct"`
}

// HelpIndex es todo lo necesario para abrir la ventana de ayuda en una llamada.
type HelpIndex struct {
	Version        string              `json:"version"`
	Topics         []help.Topic        `json:"topics"`
	Sections       []string            `json:"sections"`
	FAQ            []help.FAQ          `json:"faq"`
	FAQCategories  []string            `json:"faqCategories"`
	Shortcuts      []help.Shortcut     `json:"shortcuts"`
	ShortcutGroups []string            `json:"shortcutGroups"`
	Tutorials      []help.Tutorial     `json:"tutorials"`
	Context        []help.ContextHelp  `json:"context"`
	Progress       help.Progress       `json:"progress"`
	Summary        HelpProgressSummary `json:"summary"`
	Changelog      []help.Release      `json:"changelog"`
}

// ContextHelpResult envuelve la ayuda contextual (found=false si no existe).
type ContextHelpResult struct {
	Found bool             `json:"found"`
	Help  help.ContextHelp `json:"help"`
}

// GetHelpIndex devuelve catálogo, FAQ, atajos, tutoriales, ayuda contextual,
// progreso y changelog. El changelog se recorta a las 5 versiones más recientes.
func (a *App) GetHelpIndex() HelpIndex {
	progress := help.LoadProgress(a.helpProgressPath())
	completed, total, pct := help.TutorialProgressSummary(progress.Tutorials)
	return HelpIndex{
		Version:        Version,
		Topics:         help.Catalog(),
		Sections:       help.Sections(),
		FAQ:            help.FAQList(),
		FAQCategories:  help.FAQCategories(),
		Shortcuts:      help.Shortcuts(),
		ShortcutGroups: help.ShortcutGroups(),
		Tutorials:      help.Tutorials(),
		Context:        help.ContextEntries(),
		Progress:       progress,
		Summary: HelpProgressSummary{
			Completed: completed,
			Total:     total,
			Pct:       pct,
		},
		Changelog: help.Latest(help.LoadChangelog(a.helpExeDir()), 5),
	}
}

// SearchHelp busca en documentación y FAQ a la vez.
func (a *App) SearchHelp(query string, limit int) []help.Hit {
	return help.Search(query, limit)
}

// SearchHelpTopics limita la búsqueda al catálogo de documentación.
func (a *App) SearchHelpTopics(query string, limit int) []help.Hit {
	return help.SearchTopics(query, limit)
}

// GetHelpTopic devuelve un tema concreto (found=false si el id no existe).
func (a *App) GetHelpTopic(id string) (help.Topic, bool) {
	return help.FindTopic(id)
}

// GetChangelog parsea CHANGELOG.md (limit<=0 → todas las versiones).
func (a *App) GetChangelog(limit int) []help.Release {
	return help.Latest(help.LoadChangelog(a.helpExeDir()), limit)
}

// GetHelpShortcuts devuelve el registro de atajos.
func (a *App) GetHelpShortcuts() []help.Shortcut {
	return help.Shortcuts()
}

// GetContextHelp devuelve la ayuda contextual de un elemento de la UI.
func (a *App) GetContextHelp(target string) ContextHelpResult {
	entry, ok := help.ContextHelpFor(target)
	return ContextHelpResult{Found: ok, Help: entry}
}

// GetHelpProgress devuelve el progreso persistido.
func (a *App) GetHelpProgress() help.Progress {
	return help.LoadProgress(a.helpProgressPath())
}

// SetTutorialStep registra el paso completado de un tutorial y lo persiste.
func (a *App) SetTutorialStep(tutorialID string, step int) []string {
	if _, ok := help.FindTutorial(tutorialID); !ok {
		return []string{fmt.Sprintf("unknown tutorial %q", tutorialID)}
	}
	progress := help.LoadProgress(a.helpProgressPath()).SetTutorialStep(tutorialID, step)
	if err := help.SaveProgress(a.helpProgressPath(), progress); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DismissHelpTip marca un tip como descartado (idempotente).
func (a *App) DismissHelpTip(id string) []string {
	if id == "" {
		return []string{"tip id is required"}
	}
	progress := help.LoadProgress(a.helpProgressPath()).DismissTip(id)
	if err := help.SaveProgress(a.helpProgressPath(), progress); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// SetHelpLastTopic recuerda el último tema visto (no-op si no existe).
func (a *App) SetHelpLastTopic(topicID string) []string {
	if _, ok := help.FindTopic(topicID); !ok {
		return []string{fmt.Sprintf("unknown topic %q", topicID)}
	}
	progress := help.LoadProgress(a.helpProgressPath()).SetLastTopic(topicID)
	if err := help.SaveProgress(a.helpProgressPath(), progress); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// ResetHelpProgress borra tutoriales vistos y tips descartados.
func (a *App) ResetHelpProgress() []string {
	if err := help.SaveProgress(a.helpProgressPath(), help.NewProgress()); err != nil {
		return []string{err.Error()}
	}
	return nil
}
