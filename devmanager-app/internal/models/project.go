// Package models porta app/models/project.py.
package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ServerState replica ServerState de project.py como strings serializables
// al frontend vía EventsEmit.
type ServerState string

const (
	StateStopped  ServerState = "stopped"
	StateStarting ServerState = "starting"
	StateRunning  ServerState = "running"
	StateStopping ServerState = "stopping"
	StateError    ServerState = "error"
)

type ServerConfig struct {
	Enabled        bool   `json:"enabled"`
	Command        string `json:"command"`
	Port           int    `json:"port"`
	URL            string `json:"url"`
	StartupTimeout int    `json:"startup_timeout"`
}

type PlaywrightConfig struct {
	Enabled       bool   `json:"enabled"`
	Command       string `json:"command"`
	UICommand     string `json:"ui_command"`
	DebugCommand  string `json:"debug_command"`
	ReportCommand string `json:"report_command"`
}

// UserConfig define el comando de creación de usuarios del proyecto gestionado.
// Se ejecuta con los datos del usuario en env vars DM_USER_* (sin interpolación
// de shell); el propio proyecto decide su stack (Firebase, REST, seed DB, ...).
type UserConfig struct {
	Enabled bool   `json:"enabled"`
	Command string `json:"command"`
}

// KnownTabs son los ids de tabs del detail view (paridad con index.html).
// Logs no se puede ocultar: es el fallback del tab activo.
var KnownTabs = []string{"logs", "scripts", "git", "deps", "playwright", "evidence", "obscura", "backlog"}

// TabsConfig personaliza por proyecto el detail view: tabs ocultos y orden
// de aparición. Vacío = comportamiento default (todos visibles, orden del DOM).
type TabsConfig struct {
	Hidden []string `json:"hidden"`
	Order  []string `json:"order"`
}

type BacklogItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`   // todo, in-progress, done
	Priority    string `json:"priority"` // low, medium, high
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type Project struct {
	Name       string           `json:"name"`
	Path       string           `json:"path"`
	Server     ServerConfig     `json:"server"`
	Playwright PlaywrightConfig `json:"playwright"`
	User       UserConfig       `json:"user"`
	Tabs       TabsConfig       `json:"tabs"`
	Pinned     bool             `json:"pinned"`
	Backlog    []BacklogItem    `json:"backlog"`
}

// Validate replica Project.validate().
func (p Project) Validate() []string {
	var errs []string
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, "Project name cannot be empty")
	}
	if strings.TrimSpace(p.Path) == "" {
		errs = append(errs, "Project path cannot be empty")
	}
	errs = append(errs, p.validateTabs()...)
	return errs
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// validateTabs valida ids conocidos, sin duplicados y logs no ocultable.
func (p Project) validateTabs() []string {
	var errs []string
	seen := map[string]bool{}
	for _, id := range p.Tabs.Hidden {
		if id == "logs" {
			errs = append(errs, `tab "logs" cannot be hidden`)
		}
		if !containsString(KnownTabs, id) {
			errs = append(errs, fmt.Sprintf(`unknown tab %q in hidden`, id))
		}
		if seen[id] {
			errs = append(errs, fmt.Sprintf(`duplicate tab %q in hidden`, id))
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, id := range p.Tabs.Order {
		if !containsString(KnownTabs, id) {
			errs = append(errs, fmt.Sprintf(`unknown tab %q in order`, id))
		}
		if seen[id] {
			errs = append(errs, fmt.Sprintf(`duplicate tab %q in order`, id))
		}
		seen[id] = true
	}
	return errs
}

// Tipos sombra con punteros: distinguen clave ausente de valor cero,
// replicando dict.get(key, default) de Python.
type serverConfigJSON struct {
	Enabled        *bool   `json:"enabled"`
	Command        *string `json:"command"`
	Port           *int    `json:"port"`
	URL            *string `json:"url"`
	StartupTimeout *int    `json:"startup_timeout"`
}

type playwrightConfigJSON struct {
	Enabled       *bool   `json:"enabled"`
	Command       *string `json:"command"`
	UICommand     *string `json:"ui_command"`
	DebugCommand  *string `json:"debug_command"`
	ReportCommand *string `json:"report_command"`
}

type backlogItemJSON struct {
	ID          *string `json:"id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	CreatedAt   *string `json:"created_at"`
	UpdatedAt   *string `json:"updated_at"`
}

type userConfigJSON struct {
	Enabled *bool   `json:"enabled"`
	Command *string `json:"command"`
}

type tabsConfigJSON struct {
	Hidden *[]string `json:"hidden"`
	Order  *[]string `json:"order"`
}

type projectJSON struct {
	Name       *string               `json:"name"`
	Path       *string               `json:"path"`
	Server     *serverConfigJSON     `json:"server"`
	Playwright *playwrightConfigJSON `json:"playwright"`
	User       *userConfigJSON       `json:"user"`
	Tabs       *tabsConfigJSON       `json:"tabs"`
	Pinned     *bool                 `json:"pinned"`
	Backlog    *[]backlogItemJSON    `json:"backlog"`
}

func applyServer(j *serverConfigJSON) ServerConfig {
	c := ServerConfig{
		Enabled:        true,
		Command:        "npm run dev",
		Port:           5173,
		URL:            "http://localhost:5173",
		StartupTimeout: 15000,
	}
	if j == nil {
		return c
	}
	if j.Enabled != nil {
		c.Enabled = *j.Enabled
	}
	if j.Command != nil {
		c.Command = *j.Command
	}
	if j.Port != nil {
		c.Port = *j.Port
	}
	if j.URL != nil {
		c.URL = *j.URL
	}
	if j.StartupTimeout != nil {
		c.StartupTimeout = *j.StartupTimeout
	}
	return c
}

func applyPlaywright(j *playwrightConfigJSON) PlaywrightConfig {
	c := PlaywrightConfig{
		Enabled:       true,
		Command:       "npx playwright test",
		UICommand:     "npx playwright test --ui",
		DebugCommand:  "npx playwright test --debug",
		ReportCommand: "npx playwright show-report",
	}
	if j == nil {
		return c
	}
	if j.Enabled != nil {
		c.Enabled = *j.Enabled
	}
	if j.Command != nil {
		c.Command = *j.Command
	}
	if j.UICommand != nil {
		c.UICommand = *j.UICommand
	}
	if j.DebugCommand != nil {
		c.DebugCommand = *j.DebugCommand
	}
	if j.ReportCommand != nil {
		c.ReportCommand = *j.ReportCommand
	}
	return c
}

func applyUser(j *userConfigJSON) UserConfig {
	c := UserConfig{
		Enabled: true,
		Command: "",
	}
	if j == nil {
		return c
	}
	if j.Enabled != nil {
		c.Enabled = *j.Enabled
	}
	if j.Command != nil {
		c.Command = *j.Command
	}
	return c
}

func applyBacklogItem(j backlogItemJSON) BacklogItem {
	now := time.Now().Format(time.RFC3339Nano)
	c := BacklogItem{
		ID:          "",
		Title:       "",
		Description: "",
		Status:      "todo",
		Priority:    "medium",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if j.ID != nil {
		c.ID = *j.ID
	}
	if j.Title != nil {
		c.Title = *j.Title
	}
	if j.Description != nil {
		c.Description = *j.Description
	}
	if j.Status != nil {
		c.Status = *j.Status
	}
	if j.Priority != nil {
		c.Priority = *j.Priority
	}
	if j.CreatedAt != nil {
		c.CreatedAt = *j.CreatedAt
	}
	if j.UpdatedAt != nil {
		c.UpdatedAt = *j.UpdatedAt
	}
	return c
}

func applyTabsConfig(j *tabsConfigJSON) TabsConfig {
	if j == nil {
		return TabsConfig{}
	}
	t := TabsConfig{}
	if j.Hidden != nil {
		t.Hidden = append([]string{}, *j.Hidden...)
	}
	if j.Order != nil {
		t.Order = append([]string{}, *j.Order...)
	}
	return t
}

// ParseProject replica Project.from_dict(): claves ausentes → defaults.
func ParseProject(data []byte) (Project, error) {
	var j projectJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Project{}, fmt.Errorf("invalid project json: %w", err)
	}
	p := Project{
		Server:     applyServer(j.Server),
		Playwright: applyPlaywright(j.Playwright),
		User:       applyUser(j.User),
		Tabs:       applyTabsConfig(j.Tabs),
		Backlog:    []BacklogItem{},
	}
	if j.Name != nil {
		p.Name = *j.Name
	}
	if j.Path != nil {
		p.Path = *j.Path
	}
	if j.Pinned != nil {
		p.Pinned = *j.Pinned
	}
	if j.Backlog != nil {
		p.Backlog = make([]BacklogItem, len(*j.Backlog))
		for i, item := range *j.Backlog {
			p.Backlog[i] = applyBacklogItem(item)
		}
	}
	return p, nil
}
