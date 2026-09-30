// Package env implementa parsing/serialización dotenv (Fase 2 #67).
// Stdlib only. Nunca loguea valores: este package no loguea nada.
package env

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// keyRe valida nombres de variable: ^[A-Z_][A-Z0-9_]*$.
var keyRe = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// ValidateKey valida un nombre de variable dotenv.
func ValidateKey(key string) error {
	if !keyRe.MatchString(key) {
		return fmt.Errorf("invalid variable name %q (use ^[A-Z_][A-Z0-9_]*$)", key)
	}
	return nil
}

// Parse parsea contenido dotenv: KEY=val, quotes simples/dobles, comentarios
// `#`, prefijo `export`, ignora líneas vacías. Los errores citan línea y key,
// nunca valores.
func Parse(data []byte) (map[string]string, error) {
	out := map[string]string{}
	lines := strings.Split(string(data), "\n")
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		rest := line
		if rest == "export" {
			return nil, fmt.Errorf("line %d: missing '='", i+1)
		}
		if strings.HasPrefix(rest, "export ") || strings.HasPrefix(rest, "export\t") {
			rest = strings.TrimSpace(rest[len("export"):])
		}
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return nil, fmt.Errorf("line %d: missing '='", i+1)
		}
		key := strings.TrimSpace(rest[:eq])
		if err := ValidateKey(key); err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		val, err := parseValue(strings.TrimSpace(rest[eq+1:]))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		out[key] = val
	}
	return out, nil
}

// parseValue interpreta un valor: quoteado (doble con escapes, simple literal)
// o crudo (recorta comentario `#` precedido por espacio/tab).
func parseValue(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	switch s[0] {
	case '"':
		var b strings.Builder
		escaped := false
		closed := false
		j := 1
		for ; j < len(s); j++ {
			c := s[j]
			if escaped {
				switch c {
				case 'n':
					b.WriteByte('\n')
				case 'r':
					b.WriteByte('\r')
				case 't':
					b.WriteByte('\t')
				case '"':
					b.WriteByte('"')
				case '\\':
					b.WriteByte('\\')
				default:
					b.WriteByte(c)
				}
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == '"' {
				closed = true
				j++
				break
			}
			b.WriteByte(c)
		}
		if !closed {
			return "", fmt.Errorf("unterminated double-quoted value")
		}
		tail := strings.TrimSpace(s[j:])
		if tail != "" && !strings.HasPrefix(tail, "#") {
			return "", fmt.Errorf("unexpected content after closing quote")
		}
		return b.String(), nil
	case '\'':
		end := strings.Index(s[1:], "'")
		if end < 0 {
			return "", fmt.Errorf("unterminated single-quoted value")
		}
		tail := strings.TrimSpace(s[1+end+1:])
		if tail != "" && !strings.HasPrefix(tail, "#") {
			return "", fmt.Errorf("unexpected content after closing quote")
		}
		return s[1 : 1+end], nil
	default:
		if idx := indexComment(s); idx >= 0 {
			s = s[:idx]
		}
		return strings.TrimSpace(s), nil
	}
}

// indexComment ubica el `#` que inicia comentario: al inicio o precedido por
// espacio/tab (un `a#b` crudo conserva el `#`).
func indexComment(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '#' && (i == 0 || s[i-1] == ' ' || s[i-1] == '\t') {
			return i
		}
	}
	return -1
}

// Serialize ordena por key y quotea (dobles con escapes) cuando el valor lo
// requiere: vacío, espacios, `#`, quotes, `=` o saltos de línea.
func Serialize(vars map[string]string) string {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(quoteValue(vars[k]))
		b.WriteByte('\n')
	}
	return b.String()
}

func needsQuote(s string) bool {
	if s == "" {
		return true
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ' ', '\t', '#', '"', '\'', '=', '\n', '\r':
			return true
		}
	}
	return false
}

func quoteValue(s string) string {
	if !needsQuote(s) {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '\\':
			b.WriteString("\\\\")
		case '"':
			b.WriteString("\\\"")
		case '\n':
			b.WriteString("\\n")
		case '\r':
			b.WriteString("\\r")
		case '\t':
			b.WriteString("\\t")
		default:
			b.WriteRune(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Merge combina base+override (override gana). No muta las entradas.
func Merge(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
