// Package config porta app/config/manager.py (callbacks en vez de signals Qt).
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/d-l-n/devmanager/internal/env"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/utils/ports"
)

// normalizePath normaliza una ruta para comparación: Abs + Clean + lower + forward slash.
func normalizePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return strings.ToLower(filepath.ToSlash(filepath.Clean(p)))
	}
	return strings.ToLower(filepath.ToSlash(filepath.Clean(abs)))
}

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

// findDuplicatePath returns the index of an existing project with the same
// normalized path, or -1 if none. excludeIndex is skipped (for UpdateProject).
func (m *Manager) findDuplicatePath(path string, excludeIndex int) int {
	norm := normalizePath(path)
	if norm == "" {
		return -1
	}
	for i, p := range m.projects {
		if i == excludeIndex {
			continue
		}
		if normalizePath(p.Path) == norm {
			return i
		}
	}
	return -1
}

func (m *Manager) AddProject(p models.Project) error {
	if idx := m.findDuplicatePath(p.Path, -1); idx >= 0 {
		return fmt.Errorf("a project pointing to this directory already exists (%s)", m.projects[idx].Name)
	}
	if errs := p.Validate(); len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	p.EnsureEnvsFromTopLevel()
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
	if idx := m.findDuplicatePath(p.Path, index); idx >= 0 {
		return fmt.Errorf("a project pointing to this directory already exists (%s)", m.projects[idx].Name)
	}
	if errs := p.Validate(); len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	p.EnsureEnvsFromTopLevel()
	m.projects[index] = p
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

// SetActiveEnv cambia el entorno activo: valida que exista, copia
// env→top-level, guarda y emite changed.
// Fase 3: si el puerto target está ocupado solo advierte (emitError → toast
// warning) sin error duro: el switch igual ocurre.
func (m *Manager) SetActiveEnv(index int, env string) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	p := &m.projects[index]
	if !models.IsValidEnvName(env) {
		return fmt.Errorf("invalid environment name %q", env)
	}
	e, ok := p.Envs[env]
	if !ok {
		return fmt.Errorf("unknown environment %q", env)
	}
	p.ActiveEnv = env
	p.Server = e.Server
	p.Playwright = e.Playwright
	p.User = e.User
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	if e.Server.Port > 0 && m.probeFn("127.0.0.1", e.Server.Port) {
		m.emitError(fmt.Sprintf(
			"Environment %q uses port %d which is already in use", env, e.Server.Port))
	}
	return nil
}

// GetEnvVars devuelve una copia de las vars del entorno.
func (m *Manager) GetEnvVars(index int, envName string) (map[string]string, error) {
	if index < 0 || index >= len(m.projects) {
		return nil, fmt.Errorf("index %d out of range", index)
	}
	e, ok := m.projects[index].Envs[envName]
	if !ok {
		return nil, fmt.Errorf("unknown environment %q", envName)
	}
	out := make(map[string]string, len(e.Vars))
	for k, v := range e.Vars {
		out[k] = v
	}
	return out, nil
}

// SetEnvVars reemplaza las vars del entorno (keys validadas con
// env.ValidateKey), guarda y emite changed.
func (m *Manager) SetEnvVars(index int, envName string, vars map[string]string) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	p := &m.projects[index]
	e, ok := p.Envs[envName]
	if !ok {
		return fmt.Errorf("unknown environment %q", envName)
	}
	if vars == nil {
		vars = map[string]string{}
	}
	for k := range vars {
		if err := env.ValidateKey(k); err != nil {
			return err
		}
	}
	cp := make(map[string]string, len(vars))
	for k, v := range vars {
		cp[k] = v
	}
	// Fase 3: si llega la máscara en una key secreta, el frontend mostraba
	// "***" sin conocer el real: preserva el valor guardado en vez de
	// pisarlo con la máscara.
	for k, v := range cp {
		if v == models.MaskedValue && models.IsSecret(e.Secrets, k) {
			if old, ok := e.Vars[k]; ok {
				cp[k] = old
			}
		}
	}
	e.Vars = cp
	p.Envs[envName] = e
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

// GetSecrets devuelve copia de la lista de secretos del entorno.
func (m *Manager) GetSecrets(index int, envName string) ([]string, error) {
	if index < 0 || index >= len(m.projects) {
		return nil, fmt.Errorf("index %d out of range", index)
	}
	e, ok := m.projects[index].Envs[envName]
	if !ok {
		return nil, fmt.Errorf("unknown environment %q", envName)
	}
	return append([]string{}, e.Secrets...), nil
}

// SetSecretKey marca una key como secreta (keys validadas), guarda y emite.
func (m *Manager) SetSecretKey(index int, envName, key string) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	if err := env.ValidateKey(key); err != nil {
		return err
	}
	p := &m.projects[index]
	e, ok := p.Envs[envName]
	if !ok {
		return fmt.Errorf("unknown environment %q", envName)
	}
	if !models.IsSecret(e.Secrets, key) {
		e.Secrets = append(e.Secrets, key)
		p.Envs[envName] = e
	}
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

// UnsetSecretKey desmarca una key como secreta, guarda y emite.
func (m *Manager) UnsetSecretKey(index int, envName, key string) error {
	if index < 0 || index >= len(m.projects) {
		return fmt.Errorf("index %d out of range", index)
	}
	p := &m.projects[index]
	e, ok := p.Envs[envName]
	if !ok {
		return fmt.Errorf("unknown environment %q", envName)
	}
	kept := make([]string, 0, len(e.Secrets))
	for _, s := range e.Secrets {
		if s != key {
			kept = append(kept, s)
		}
	}
	e.Secrets = kept
	p.Envs[envName] = e
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
	// Fase 1 #67: el puerto detectado también vive en envs[active].server.
	if p.Envs != nil {
		if e, ok := p.Envs[p.ActiveEnv]; ok {
			e.Server.Port = port
			e.Server.URL = p.Server.URL
			p.Envs[p.ActiveEnv] = e
		}
	}
	if err := m.Save(); err != nil {
		return err
	}
	m.emitChanged()
	return nil
}

// envPorts devuelve los puertos de todos los envs en orden estable.
func envPorts(p models.Project) []int {
	names := make([]string, 0, len(p.Envs))
	for n := range p.Envs {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []int
	for _, n := range names {
		if port := p.Envs[n].Server.Port; port > 0 {
			out = append(out, port)
		}
	}
	return out
}

// ConfiguredPorts replica get_configured_ports: todos los port>0.
// Fase 3: incluye puertos de todos los envs (cross-env), sin duplicados.
func (m *Manager) ConfiguredPorts() []int {
	var out []int
	seen := map[int]bool{}
	add := func(port int) {
		if port > 0 && !seen[port] {
			seen[port] = true
			out = append(out, port)
		}
	}
	for _, p := range m.projects {
		add(p.Server.Port)
		for _, port := range envPorts(p) {
			add(port)
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
// Fase 3: evita colisión cross-env (envs no-activos de otros proyectos
// también reservan) y mantiene el espejo top-level↔active env al reasignar.
func (m *Manager) AutoAssignUniquePorts(startPort int) int {
	used := map[int]bool{}
	changed := 0
	current := startPort

	for i := range m.projects {
		p := &m.projects[i]
		if !p.Server.Enabled {
			continue
		}
		if used[p.Server.Port] || p.Server.Port <= 0 || otherEnvUsesPort(m.projects, i, p.Server.Port) {
			for used[current] || m.probeFn("127.0.0.1", current) || otherEnvUsesPort(m.projects, i, current) {
				current++
			}
			p.Server.Port = current
			p.Server.URL = fmt.Sprintf("http://localhost:%d", current)
			if p.Envs != nil {
				if e, ok := p.Envs[p.ActiveEnv]; ok {
					e.Server.Port = current
					e.Server.URL = p.Server.URL
					p.Envs[p.ActiveEnv] = e
				}
			}
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

// otherEnvUsesPort reporta si otro proyecto reserva el puerto en alguno de
// sus envs no-activos. El propio proyecto se excluye: compartir puerto entre
// sus envs es válido (solo corre el activo).
func otherEnvUsesPort(projects []models.Project, index, port int) bool {
	if port <= 0 {
		return false
	}
	for j := range projects {
		if j == index {
			continue
		}
		for n, e := range projects[j].Envs {
			if n != projects[j].ActiveEnv && e.Server.Port == port {
				return true
			}
		}
	}
	return false
}
