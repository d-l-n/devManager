package help

import (
	"strings"
	"testing"
)

func TestSearchTituloGanaAlCuerpo(t *testing.T) {
	hits := SearchTopics("environments", 0)
	if len(hits) == 0 {
		t.Fatal("sin resultados para environments")
	}
	if hits[0].ID != "environments" {
		t.Errorf("primero = %q, want environments (%+v)", hits[0].ID, hits)
	}
	if hits[0].Kind != "topic" {
		t.Errorf("kind = %q", hits[0].Kind)
	}
	if hits[0].Snippet == "" {
		t.Error("el hit debería traer snippet")
	}
}

func TestSearchEncuentraPorKeywordYSeccion(t *testing.T) {
	for _, query := range []string{"k6", "visual regression", "flaky", "secrets", "backups"} {
		hits := SearchTopics(query, 5)
		if len(hits) == 0 {
			t.Errorf("sin resultados para %q", query)
			continue
		}
		if hits[0].Score <= 0 {
			t.Errorf("score inválido para %q: %+v", query, hits[0])
		}
	}
}

func TestSearchVacioYSinResultados(t *testing.T) {
	if hits := SearchTopics("", 5); len(hits) != 0 {
		t.Errorf("query vacía = %+v", hits)
	}
	if hits := SearchTopics("a", 5); len(hits) != 0 {
		t.Errorf("tokens de 1 carácter se ignoran: %+v", hits)
	}
	if hits := SearchTopics("zzzzzzzz", 5); len(hits) != 0 {
		t.Errorf("query sin match = %+v", hits)
	}
}

func TestSearchLimiteYOrdenEstable(t *testing.T) {
	hits := SearchTopics("playwright", 1)
	if len(hits) != 1 {
		t.Fatalf("limit no aplicado: %+v", hits)
	}
	all := SearchTopics("playwright", 0)
	for i := 1; i < len(all); i++ {
		if all[i-1].Score < all[i].Score {
			t.Fatalf("resultados sin ordenar por score: %+v", all)
		}
		if all[i-1].Score == all[i].Score && all[i-1].Title > all[i].Title {
			t.Errorf("empate sin orden alfabético: %+v", []Hit{all[i-1], all[i]})
		}
	}
}

func TestSearchGlobalYFAQ(t *testing.T) {
	hits := Search("settings.json", 0)
	kindSeen := map[string]bool{}
	for _, h := range hits {
		kindSeen[h.Kind] = true
	}
	if len(hits) == 0 {
		t.Fatal("sin resultados buscando settings.json")
	}
	faqHits := SearchFAQ("port", 0)
	if len(faqHits) == 0 {
		t.Fatal("sin resultados FAQ para port")
	}
	for _, h := range faqHits {
		if h.Kind != "faq" {
			t.Errorf("SearchFAQ devolvió kind %q", h.Kind)
		}
	}
	if !kindSeen["faq"] || !kindSeen["topic"] {
		t.Errorf("search global debería mezclar documentación y FAQ: %+v", kindSeen)
	}
}

func TestSnippetRecorta(t *testing.T) {
	long := strings.Repeat("palabra ", 60)
	snippet := snippetFor(long, []string{"palabra"})
	if len([]rune(snippet)) > 160 {
		t.Errorf("snippet demasiado largo: %d runas", len([]rune(snippet)))
	}
	if !strings.HasSuffix(snippet, "…") {
		t.Errorf("snippet recortado debería terminar en elipsis: %q", snippet)
	}
	if got := snippetFor("", nil); got != "" {
		t.Errorf("cuerpo vacío = %q", got)
	}
	if got := snippetFor("## Solo titulo\n- item corto", []string{"item"}); got != "item corto" {
		t.Errorf("snippet = %q (no debe incluir marcado)", got)
	}
}

func TestTokenize(t *testing.T) {
	got := tokenize("  Playwright, TESTS! playwright ")
	want := []string{"playwright", "tests"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestFAQIntegridad(t *testing.T) {
	list := FAQList()
	if len(list) < 10 {
		t.Fatalf("FAQ demasiado corta: %d", len(list))
	}
	seen := map[string]bool{}
	topics := map[string]bool{}
	for _, topic := range Catalog() {
		topics[topic.ID] = true
	}
	for i, faq := range list {
		if faq.ID == "" || faq.Question == "" || faq.Answer == "" || faq.Category == "" {
			t.Errorf("FAQ con campos vacíos: %+v", faq)
		}
		if seen[faq.ID] {
			t.Errorf("FAQ duplicada: %q", faq.ID)
		}
		seen[faq.ID] = true
		if faq.TopicID != "" && !topics[faq.TopicID] {
			t.Errorf("FAQ %q apunta al tema inexistente %q", faq.ID, faq.TopicID)
		}
		if i > 0 && faq.Category < list[i-1].Category {
			t.Errorf("FAQ desordenada por categoría: %v", list)
		}
	}
	categories := FAQCategories()
	if len(categories) == 0 {
		t.Error("sin categorías de FAQ")
	}
	if _, ok := FindFAQ("secrets-storage"); !ok {
		t.Error("FindFAQ no encuentra una entrada conocida")
	}
	if _, ok := FindFAQ("nope"); ok {
		t.Error("FindFAQ no debería inventar entradas")
	}
}

func TestShortcutsIntegridad(t *testing.T) {
	list := Shortcuts()
	if len(list) < 20 {
		t.Fatalf("registro de atajos demasiado corto: %d", len(list))
	}
	for _, s := range list {
		if s.Keys == "" || s.Action == "" || s.Context == "" || s.Group == "" {
			t.Errorf("atajo incompleto: %+v", s)
		}
	}
	// F1 y Ctrl+, deben existir: el issue los pide explícitamente.
	if _, ok := FindShortcut("Open this help window"); !ok {
		t.Error("falta el atajo F1 de ayuda")
	}
	f1, _ := FindShortcut("Open this help window")
	if f1.Keys != "F1" {
		t.Errorf("atajo de ayuda = %q, want F1", f1.Keys)
	}
	if _, ok := FindShortcut("Open Settings"); !ok {
		t.Error("falta el atajo Ctrl+, de settings")
	}
	if len(ShortcutGroups()) == 0 {
		t.Error("sin grupos de atajos")
	}
}
