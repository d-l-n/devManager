package config

import (
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
	p, _ := models.ParseProject([]byte(`{"name":"` + name + `","path":"C:/x","server":{"port":4000}}`))
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
		t.Error("archivo vac├¡o debe crearse")
	}
	if !gotChanged {
		t.Error("OnProjectsChanged debe dispararse en creaci├│n")
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
	// aunque el servidor est├® disabled.
	got := m.ConfiguredPorts()
	if len(got) != 2 || got[0] != 4000 || got[1] != 9999 {
		t.Errorf("ConfiguredPorts = %v", got)
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
