package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newHelpTestApp reutiliza el App de tests con settingsPath temporal para que
// help.json nunca se escriba en %APPDATA% real.
func newHelpTestApp(t *testing.T) *App {
	t.Helper()
	return newTestadvApp(t, t.TempDir())
}

func TestGetHelpIndex(t *testing.T) {
	a := newHelpTestApp(t)
	index := a.GetHelpIndex()

	if len(index.Topics) < 20 {
		t.Errorf("topics = %d", len(index.Topics))
	}
	if len(index.Sections) == 0 || index.Sections[0] != "Getting started" {
		t.Errorf("sections = %v", index.Sections)
	}
	if len(index.FAQ) == 0 || len(index.FAQCategories) == 0 {
		t.Errorf("FAQ vacía: %d/%d", len(index.FAQ), len(index.FAQCategories))
	}
	if len(index.Shortcuts) == 0 || len(index.ShortcutGroups) == 0 {
		t.Errorf("shortcuts vacíos")
	}
	if len(index.Tutorials) == 0 || len(index.Context) == 0 {
		t.Errorf("tutoriales/contexto vacíos")
	}
	if index.Version == "" {
		t.Error("version vacía")
	}
	if index.Summary.Total != len(index.Tutorials) || index.Summary.Completed != 0 {
		t.Errorf("summary = %+v", index.Summary)
	}
	if len(index.Changelog) == 0 {
		t.Error("changelog vacío (debería leer el CHANGELOG.md del repo en dev)")
	}
	if len(index.Changelog) > 5 {
		t.Errorf("changelog = %d releases (máximo 5)", len(index.Changelog))
	}
}

func TestHelpSearchYTema(t *testing.T) {
	a := newHelpTestApp(t)
	hits := a.SearchHelp("playwright", 3)
	if len(hits) == 0 || len(hits) > 3 {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].ID == "" || hits[0].Title == "" {
		t.Errorf("hit incompleto: %+v", hits[0])
	}
	if topicsOnly := a.SearchHelpTopics("playwright", 3); len(topicsOnly) == 0 {
		t.Error("SearchHelpTopics sin resultados")
	}
	if hits := a.SearchHelp("", 3); len(hits) != 0 {
		t.Errorf("query vacía = %+v", hits)
	}

	topic, ok := a.GetHelpTopic("servers")
	if !ok || topic.ID != "servers" || topic.Body == "" {
		t.Errorf("topic = %+v ok=%v", topic, ok)
	}
	if _, ok := a.GetHelpTopic("nope"); ok {
		t.Error("tema inexistente debería dar ok=false")
	}
}

func TestHelpContextual(t *testing.T) {
	a := newHelpTestApp(t)
	got := a.GetContextHelp("#btn-start")
	if !got.Found || got.Help.TopicID != "servers" || got.Help.Text == "" {
		t.Errorf("contexto btn-start = %+v", got)
	}
	if got.Help.Keys != "F5" {
		t.Errorf("atajo sugerido = %q", got.Help.Keys)
	}
	if missing := a.GetContextHelp("#no-existe"); missing.Found {
		t.Errorf("target inexistente = %+v", missing)
	}
}

func TestHelpChangelogLimit(t *testing.T) {
	a := newHelpTestApp(t)
	all := a.GetChangelog(0)
	if len(all) == 0 {
		t.Skip("CHANGELOG.md no accesible desde el CWD del test")
	}
	if two := a.GetChangelog(2); len(two) != 2 {
		t.Errorf("limit 2 = %d releases", len(two))
	}
	if len(all) < len(a.GetHelpIndex().Changelog) {
		t.Error("el índice no debería traer más releases que GetChangelog(0)")
	}
}

func TestHelpProgressPersistencia(t *testing.T) {
	a := newHelpTestApp(t)
	path := a.helpProgressPath()
	if !strings.Contains(path, "help.json") {
		t.Fatalf("ruta de progreso = %q", path)
	}

	if errs := a.SetTutorialStep("first-project", 1); len(errs) != 0 {
		t.Fatalf("set step: %v", errs)
	}
	progress := a.GetHelpProgress()
	entry, ok := progress.Tutorials["first-project"]
	if !ok || entry.Step != 1 || entry.Completed {
		t.Fatalf("progreso = %+v (ok=%v)", entry, ok)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("help.json no se escribió: %v", err)
	}

	if errs := a.SetTutorialStep("no-existe", 0); len(errs) == 0 {
		t.Error("tutorial inexistente debería devolver error")
	}
	if errs := a.SetHelpLastTopic("themes"); len(errs) != 0 {
		t.Fatalf("last topic: %v", errs)
	}
	if errs := a.SetHelpLastTopic("no-existe"); len(errs) == 0 {
		t.Error("tema inexistente debería devolver error")
	}
	if errs := a.DismissHelpTip("tip-x"); len(errs) != 0 {
		t.Fatalf("dismiss: %v", errs)
	}
	if errs := a.DismissHelpTip("tip-x"); len(errs) != 0 {
		t.Fatalf("dismiss repetido: %v", errs)
	}
	if errs := a.DismissHelpTip(""); len(errs) == 0 {
		t.Error("tip sin id debería fallar")
	}

	after := a.GetHelpProgress()
	if after.LastTopic != "themes" || len(after.DismissedTips) != 1 {
		t.Errorf("progreso tras editar = %+v", after)
	}

	if errs := a.ResetHelpProgress(); len(errs) != 0 {
		t.Fatalf("reset: %v", errs)
	}
	cleared := a.GetHelpProgress()
	if cleared.LastTopic != "" || len(cleared.Tutorials) != 0 || len(cleared.DismissedTips) != 0 {
		t.Errorf("reset incompleto = %+v", cleared)
	}
}

func TestHelpIndexReflejaProgreso(t *testing.T) {
	a := newHelpTestApp(t)
	tutorial, ok := a.GetHelpTopic("environments")
	if !ok || tutorial.ID == "" {
		t.Fatal("catálogo inesperado")
	}
	tutorials := a.GetHelpIndex().Tutorials
	if len(tutorials) == 0 {
		t.Fatal("sin tutoriales")
	}
	// Completa todos los pasos del primer tutorial.
	for i := range tutorials[0].Steps {
		if errs := a.SetTutorialStep(tutorials[0].ID, i); len(errs) != 0 {
			t.Fatalf("paso %d: %v", i, errs)
		}
	}
	summary := a.GetHelpIndex().Summary
	if summary.Completed != 1 || summary.Pct <= 0 {
		t.Errorf("summary = %+v", summary)
	}
	if summary.Pct >= 100 && summary.Total > 1 {
		t.Errorf("un tutorial de %d no puede dar 100%%: %+v", summary.Total, summary)
	}
}

func TestHelpProgressPathJuntoASettings(t *testing.T) {
	a := newHelpTestApp(t)
	want := filepath.Join(filepath.Dir(a.settingsPath), "help.json")
	if got := a.helpProgressPath(); got != want {
		t.Errorf("helpProgressPath = %q, want %q", got, want)
	}
}
