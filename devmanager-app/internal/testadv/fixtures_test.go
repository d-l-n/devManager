package testadv

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestListFixtures(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "test-data")
	if err := os.MkdirAll(filepath.Join(dataDir, "usuarios"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "seed.json"), []byte(`{"a":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, ".oculto"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixDir := filepath.Join(dir, "tests", "fixtures")
	if err := os.MkdirAll(fixDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixDir, "user.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	sets := ListFixtures(dir)
	if len(sets) != 2 {
		t.Fatalf("sets = %+v, want 2 (test-data y tests/fixtures)", sets)
	}
	if sets[0].Dir != "test-data" || sets[1].Dir != "tests/fixtures" {
		t.Errorf("orden/dir = %+v", sets)
	}
	first := sets[0]
	if first.Count != 2 || first.Files != 1 {
		t.Errorf("count/files = %d/%d (el fichero oculto no cuenta)", first.Count, first.Files)
	}
	if first.Bytes != int64(len(`{"a":1}`)) {
		t.Errorf("bytes = %d", first.Bytes)
	}
	if first.Truncated {
		t.Error("set pequeño no debe estar truncado")
	}
	names := []string{first.Entries[0].Name, first.Entries[1].Name}
	if names[0] != "seed.json" || names[1] != "usuarios" {
		t.Errorf("entradas ordenadas = %+v", names)
	}
	if !first.Entries[1].IsDir || first.Entries[1].Size != 0 {
		t.Errorf("el directorio debe marcarse IsDir con size 0: %+v", first.Entries[1])
	}
	if first.Entries[0].Path != "test-data/seed.json" {
		t.Errorf("path = %q", first.Entries[0].Path)
	}
}

func TestListFixturesVacio(t *testing.T) {
	if got := ListFixtures(t.TempDir()); len(got) != 0 {
		t.Errorf("proyecto sin fixtures = %+v", got)
	}
	if got := ListFixtures(""); len(got) != 0 {
		t.Errorf("path vacío = %+v", got)
	}
	if len(FixtureDirNames()) == 0 {
		t.Error("debe exponer los directorios inspeccionados")
	}
}

func TestListFixturesTrunca(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "fixtures")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < fixtureEntryCap+5; i++ {
		name := filepath.Join(dataDir, fmt.Sprintf("f%03d.json", i))
		if err := os.WriteFile(name, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sets := ListFixtures(dir)
	if len(sets) != 1 {
		t.Fatalf("sets = %+v", sets)
	}
	if !sets[0].Truncated {
		t.Error("debe marcar Truncated al superar el tope")
	}
	if len(sets[0].Entries) != fixtureEntryCap {
		t.Errorf("entries = %d, want %d", len(sets[0].Entries), fixtureEntryCap)
	}
	if sets[0].Count != fixtureEntryCap+5 {
		t.Errorf("count = %d (el total no se trunca)", sets[0].Count)
	}
}
