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

func TestConfiguredPortsCrossEnv(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.AddProject(sample("A")) // top 4000 + dev 4000
	// Segundo proyecto con staging divergente.
	b := sampleAt("B", "C:/xb", 5000)
	if err := m.AddProject(b); err != nil {
		t.Fatalf("add B: %v", err)
	}
	m.projects[1].Envs["staging"] = models.EnvConfig{
		Server: models.ServerConfig{Enabled: true, Command: "npm run dev", Port: 5001, URL: "http://localhost:5001"},
		Vars:   map[string]string{}, EnvFile: ".env.staging", Secrets: []string{},
	}
	got := m.ConfiguredPorts()
	for _, want := range []int{4000, 5000, 5001} {
		found := false
		for _, p := range got {
			if p == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("ConfiguredPorts = %v, falta %d", got, want)
		}
	}
	if len(got) != 3 {
		t.Errorf("ConfiguredPorts sin duplicados = %v", got)
	}
}

func TestAutoAssignAvoidsCrossEnv(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.probeFn = func(host string, port int) bool { return false }
	m.AddProject(sampleAt("A", "C:/xa", 5173))
	if err := m.AddProject(sampleAt("B", "C:/xb", 5174)); err != nil {
		t.Fatalf("add B: %v", err)
	}
	// B.staging reserva 5175 (no-activo): A no debe moverse pero un nuevo
	// proyecto colisionando debe saltarlo.
	m.projects[1].Envs["staging"] = models.EnvConfig{
		Server: models.ServerConfig{Enabled: true, Command: "npm run dev", Port: 5175, URL: "http://localhost:5175"},
		Vars:   map[string]string{}, EnvFile: ".env.staging", Secrets: []string{},
	}
	if err := m.AddProject(sampleAt("C", "C:/xc", 5175)); err != nil {
		t.Fatalf("add C: %v", err)
	}
	n := m.AutoAssignUniquePorts(5173)
	if n != 1 {
		t.Fatalf("esperaba 1 reasignado, got %d", n)
	}
	if got := m.Projects()[2].Server.Port; got != 5176 {
		t.Errorf("C reasignado a %d, want 5176 (salta staging de B)", got)
	}
	if got := m.Projects()[0].Server.Port; got != 5173 {
		t.Errorf("A debe conservar 5173, got %d", got)
	}
}

func TestSetActiveEnvOccupiedWarnsWithoutError(t *testing.T) {
	path := tempPath(t)
	var warnings []string
	m, _ := NewManager(path, Options{OnError: func(msg string) { warnings = append(warnings, msg) }})
	m.AddProject(sampleAt("A", "C:/xa", 5173))
	m.projects[0].Envs["staging"] = models.EnvConfig{
		Server: models.ServerConfig{Enabled: true, Command: "npm run dev", Port: 9999, URL: "http://localhost:9999"},
		Vars:   map[string]string{}, EnvFile: ".env.staging", Secrets: []string{},
	}
	m.probeFn = func(host string, port int) bool { return port == 9999 }
	if err := m.SetActiveEnv(0, "staging"); err != nil {
		t.Fatalf("puerto ocupado no debe ser error duro: %v", err)
	}
	if m.Projects()[0].ActiveEnv != "staging" {
		t.Error("el switch igual debe ocurrir")
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "9999") {
		t.Errorf("esperaba 1 warning con el puerto, got %v", warnings)
	}
	// Puerto libre → sin warning.
	m.probeFn = func(host string, port int) bool { return false }
	warnings = nil
	if err := m.SetActiveEnv(0, "dev"); err != nil {
		t.Fatalf("switch: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("sin warnings esperados, got %v", warnings)
	}
}

func TestSetGetEnvVars(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.AddProject(sample("A"))

	if err := m.SetEnvVars(0, "dev", map[string]string{"FOO": "bar"}); err != nil {
		t.Fatalf("SetEnvVars: %v", err)
	}
	got, err := m.GetEnvVars(0, "dev")
	if err != nil {
		t.Fatalf("GetEnvVars: %v", err)
	}
	if got["FOO"] != "bar" {
		t.Errorf("vars = %v", got)
	}
	// Copia: mutar el resultado no afecta al manager.
	got["FOO"] = "mutated"
	got2, _ := m.GetEnvVars(0, "dev")
	if got2["FOO"] != "bar" {
		t.Error("GetEnvVars debe devolver copia")
	}
	// Persistencia.
	m2, _ := NewManager(path, Options{})
	got3, err := m2.GetEnvVars(0, "dev")
	if err != nil || got3["FOO"] != "bar" {
		t.Errorf("persisted vars = %v, err = %v", got3, err)
	}
	// Keys inválidas / env desconocido / índice malo.
	if err := m.SetEnvVars(0, "dev", map[string]string{"bad-key": "x"}); err == nil {
		t.Error("key inválida debe fallar")
	}
	if err := m.SetEnvVars(0, "ghost", map[string]string{"A": "b"}); err == nil {
		t.Error("env desconocido debe fallar")
	}
	if err := m.SetEnvVars(9, "dev", map[string]string{"A": "b"}); err == nil {
		t.Error("índice fuera de rango debe fallar")
	}
	if _, err := m.GetEnvVars(0, "ghost"); err == nil {
		t.Error("GetEnvVars env desconocido debe fallar")
	}
}

func TestSetEnvVarsPreservesMaskedSecrets(t *testing.T) {
	path := tempPath(t)
	m, _ := NewManager(path, Options{})
	m.AddProject(sample("A"))
	if err := m.SetEnvVars(0, "dev", map[string]string{"TOKEN": "real123"}); err != nil {
		t.Fatalf("set inicial: %v", err)
	}
	e := m.projects[0].Envs["dev"]
	e.Secrets = []string{"TOKEN"}
	m.projects[0].Envs["dev"] = e
	// Llega la máscara (frontend no conoce el real) → preserva.
	if err := m.SetEnvVars(0, "dev", map[string]string{"TOKEN": models.MaskedValue, "PLAIN": "x"}); err != nil {
		t.Fatalf("set enmascarado: %v", err)
	}
	got, _ := m.GetEnvVars(0, "dev")
	if got["TOKEN"] != "real123" {
		t.Errorf("TOKEN real pisado por máscara: %q", got["TOKEN"])
	}
	if got["PLAIN"] != "x" {
		t.Errorf("PLAIN = %q", got["PLAIN"])
	}
}
