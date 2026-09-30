package devtools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func tempProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{\"name\":\"x\"}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "src", "app.js"), []byte("const port = 5173;\nconsole.log('hola');\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "lib", "index.js"), []byte("IGNORED"), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// ---- clampRoot ----

func TestClampRootRechazaTraversal(t *testing.T) {
	if _, err := clampRoot(t.TempDir(), "../escapes"); err == nil {
		t.Fatal("../ debe ser rechazado")
	}
	if _, err := clampRoot(t.TempDir(), "C:/absoluto"); err == nil {
		t.Fatal("ruta absoluta debe ser rechazada")
	}
	full, err := clampRoot(t.TempDir(), "src/../package.json")
	if err != nil || !strings.HasSuffix(filepath.ToSlash(full), "/package.json") {
		t.Fatalf("clean interno debe aceptar src/..: %v %s", err, full)
	}
}

// ---- ListDir ----

func TestListDirOrdenaYFiltra(t *testing.T) {
	root := tempProject(t)
	entries, err := ListDir(root, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entradas (node_modules, package.json, src), got %d: %+v", len(entries), entries)
	}
	if !entries[0].IsDir || !entries[1].IsDir {
		t.Fatalf("dirs primero: %+v", entries)
	}
	if entries[2].IsDir || entries[2].Name != "package.json" {
		t.Fatalf("archivos tras dirs: %+v", entries[2])
	}
	// Dirs primero: node_modules y src preceden a package.json.
}

func TestListDirSubcarpetaYError(t *testing.T) {
	root := tempProject(t)
	entries, err := ListDir(root, "src")
	if err != nil || len(entries) != 1 || entries[0].Name != "app.js" {
		t.Fatalf("src listing: %v %+v", err, entries)
	}
	if _, err := ListDir(root, "../../etc"); err == nil {
		t.Fatal("traversal debe fallar")
	}
}

// ---- Read/Write ----

func TestReadWriteFileRoundTrip(t *testing.T) {
	root := tempProject(t)
	if err := WriteFile(root, "config/app.conf", "port=8080\n"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFile(root, "config/app.conf")
	if err != nil || got != "port=8080\n" {
		t.Fatalf("round trip: %v %q", err, got)
	}
	if err := WriteFile(root, "../../pwned.txt", "x"); err == nil {
		t.Fatal("write traversal debe fallar")
	}
	if _, err := ReadFile(root, "../../pwned.txt"); err == nil {
		t.Fatal("read traversal debe fallar")
	}
}

func TestReadFileTope(t *testing.T) {
	root := t.TempDir()
	big := strings.Repeat("a", MaxFileSize+1)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(root, "big.txt"); err == nil {
		t.Fatal("archivo > 1MB debe fallar")
	}
}

// ---- Search ----

func TestSearchFilesNombreYContenido(t *testing.T) {
	root := tempProject(t)
	// Por nombre.
	got := SearchFiles(root, "package")
	if len(got) != 1 || got[0].Path != "package.json" || got[0].Line != 0 {
		t.Fatalf("por nombre: %+v", got)
	}
	// Por contenido (5173 está en src/app.js).
	got = SearchFiles(root, "5173")
	if len(got) != 1 || got[0].Path != "src/app.js" || got[0].Line != 1 {
		t.Fatalf("por contenido: %+v", got)
	}
	// node_modules excluido.
	SearchFiles(root, "IGNORED")
	for _, r := range SearchFiles(root, "ignored") {
		if strings.Contains(r.Path, "node_modules") {
			t.Fatalf("node_modules debe excluirse: %+v", r)
		}
	}
	// Query vacío → nil.
	if SearchFiles(root, "") != nil {
		t.Fatal("query vacío debe devolver nil")
	}
}

// ---- Snippets ----

func TestSnippetStoreCRUDYPersistencia(t *testing.T) {
	path := filepath.Join(t.TempDir(), "snippets.json")
	s := NewSnippetStore(path)
	s.Load()
	if errs := s.SaveSnippet(Snippet{Name: "Run dev", Command: "npm run dev"}); len(errs) != 0 {
		t.Fatalf("save: %v", errs)
	}
	if len(s.Snippets) != 1 || s.Snippets[0].ID != "run-dev" {
		t.Fatalf("snippet mal guardado: %+v", s.Snippets)
	}
	if s.Snippets[0].CreatedAt == "" {
		t.Fatal("created_at debe sellarse")
	}
	if _, err := time.Parse(time.RFC3339, s.Snippets[0].CreatedAt); err != nil {
		t.Fatalf("created_at no RFC3339: %v", err)
	}
	// Update.
	up := s.Snippets[0]
	up.Command = "npm run dev -- --host"
	if errs := s.SaveSnippet(up); len(errs) != 0 {
		t.Fatalf("update: %v", errs)
	}
	// Recarga.
	s2 := NewSnippetStore(path)
	s2.Load()
	if len(s2.Snippets) != 1 || s2.Snippets[0].Command != "npm run dev -- --host" {
		t.Fatalf("persistencia: %+v", s2.Snippets)
	}
	// Delete + doble delete.
	if errs := s2.DeleteSnippet("run-dev"); len(errs) != 0 {
		t.Fatalf("delete: %v", errs)
	}
	if errs := s2.DeleteSnippet("run-dev"); len(errs) == 0 {
		t.Fatal("segundo delete debe fallar")
	}
}

func TestSnippetValidacion(t *testing.T) {
	s := NewSnippetStore(filepath.Join(t.TempDir(), "s.json"))
	if errs := s.SaveSnippet(Snippet{Name: "", Command: "x"}); len(errs) == 0 {
		t.Fatal("nombre vacío debe fallar")
	}
	if errs := s.SaveSnippet(Snippet{Name: "x", Command: "  "}); len(errs) == 0 {
		t.Fatal("comando vacío debe fallar")
	}
}
