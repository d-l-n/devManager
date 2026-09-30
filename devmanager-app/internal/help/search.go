package help

import (
	"sort"
	"strings"
)

// Hit es un resultado de búsqueda (topic o entrada de FAQ).
type Hit struct {
	Kind    string  `json:"kind"` // topic | faq
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Section string  `json:"section"`
	Score   float64 `json:"score"`
	Snippet string  `json:"snippet"`
}

// doc es la vista indexable común de topics y FAQs.
type doc struct {
	kind     string
	id       string
	title    string
	section  string
	keywords []string
	body     string
}

// SearchTopics busca en el catálogo de documentación.
func SearchTopics(query string, limit int) []Hit {
	return searchDocs(query, limit, topicDocs(), "topic")
}

// SearchFAQ busca sólo en las preguntas frecuentes.
func SearchFAQ(query string, limit int) []Hit {
	return searchDocs(query, limit, faqDocs(), "faq")
}

// Search busca en documentación + FAQ, ordenado por score.
func Search(query string, limit int) []Hit {
	docs := append(topicDocs(), faqDocs()...)
	return searchDocs(query, limit, docs, "")
}

func topicDocs() []doc {
	out := make([]doc, 0, len(topics))
	for _, t := range topics {
		out = append(out, doc{
			kind: "topic", id: t.ID, title: t.Title, section: t.Section,
			keywords: t.Keywords, body: t.Body,
		})
	}
	return out
}

func faqDocs() []doc {
	out := make([]doc, 0, len(faqs))
	for _, f := range faqs {
		out = append(out, doc{
			kind: "faq", id: f.ID, title: f.Question, section: f.Category,
			keywords: strings.Fields(strings.ToLower(f.Category + " " + f.Question)),
			body:     f.Answer,
		})
	}
	return out
}

// searchDocs puntúa cada doc: título 6, keywords 4, sección 2, cuerpo 1.
// La frase completa suma bonus (título 8, cuerpo 4). Empate → alfabético.
func searchDocs(query string, limit int, docs []doc, kindFilter string) []Hit {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return []Hit{}
	}
	phrase := strings.ToLower(strings.TrimSpace(query))
	hits := []Hit{}
	for _, d := range docs {
		if kindFilter != "" && d.kind != kindFilter {
			continue
		}
		title := strings.ToLower(d.title)
		body := strings.ToLower(d.body)
		keywords := strings.ToLower(strings.Join(d.keywords, " "))
		section := strings.ToLower(d.section)

		score := 0.0
		for _, token := range tokens {
			switch {
			case strings.Contains(title, token):
				score += 6
			case strings.Contains(keywords, token):
				score += 4
			case strings.Contains(section, token):
				score += 2
			case strings.Contains(body, token):
				score += 1
			}
		}
		if score == 0 {
			continue
		}
		if strings.Contains(title, phrase) {
			score += 8
		} else if strings.Contains(body, phrase) {
			score += 4
		}
		hits = append(hits, Hit{
			Kind:    d.kind,
			ID:      d.id,
			Title:   d.title,
			Section: d.section,
			Score:   score,
			Snippet: snippetFor(d.body, tokens),
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		return hits[i].Title < hits[j].Title
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// tokenize parte la query en tokens útiles (>= 2 caracteres, sin acentos).
func tokenize(query string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, raw := range strings.Fields(strings.ToLower(query)) {
		token := strings.Trim(raw, ".,;:!?()[]\"'`*")
		if len([]rune(token)) < 2 || seen[token] {
			continue
		}
		seen[token] = true
		out = append(out, token)
	}
	return out
}

// snippetFor devuelve la primera línea del cuerpo que menciona algún token
// (o la primera línea no vacía), recortada a 160 caracteres.
func snippetFor(body string, tokens []string) string {
	lines := []string{}
	for _, line := range strings.Split(body, "\n") {
		clean := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- "))
		if clean != "" && !strings.HasPrefix(clean, "##") {
			lines = append(lines, clean)
		}
	}
	if len(lines) == 0 {
		return ""
	}
	best := lines[0]
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, token := range tokens {
			if strings.Contains(lower, token) {
				best = line
				break
			}
		}
		if best != lines[0] {
			break
		}
	}
	return truncate(best, 160)
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return strings.TrimSpace(string(runes[:max-1])) + "…"
}
