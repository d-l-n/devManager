// Package devtools implementa las herramientas de desarrollo integradas
// (Issue #68): file browser seguro (clamped al proyecto), lectura/escritura de
// archivos, búsqueda por nombre y contenido, y snippets persistentes.
package devtools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MaxFileSize acota lectura/escritura (1 MB: configs y scripts, no binarios).
const MaxFileSize = 1 << 20

// MaxResults acota búsquedas y listados.
const MaxResults = 500

// FileEntry es una fila del file browser.
type FileEntry struct {
	Name  string `json:"name"`
	Rel   string `json:"rel"` // ruta relativa al proyecto, con /
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// SearchResult es una coincidencia de búsqueda.
type SearchResult struct {
	Path    string `json:"path"`    // rel al proyecto
	Line    int    `json:"line"`    // 1-based; 0 = match de nombre
	Preview string `json:"preview"` // línea recortada a 160 chars
}

// Snippet es un comando/código reutilizable.
type Snippet struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Command  string `json:"command"`
	Tags     []string `json:"tags,omitempty"`
	CreatedAt string `json:"created_at"`
}

// clampRoot garantiza que path esté dentro de root (sin traversal). Devuelve
// la ruta absoluta unida o error si escapa del proyecto.
func clampRoot(root, rel string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	clean := filepath.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if clean == "." || clean == "" {
		return absRoot, nil
	}
	if filepath.IsAbs(clean) || strings.HasPrefix(clean, "..") || strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "\\") || filepath.VolumeName(clean) != "" || (len(clean) >= 2 && clean[1] == ':') {
		return "", fmt.Errorf("path %q escapes project root", rel)
	}
	full := filepath.Join(absRoot, clean)
	// Doble check tras Clean (symlinks no se resuelven: documentado).
	if !strings.HasPrefix(full, absRoot+string(filepath.Separator)) && full != absRoot {
		return "", fmt.Errorf("path %q escapes project root", rel)
	}
	return full, nil
}

// toRel convierte una ruta absoluta a relativa al root con separador /.
func toRel(root, full string) string {
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return full
	}
	return filepath.ToSlash(rel)
}

// ListDir lista el contenido de rel dentro del proyecto (dirs primero,
// luego archivos alfabético). "." lista la raíz.
func ListDir(root, rel string) ([]FileEntry, error) {
	full, err := clampRoot(root, rel)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(full)
	if err != nil {
		return nil, err
	}
	out := []FileEntry{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && name != ".env" {
			continue // oculta dotfiles salvo .env
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		child := filepath.ToSlash(filepath.Join(rel, name))
		if rel == "." || rel == "" {
			child = name
		}
		out = append(out, FileEntry{
			Name:  name,
			Rel:   child,
			IsDir: e.IsDir(),
			Size:  info.Size(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	if len(out) > MaxResults {
		out = out[:MaxResults]
	}
	return out, nil
}

// ReadFile lee un archivo del proyecto (tope MaxFileSize).
func ReadFile(root, rel string) (string, error) {
	full, err := clampRoot(root, rel)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(full)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s is a directory", rel)
	}
	if info.Size() > MaxFileSize {
		return "", fmt.Errorf("file too large (%d bytes, max %d)", info.Size(), MaxFileSize)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// WriteFile escribe un archivo dentro del proyecto (crea dirs padre).
func WriteFile(root, rel, content string) error {
	full, err := clampRoot(root, rel)
	if err != nil {
		return err
	}
	if len(content) > MaxFileSize {
		return fmt.Errorf("content too large (max %d bytes)", MaxFileSize)
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return err
	}
	return os.WriteFile(full, []byte(content), 0o644)
}

// SearchFiles busca por nombre (case-insensitive) y por contenido si query
// tiene ≥3 chars. Devuelve hasta MaxResults resultados (nombre primero).
func SearchFiles(root, query string) []SearchResult {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil
	}
	out := []SearchResult{}
	contentMode := len(query) >= 3
	lq := strings.ToLower(query)

	_ = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(out) >= MaxResults {
			return filepath.SkipAll
		}
		name := d.Name()
		if d.IsDir() {
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == ".venv" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") && name != ".env" {
			return nil
		}
		rel := toRel(absRoot, path)
		if strings.Contains(strings.ToLower(name), lq) {
			out = append(out, SearchResult{Path: rel, Line: 0, Preview: name})
			return nil
		}
		if !contentMode {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > MaxFileSize {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), lq) {
				preview := strings.TrimSpace(line)
				if len(preview) > 160 {
					preview = preview[:160]
				}
				out = append(out, SearchResult{Path: rel, Line: i + 1, Preview: preview})
				break // una coincidencia por archivo
			}
		}
		return nil
	})
	return out
}

// ---- Snippets (persistidos en %APPDATA%/devManager/snippets.json) ----

// SnippetStore persiste snippets como JSON.
type SnippetStore struct {
	path     string
	Snippets []Snippet `json:"snippets"`
}

// NewSnippetStore crea el store sobre path.
func NewSnippetStore(path string) *SnippetStore {
	return &SnippetStore{path: path, Snippets: []Snippet{}}
}

// Load lee snippets.json; ausente/corrupto → vacío.
func (s *SnippetStore) Load() {
	s.Snippets = []Snippet{}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var st SnippetStore
	if json.Unmarshal(data, &st) == nil && st.Snippets != nil {
		s.Snippets = st.Snippets
	}
}

// Save persiste snippets.json.
func (s *SnippetStore) Save() error {
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if dir := s.path; dir != "" {
		if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(s.path, out, 0o644)
}

// SaveSnippet inserta (ID vacío) o actualiza (upsert). Valida nombre/comando.
func (s *SnippetStore) SaveSnippet(sn Snippet) []string {
	if strings.TrimSpace(sn.Name) == "" {
		return []string{"name cannot be empty"}
	}
	if strings.TrimSpace(sn.Command) == "" {
		return []string{"command cannot be empty"}
	}
	replaced := false
	if sn.ID != "" {
		for i := range s.Snippets {
			if s.Snippets[i].ID == sn.ID {
				s.Snippets[i] = sn
				replaced = true
				break
			}
		}
	}
	if !replaced {
		if sn.ID == "" {
			sn.ID = newSnippetID(s.Snippets, slugify(sn.Name))
		}
		if sn.CreatedAt == "" {
			sn.CreatedAt = nowRFC3339()
		}
		s.Snippets = append(s.Snippets, sn)
	}
	if err := s.Save(); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DeleteSnippet elimina por ID.
func (s *SnippetStore) DeleteSnippet(id string) []string {
	out := make([]Snippet, 0, len(s.Snippets))
	removed := false
	for _, sn := range s.Snippets {
		if sn.ID == id {
			removed = true
			continue
		}
		out = append(out, sn)
	}
	if !removed {
		return []string{fmt.Sprintf("snippet %q does not exist", id)}
	}
	s.Snippets = out
	if err := s.Save(); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// nowRFC3339 sella CreatedAt de snippets nuevos.
func nowRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

func newSnippetID(existing []Snippet, base string) string {
	if base == "" {
		base = "snippet"
	}
	taken := map[string]bool{}
	for _, sn := range existing {
		taken[sn.ID] = true
	}
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s-%d", base, i)
		if !taken[c] {
			return c
		}
	}
}

// slugify normaliza a [a-z0-9-] para IDs legibles.
func slugify(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
