package main

import (
	"fmt"
	"strings"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/google/uuid"

	"github.com/d-l-n/devmanager/internal/models"
)

// ---- Backlog bindings (feature) ----
// Los items viven en models.Project.Backlog y se persisten vía config.Manager.

// emitBacklogChanged notifica al frontend para refrescar el panel del proyecto.
func (a *App) emitBacklogChanged(index int) {
	wails.EventsEmit(a.ctx, "backlog:changed", map[string]int{"projectIndex": index})
}

func (a *App) GetBacklog(index int) []models.BacklogItem {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return nil
	}
	return projects[index].Backlog
}

func (a *App) AddBacklogItem(index int, title, description, status, priority string) error {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return fmt.Errorf("project index %d out of range", index)
	}
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("backlog item title cannot be empty")
	}
	p := projects[index]
	now := time.Now().Format(time.RFC3339Nano)
	item := models.BacklogItem{
		ID:          uuid.NewString(),
		Title:       strings.TrimSpace(title),
		Description: strings.TrimSpace(description),
		Status:      status,
		Priority:    priority,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	p.Backlog = append(p.Backlog, item)
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return err
	}
	a.emitBacklogChanged(index)
	return nil
}

func (a *App) UpdateBacklogItem(index int, itemID, title, description, status, priority string) error {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return fmt.Errorf("project index %d out of range", index)
	}
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("backlog item title cannot be empty")
	}
	p := projects[index]
	for i := range p.Backlog {
		if p.Backlog[i].ID == itemID {
			p.Backlog[i].Title = strings.TrimSpace(title)
			p.Backlog[i].Description = strings.TrimSpace(description)
			p.Backlog[i].Status = status
			p.Backlog[i].Priority = priority
			p.Backlog[i].UpdatedAt = time.Now().Format(time.RFC3339Nano)
			if err := a.cfg.UpdateProject(index, p); err != nil {
				return err
			}
			a.emitBacklogChanged(index)
			return nil
		}
	}
	return fmt.Errorf("backlog item %s not found", itemID)
}

func (a *App) DeleteBacklogItem(index int, itemID string) error {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return fmt.Errorf("project index %d out of range", index)
	}
	p := projects[index]
	for i := range p.Backlog {
		if p.Backlog[i].ID == itemID {
			p.Backlog = append(p.Backlog[:i], p.Backlog[i+1:]...)
			if err := a.cfg.UpdateProject(index, p); err != nil {
				return err
			}
			a.emitBacklogChanged(index)
			return nil
		}
	}
	return fmt.Errorf("backlog item %s not found", itemID)
}

func (a *App) MoveBacklogItem(index int, itemID string, newIndex int) error {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return fmt.Errorf("project index %d out of range", index)
	}
	p := projects[index]
	if newIndex < 0 || newIndex >= len(p.Backlog) {
		return fmt.Errorf("backlog index %d out of range", newIndex)
	}
	cur := -1
	for i := range p.Backlog {
		if p.Backlog[i].ID == itemID {
			cur = i
			break
		}
	}
	if cur < 0 {
		return fmt.Errorf("backlog item %s not found", itemID)
	}
	if cur == newIndex {
		return nil
	}
	item := p.Backlog[cur]
	p.Backlog = append(p.Backlog[:cur], p.Backlog[cur+1:]...)
	p.Backlog = append(p.Backlog[:newIndex], append([]models.BacklogItem{item}, p.Backlog[newIndex:]...)...)
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return err
	}
	a.emitBacklogChanged(index)
	return nil
}
