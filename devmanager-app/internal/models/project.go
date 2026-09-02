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

type projectJSON struct {
	Name       *string               `json:"name"`
	Path       *string               `json:"path"`
	Server     *serverConfigJSON     `json:"server"`
	Playwright *playwrightConfigJSON `json:"playwright"`
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

// ParseProject replica Project.from_dict(): claves ausentes → defaults.
func ParseProject(data []byte) (Project, error) {
	var j projectJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Project{}, fmt.Errorf("invalid project json: %w", err)
	}
	p := Project{
		Server:     applyServer(j.Server),
		Playwright: applyPlaywright(j.Playwright),
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
