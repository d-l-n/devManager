package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/d-l-n/devmanager/internal/models"
)

func tempPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "projects.json")
}

func sample(name string) models.Project {
	p, _ := models.ParseProject([]byte(`{"name":"` + name + `","path":"C:/x` + name + `","server":{"port":4000}}`))
	return p
}

func sampleAt(name, path string, port int) models.Project {
	data := fmt.Sprintf(`{"name":"%s","path":"%s","server":{"port":%d}}`, name, path, port)
	p, _ := models.ParseProject([]byte(data))
	return p
}

func TestLoadMissingCreatesEmpty(t *testing.T) {
	path := tempPath(t)
	gotChanged := false
	m, err := NewManager(path, Options{OnProjectsChanged: func() { gotChanged = true }})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("esperaba 0 proyectos, got %d", m.Count())
	}
	if _, err := os.Stat(path); err != nil {
		t.Error("archivo vacío debe crearse")
	}
	if !gotChanged {
		t.Error("OnProjectsChanged debe dispararse en creación")
	}
}

func TestLoadValidFile(t *testing.T) {
	path := tempPath(t)
	os.WriteFile(path, []byte(`{"projects":[{"name":"A","path":"C:/a"},{"name":"B","path":"C:/b","pinned":true}]}`), 0o644)
	m, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if m.Count() != 2 {
		t.Fatalf("esperaba 2 proyectos, got %d", m.Count())
	}
	ps := m.Projects()
	if ps[1].Name != "B" || !ps[1].Pinned {
		t.Errorf("proyecto B mal parseado: %+v", ps[1])
	}
	if ps[0].Server.Port != 5173 {
		t.Errorf("defaults aplicados en carga: %+v", ps[0].Server)
	}
}

func TestLoadCorruptedBacksUpAndResets(t *testing.T) {
	path := tempPath(t)
	os.WriteFile(path, []byte(`{invalid json`), 0o644)
	var errMsg string
	m, err := NewManager(path, Options{OnError: func(msg string) { errMsg = msg }})
	if err != nil {
		t.Fatalf("load corrupto no debe fallar duro: %v", err)
	}
	if m.Count() != 0 {
		t.Errorf("corrupto debe resetear a 0, got %d", m.Count())
	}
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Error("debe existir backup .bak")
	}
	if !strings.Contains(errMsg, "Config file corrupted") {
		t.Errorf("mensaje de error inesperado: %q", errMsg)
	}
	data, _ := os.ReadFile(path)
	if len(data) == 0 {
		t.Error("config nueva debe haberse escrito")
	}
}

func TestCRUDPersists(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})

	m.AddProject(sample("A"))
	m.AddProject(sample("B"))
	if m.Count() != 2 {
		t.Fatalf("add fallo, count=%d", m.Count())
	}

	m.UpdateProject(0, sample("A2"))
	if m.Projects()[0].Name != "A2" {
		t.Error("update fallo")
	}

	m.TogglePin(1)
	if !m.Projects()[1].Pinned {
		t.Error("toggle_pin fallo")
	}

	// Persistencia: nueva instancia lee lo mismo
	m2, _ := NewManager(path, Options{})
	ps := m2.Projects()
	if ps[0].Name != "A2" || !ps[1].Pinned {
		t.Errorf("persistencia rota: %+v", ps)
	}

	m.RemoveProject(0)
	if m.Count() != 1 {
		t.Error("remove fallo")
	}
}

func TestIndexOutOfBoundsNoop(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.AddProject(sample("A"))

	before := m.Count()
	if err := m.RemoveProject(5); err == nil {
		t.Error("remove fuera de rango debe dar error")
	}
	if err := m.TogglePin(9); err == nil {
		t.Error("pin fuera de rango debe dar error")
	}
	if m.Count() != before {
		t.Error("operaciones fuera de rango no deben mutar")
	}
}

func TestNextAvailablePort(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.AddProject(sample("A"))                                            // puerto 4000 configurado
	m.probeFn = func(host string, port int) bool { return port <= 5174 } // 5173 y 5174 simulados ocupados

	got := m.NextAvailablePort(5173)
	if got != 5175 {
		t.Errorf("NextAvailablePort = %d, want 5175 (salta 5174 ocupado)", got)
	}
}

func TestConfiguredPortsIncludesDisabled(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	a := sample("A")
	b := sample("B")
	b.Server.Enabled = false
	b.Server.Port = 9999
	m.AddProject(a)
	m.AddProject(b)

	// Paridad Python get_configured_ports: incluye TODOS los port>0,
	// aunque el servidor esté disabled.
	got := m.ConfiguredPorts()
	if len(got) != 2 || got[0] != 4000 || got[1] != 9999 {
		t.Errorf("ConfiguredPorts = %v", got)
	}
}

func TestAddDuplicatePathRejected(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})

	a := sample("A")
	if err := m.AddProject(a); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Same path, different name → must be rejected
	b := sampleAt("B", "C:/xA", 4001)
	if err := m.AddProject(b); err == nil {
		t.Error("AddProject with duplicate path must fail")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists': %q", err.Error())
	}
	if m.Count() != 1 {
		t.Errorf("count should remain 1 after rejected add, got %d", m.Count())
	}
}

func TestUpdateDuplicatePathRejected(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})

	a := sample("A")
	a.Path = "C:/alpha"
	b := sample("B")
	b.Path = "C:/beta"
	m.AddProject(a)
	m.AddProject(b)

	// Try to change B's path to A's path → must be rejected
	b.Path = "C:/alpha"
	if err := m.UpdateProject(1, b); err == nil {
		t.Error("UpdateProject with duplicate path must fail")
	} else if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists': %q", err.Error())
	}
	// B should still have original path
	if m.Projects()[1].Path != "C:/beta" {
		t.Errorf("B's path should not have changed, got %q", m.Projects()[1].Path)
	}
}

func TestUpdateSamePathAllowed(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})

	a := sample("A")
	a.Path = "C:/alpha"
	m.AddProject(a)

	// Update A with same path but different name → allowed
	a.Name = "A2"
	if err := m.UpdateProject(0, a); err != nil {
		t.Errorf("UpdateProject with same path on same index should succeed: %v", err)
	}
	if m.Projects()[0].Name != "A2" {
		t.Error("name should be updated")
	}
}

func TestNormalizePathVariants(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})

	a := sampleAt("A", "C:/projects/myapp", 5173)
	if err := m.AddProject(a); err != nil {
		t.Fatalf("first add: %v", err)
	}

	// Same path with trailing slash → should still be duplicate
	b := sampleAt("B", "C:/projects/myapp/", 5174)
	if err := m.AddProject(b); err == nil {
		t.Error("trailing slash variant should be rejected as duplicate")
	}

	// Different case on Windows → should be duplicate (case-insensitive via normalizePath)
	c := sampleAt("C", "C:/Projects/MyApp", 5175)
	if err := m.AddProject(c); err == nil {
		t.Error("case-insensitive variant should be rejected as duplicate")
	}
}

func TestNormalizePathDifferentDirsAllowed(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})

	a := sample("A")
	a.Path = "C:/projects/app-a/frontend"
	if err := m.AddProject(a); err != nil {
		t.Fatalf("first add: %v", err)
	}

	b := sample("B")
	b.Path = "C:/projects/app-b/frontend"
	if err := m.AddProject(b); err != nil {
		t.Errorf("different directories with same name should be allowed: %v", err)
	}
	if m.Count() != 2 {
		t.Errorf("count should be 2, got %d", m.Count())
	}
}

func TestNormalizePathEmptySkipped(t *testing.T) {
	// Empty path should not trigger duplicate detection
	n1 := normalizePath("")
	n2 := normalizePath("")
	if n1 != "" || n2 != "" {
		t.Errorf("empty path normalization should yield empty, got %q, %q", n1, n2)
	}
	// findDuplicatePath with empty path should return -1
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	a := sampleAt("A", "C:/real", 5173)
	_ = m.AddProject(a)
	if idx := m.findDuplicatePath("", -1); idx != -1 {
		t.Errorf("empty path should not find duplicate, got index %d", idx)
	}
}

func TestSaveDetectedPortPersists(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.AddProject(sample("A"))

	if err := m.SaveDetectedPort(0, 4300); err != nil {
		t.Fatalf("SaveDetectedPort: %v", err)
	}
	got := m.Projects()[0].Server
	if got.Port != 4300 || got.URL != "http://localhost:4300" {
		t.Errorf("server = %+v, want port/url for 4300", got)
	}

	m2, _ := NewManager(path, Options{})
	got = m2.Projects()[0].Server
	if got.Port != 4300 || got.URL != "http://localhost:4300" {
		t.Errorf("persisted server = %+v, want port/url for 4300", got)
	}
	if err := m.SaveDetectedPort(4, 4300); err == nil {
		t.Error("out-of-range index must fail")
	}
	if err := m.SaveDetectedPort(0, 0); err == nil {
		t.Error("invalid port must fail")
	}
}
