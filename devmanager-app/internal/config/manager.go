// Package config porta app/config/manager.py (callbacks en vez de signals Qt).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/utils/ports"
)

type Options struct {
	OnProjectsChanged func()
	OnError           func(msg string)
}

type fileFormat struct {
	Projects []json.RawMessage `json:"projects"`
}

type Manager struct {
	path     string
	opts     Options
	projects []models.Project

	// Inyectable para tests de puertos.
	probeFn func(host string, port int) bool
}

func NewManager(path string, opts Options) (*Manager, error) {
	m := &Manager{path: path, opts: opts, probeFn: ports.IsPortOpen}
	m.Load()
	return m, nil
}

func (m *Manager) emitChanged() {
	if m.opts.OnProjectsChanged != nil {
		m.opts.OnProjectsChanged()
	}
}

func (m *Manager) emitError(msg string) {
	if m.opts.OnError != nil {
		m.opts.OnError(msg)
	}
}

// Load replica ConfigManager.load().
func (m *Manager) Load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			m.projects = nil
			_ = m.Save()
			m.emitChanged()
			return
		}
		m.emitError(fmt.Sprintf("Failed to load config: %v", err))
		return
	}

	var ff fileFormat
	if err := json.Unmarshal(data, &ff); err != nil {
		backupPath := m.path + ".bak"
		_ = os.WriteFile(backupPath, data, 0o644)
		m.projects = nil
		_ = m.Save()
		m.emitError(fmt.Sprintf(
			"Config file corrupted. Backed up to %s and created a new empty config.", backupPath))
		m.emitChanged()
		return
	}

	m.projects = make([]models.Project, 0, len(ff.Projects))
	for _, raw := range ff.Projects {
		p, perr := models.ParseProject(raw)
		if perr != nil {
			m.emitError(fmt.Sprintf("Failed to load config: %v", perr))
			continue
		}
		m.projects = append(m.projects, p)
	}
	m.emitChanged()
}

// Save replica save(): MarshalIndent("", "  ") paridad json.dump(indent=2).
func (m *Manager) Save() error {
	ff := fileFormat{Projects: make([]json.RawMessage, 0, len(m.projects))}
	for _, p := range m.projects {
		raw, err := json.Marshal(p)
		if err != nil {
			m.emitError(fmt.Sprintf("Failed to save config: %v", err))
			return err
		}
		ff.Projects = append(ff.Projects, raw)
	}
	out, err := json.MarshalIndent(ff, "", "  ")
	if err != nil {
		m.emitError(fmt.Sprintf("Failed to save config: %v", err))
		return err
	}
	if dir := filepath.Dir(m.path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	if err := os.WriteFile(m.path, out, 0o644); err != nil {
		m.emitError(fmt.Sprintf("Failed to save config: %v", err))
		return err
	}
	return nil
}

func (m *Manager) Projects() []models.Project {
	out := make([]models.Project, len(m.projects))
	copy(out, m.projects)
	return out
}

func (m *Manager) Count() int { return len(m.projects) }

func (m *Manager) AddProject(p models.Project) error {
	m.projects = append(m.projects, p)
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

func (m *Manager) UpdateProject(index int, p models.Project) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	m.projects[index] = p
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

func (m *Manager) RemoveProject(index int) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	m.projects = append(m.projects[:index], m.projects[index+1:]...)
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

func (m *Manager) TogglePin(index int) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	m.projects[index].Pinned = !m.projects[index].Pinned
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

// SaveDetectedPort persiste el puerto que el servidor informó en runtime y
// reconstruye su URL localhost. Mantiene la configuración coherente para el
// siguiente inicio sin aceptar una URL arbitraria desde el frontend.
func (m *Manager) SaveDetectedPort(index, port int) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port %d", port)
	}
	p := &m.projects[index]
	p.Server.Port = port
	p.Server.URL = fmt.Sprintf("http://localhost:%d", port)
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

// ConfiguredPorts replica get_configured_ports: todos los port>0.
func (m *Manager) ConfiguredPorts() []int {
	var out []int
	for _, p := range m.projects {
		if p.Server.Port > 0 {
			out = append(out, p.Server.Port)
		}
	}
	return out
}

// NextAvailablePort replica get_next_available_port.
func (m *Manager) NextAvailablePort(basePort int) int {
	configured := map[int]bool{}
	for _, p := range m.ConfiguredPorts() {
		configured[p] = true
	}
	port := basePort
	if port < 1024 {
		port = 1024
	}
	for ; port <= 65535; port++ {
		if !configured[port] && !m.probeFn("127.0.0.1", port) {
			return port
		}
	}
	return basePort
}

// AutoAssignUniquePorts replica auto_assign_unique_ports.
// Devuelve cantidad de proyectos reasignados.
func (m *Manager) AutoAssignUniquePorts(startPort int) int {
	used := map[int]bool{}
	changed := 0
	current := startPort

	for i := range m.projects {
		p := &m.projects[i]
		if !p.Server.Enabled {
			continue
		}
		if used[p.Server.Port] || p.Server.Port <= 0 {
			for used[current] || m.probeFn("127.0.0.1", current) {
				current++
			}
			p.Server.Port = current
			p.Server.URL = fmt.Sprintf("http://localhost:%d", current)
			used[current] = true
			current++
			changed++
		} else {
			used[p.Server.Port] = true
		}
	}

	if changed > 0 {
		_ = m.Save()
		m.emitChanged()
	}
	return changed
}
