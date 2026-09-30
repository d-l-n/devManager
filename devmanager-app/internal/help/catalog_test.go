package help

import (
	"strings"
	"testing"
)

func TestCatalogIntegridad(t *testing.T) {
	topics := Catalog()
	if len(topics) < 20 {
		t.Fatalf("catálogo sospechosamente corto: %d temas", len(topics))
	}
	byID := map[string]Topic{}
	for _, topic := range topics {
		if topic.ID == "" || topic.Title == "" || topic.Section == "" || topic.Summary == "" || topic.Body == "" {
			t.Errorf("tema con campos vacíos: %+v", topic)
		}
		if len(topic.Keywords) == 0 {
			t.Errorf("tema %q sin keywords (no se puede buscar por sinónimos)", topic.ID)
		}
		if _, dup := byID[topic.ID]; dup {
			t.Errorf("ID duplicado: %q", topic.ID)
		}
		byID[topic.ID] = topic
	}
	for _, topic := range topics {
		for _, related := range topic.Related {
			if _, ok := byID[related]; !ok {
				t.Errorf("tema %q referencia related inexistente %q", topic.ID, related)
			}
		}
	}
}

func TestCatalogOrdenaPorSeccion(t *testing.T) {
	sections := Sections()
	if len(sections) == 0 {
		t.Fatal("sin secciones")
	}
	rank := map[string]int{}
	for i, s := range SectionOrder {
		rank[s] = i
	}
	last := -1
	for _, s := range sections {
		if r, ok := rank[s]; ok {
			if r < last {
				t.Fatalf("secciones fuera de SectionOrder: %v", sections)
			}
			last = r
		}
	}
	if got := SectionOrder[0]; sections[0] != got {
		t.Errorf("primera sección = %q, want %q", sections[0], got)
	}
}

func TestFindTopic(t *testing.T) {
	if topic, ok := FindTopic("ENVIRONMENTS"); !ok || topic.ID != "environments" {
		t.Error("FindTopic debería ser case-insensitive y encontrar 'environments'")
	}
	if _, ok := FindTopic("nope"); ok {
		t.Error("tema inexistente no debería encontrado")
	}
}

// TestContextualIntegridad es el test más valioso del paquete: la ayuda
// contextual no puede apuntar a temas ni elementos inexistentes.
func TestContextualIntegridad(t *testing.T) {
	topics := map[string]bool{}
	for _, topic := range Catalog() {
		topics[topic.ID] = true
	}
	targets := ContextTargets()
	if len(targets) < 15 {
		t.Fatalf("targets contextuales = %d", len(targets))
	}
	for _, target := range targets {
		entry, ok := ContextHelpFor(target)
		if !ok {
			t.Errorf("target %q no resoluble", target)
			continue
		}
		if entry.Text == "" {
			t.Errorf("target %q sin texto", target)
		}
		if !topics[entry.TopicID] {
			t.Errorf("target %q apunta al tema inexistente %q", target, entry.TopicID)
		}
		if entry.Title == "" {
			t.Errorf("target %q sin título de tema", target)
		}
	}
	// Normalización de selectores reales.
	for target, want := range map[string]string{
		"#btn-start":        "btn-start",
		"btn-start":         "btn-start",
		"  #env-switcher  ": "env-switcher",
		"input[type=text]":  "input",
		"button.mono":       "button.mono",
	} {
		got := normalizeTarget(target)
		if got != want {
			t.Errorf("normalizeTarget(%q) = %q, want %q", target, got, want)
		}
	}
	if _, ok := ContextHelpFor("#boton-inexistente"); ok {
		t.Error("target desconocido no debería resolver")
	}
	if len(ContextEntries()) != len(targets) {
		t.Errorf("ContextEntries = %d, targets = %d", len(ContextEntries()), len(targets))
	}
}

func TestContextTopicsForTargets(t *testing.T) {
	byTopic := TopicsForTargets()
	if len(byTopic["environments"]) < 3 {
		t.Errorf("environments debería agrupar varios targets: %v", byTopic["environments"])
	}
	for topic, targets := range byTopic {
		if len(targets) == 0 {
			t.Errorf("tema %q sin targets", topic)
		}
	}
}

func TestCatalogBodiesUsanFormatoSoportado(t *testing.T) {
	for _, topic := range Catalog() {
		for _, line := range strings.Split(topic.Body, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(trimmed, "<") || strings.Contains(trimmed, "&lt;") {
				t.Errorf("tema %q contiene HTML crudo (el visor escapa texto): %q", topic.ID, trimmed)
			}
		}
	}
}
