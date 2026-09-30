package help

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTutorialsIntegridad(t *testing.T) {
	list := Tutorials()
	if len(list) < 4 {
		t.Fatalf("tutoriales = %d, want >= 4", len(list))
	}
	topics := map[string]bool{}
	for _, topic := range Catalog() {
		topics[topic.ID] = true
	}
	targets := map[string]bool{}
	for _, target := range ContextTargets() {
		targets[target] = true
	}
	seen := map[string]bool{}
	for _, tutorial := range list {
		if tutorial.ID == "" || tutorial.Title == "" || tutorial.Summary == "" {
			t.Errorf("tutorial incompleto: %+v", tutorial)
		}
		if seen[tutorial.ID] {
			t.Errorf("tutorial duplicado: %q", tutorial.ID)
		}
		seen[tutorial.ID] = true
		if len(tutorial.Steps) < 3 {
			t.Errorf("tutorial %q con %d pasos (mínimo 3)", tutorial.ID, len(tutorial.Steps))
		}
		for i, step := range tutorial.Steps {
			if step.Index != i {
				t.Errorf("%s paso %d con Index %d", tutorial.ID, i, step.Index)
			}
			if step.Title == "" || step.Body == "" {
				t.Errorf("%s paso %d incompleto", tutorial.ID, i)
			}
			if step.TopicID != "" && !topics[step.TopicID] {
				t.Errorf("%s paso %d apunta al tema inexistente %q", tutorial.ID, i, step.TopicID)
			}
			if step.Target != "" && !targets[normalizeTarget(step.Target)] {
				t.Errorf("%s paso %d apunta al target sin ayuda contextual %q", tutorial.ID, i, step.Target)
			}
		}
	}
	if _, ok := FindTutorial("first-project"); !ok {
		t.Error("FindTutorial no encuentra first-project")
	}
	if _, ok := FindTutorial("nope"); ok {
		t.Error("FindTutorial no debería inventar tutoriales")
	}
	if len(StepsOfTopic("servers")) == 0 {
		t.Error("StepsOfTopic debería enlazar servers con algún tutorial")
	}
}

func TestTutorialProgreso(t *testing.T) {
	tutorial, _ := FindTutorial("first-project")
	total := len(tutorial.Steps)

	empty := TutorialProgress{Step: -1, Total: total}
	if next := NextStep(tutorial, empty); next != 0 {
		t.Errorf("next desde -1 = %d, want 0", next)
	}
	if IsComplete(tutorial, empty) {
		t.Error("progreso inicial no está completo")
	}
	if pct := CompletionPct(tutorial, empty); pct != 0 {
		t.Errorf("pct inicial = %v", pct)
	}

	last := TutorialProgress{Step: total - 1, Total: total}
	if !IsComplete(tutorial, last) {
		t.Error("último paso debería estar completo")
	}
	if pct := CompletionPct(tutorial, last); pct != 100 {
		t.Errorf("pct final = %v", pct)
	}
	if next := NextStep(tutorial, last); next != total {
		t.Errorf("next al final = %d, want %d", next, total)
	}

	mid := TutorialProgress{Step: 1, Total: total}
	if pct := CompletionPct(tutorial, mid); pct != 50 {
		t.Errorf("pct a mitad = %v, want 50", pct)
	}

	// Índices fuera de rango se acotan.
	if got := NormalizeStep(tutorial, 99); got != total-1 {
		t.Errorf("NormalizeStep(99) = %d", got)
	}
	if got := NormalizeStep(tutorial, -5); got != -1 {
		t.Errorf("NormalizeStep(-5) = %d", got)
	}
	if pct := CompletionPct(Tutorial{}, TutorialProgress{Step: 0}); pct != 0 {
		t.Errorf("tutorial vacío = %v", pct)
	}
}

func TestProgressResumen(t *testing.T) {
	progress := NewProgress()
	completed, total, pct := TutorialProgressSummary(progress.Tutorials)
	if completed != 0 || total != len(Tutorials()) || pct != 0 {
		t.Errorf("resumen inicial = %d/%d %v", completed, total, pct)
	}
	first, _ := FindTutorial("first-project")
	for i := range first.Steps {
		progress = progress.SetTutorialStep(first.ID, i)
	}
	completed, total, pct = TutorialProgressSummary(progress.Tutorials)
	if completed != 1 || total != len(Tutorials()) {
		t.Errorf("resumen = %d/%d", completed, total)
	}
	if pct <= 0 || pct > 100 {
		t.Errorf("pct = %v", pct)
	}
	if ids := CompletedTutorials(progress.Tutorials); len(ids) != 1 || ids[0] != first.ID {
		t.Errorf("completed = %v", ids)
	}
}

func TestSetTutorialStepAcota(t *testing.T) {
	progress := NewProgress()
	progress = progress.SetTutorialStep("first-project", 99)
	entry := progress.Tutorials["first-project"]
	tutorial, _ := FindTutorial("first-project")
	if entry.Step != len(tutorial.Steps)-1 || !entry.Completed {
		t.Errorf("progreso acotado = %+v", entry)
	}
	if entry.Total != len(tutorial.Steps) {
		t.Errorf("total = %d", entry.Total)
	}
	if entry.UpdatedAt.IsZero() {
		t.Error("debería registrar la fecha de actualización")
	}
	// Tutorial inexistente: no-op.
	before := progress.Tutorials["nope"]
	progress = progress.SetTutorialStep("nope", 0)
	if progress.Tutorials["nope"] != before {
		t.Error("un tutorial inexistente no debe crear progreso")
	}
}

func TestProgressTipsYUltimoTema(t *testing.T) {
	progress := NewProgress()
	progress = progress.DismissTip("tip-a").DismissTip("tip-a")
	if len(progress.DismissedTips) != 1 {
		t.Errorf("DismissTip debe ser idempotente: %v", progress.DismissedTips)
	}
	if !progress.IsTipDismissed("tip-a") || progress.IsTipDismissed("tip-b") {
		t.Error("IsTipDismissed inconsistente")
	}
	progress = progress.SetLastTopic("environments")
	if progress.LastTopic != "environments" {
		t.Errorf("lastTopic = %q", progress.LastTopic)
	}
	progress = progress.SetLastTopic("no-existe")
	if progress.LastTopic != "environments" {
		t.Errorf("un tema inexistente no debe sobrescribir lastTopic: %q", progress.LastTopic)
	}
	reset := progress.Reset()
	if reset.LastTopic != "" || len(reset.Tutorials) != 0 || reset.SchemaVersion != CurrentProgressSchema {
		t.Errorf("reset = %+v", reset)
	}
}

func TestProgressStorePersistencia(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "help.json")
	progress := NewProgress().
		SetTutorialStep("first-project", 1).
		DismissTip("tip-a").
		SetLastTopic("themes")
	if err := SaveProgress(path, progress); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	loaded := LoadProgress(path)
	if loaded.LastTopic != "themes" || len(loaded.DismissedTips) != 1 {
		t.Errorf("round trip = %+v", loaded)
	}
	if entry := loaded.Tutorials["first-project"]; entry.Step != 1 || entry.Completed {
		t.Errorf("progreso = %+v", entry)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("quedó un temporal: %s", e.Name())
		}
	}

	// Tolerancia a ausencia y corrupción.
	if got := LoadProgress(filepath.Join(t.TempDir(), "nope.json")); got.Tutorials == nil {
		t.Error("fichero ausente debe dar progreso usable")
	}
	corrupt := filepath.Join(t.TempDir(), "roto.json")
	if err := os.WriteFile(corrupt, []byte("{ no json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadProgress(corrupt)
	if got.Tutorials == nil || len(got.Tutorials) != 0 {
		t.Errorf("fichero corrupto = %+v", got)
	}
	if got.SchemaVersion != CurrentProgressSchema {
		t.Errorf("schemaVersion = %d", got.SchemaVersion)
	}
}

func TestProgressStoreSellaFecha(t *testing.T) {
	progress := NewProgress().SetTutorialStep("run-tests", 0)
	if progress.LastInteraction.IsZero() || time.Since(progress.LastInteraction) > time.Minute {
		t.Errorf("lastInteraction = %v", progress.LastInteraction)
	}
}
