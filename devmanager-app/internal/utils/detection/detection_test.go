package detection

import "testing"

func TestExtractPortFromLog(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"Local:   http://localhost:5174/", 5174},
		{"ready on http://127.0.0.1:3000", 3000},
		{"Network: http://192.168.1.5:8080/", 8080},
		{"listening on http://[::1]:4200/", 4200},
		{"http://0.0.0.0:9000 active", 9000},
		{"LOCAL: HTTP://LOCALHOST:7777/", 7777}, // case-insensitive
		{"sin url aqui", 0},
		{"", 0},
		{"https://example.com/path", 0}, // dominio externo sin puerto no matchea
	}
	for _, tc := range cases {
		if got := ExtractPortFromLog(tc.line); got != tc.want {
			t.Errorf("ExtractPortFromLog(%q) = %d, want %d", tc.line, got, tc.want)
		}
	}
}

func TestExtractPortOutOfRangeRejected(t *testing.T) {
	// La regex captura hasta 5 dígitos pero >65535 se rechaza.
	if got := ExtractPortFromLog("http://localhost:99999/"); got != 0 {
		t.Errorf("puerto >65535 debe devolver 0, got %d", got)
	}
}
