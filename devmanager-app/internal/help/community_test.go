package help

import "testing"

func TestCommunityLinksIntegridad(t *testing.T) {
	links := KnownCommunityLinks()
	if len(links) < 3 {
		t.Fatalf("se esperaban al menos 3 enlaces, got %d", len(links))
	}
	seen := map[string]bool{}
	for _, l := range links {
		if l.ID == "" || l.Label == "" || l.URL == "" {
			t.Fatalf("enlace incompleto: %+v", l)
		}
		if l.URL[:8] != "https://" && l.URL[:7] != "http://" {
			t.Fatalf("URL no http(s): %s", l.URL)
		}
		if seen[l.ID] {
			t.Fatalf("id duplicado: %s", l.ID)
		}
		seen[l.ID] = true
	}
	// Los enlaces canónicos del repo deben existir.
	if !seen["repo"] || !seen["issues"] {
		t.Fatalf("faltan enlaces repo/issues: %v", seen)
	}
}
