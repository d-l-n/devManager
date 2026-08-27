package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx      context.Context
	cancel   context.CancelFunc
	projects []Project
	mu       sync.RWMutex
}

type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Path   string `json:"path"`
	Active bool   `json:"active"`
}

func NewApp() *App {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &App{
		ctx:      ctx,
		cancel:   cancel,
		projects: []Project{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	
	// Load projects
	if err := a.loadProjects(); err != nil {
		fmt.Printf("Error loading projects: %v\n", err)
	}
	
	// Start background monitoring
	go a.monitorProjects()
}

func (a *App) domReady(ctx context.Context) {
	// This function is called when the DOM is ready
}

func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	return false
}

func (a *App) shutdown(ctx context.Context) {
	a.cancel()
}

func (a *App) loadProjects() error {
	// For now, just add a sample project
	project := Project{
		ID:     uuid.New().String(),
		Name:   "Sample Project",
		Path:   "..",
		Active: true,
	}
	a.projects = append(a.projects, project)
	return nil
}

func (a *App) monitorProjects() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.checkProjectStatus()
		}
	}
}

func (a *App) checkProjectStatus() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.projects {
		// For now, just mark all as active
		a.projects[i].Active = true
	}
}

// Tray methods
func (a *App) onTrayReady() {
	fmt.Println("Tray ready")
}

// Getters for frontend
func (a *App) GetProjects() []Project {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.projects
}

func (a *App) AddProject(name, path string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	project := Project{
		ID:     uuid.New().String(),
		Name:   name,
		Path:   path,
		Active: false,
	}
	
	a.projects = append(a.projects, project)
	return nil
}

func (a *App) RemoveProject(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	for i, project := range a.projects {
		if project.ID == id {
			a.projects = append(a.projects[:i], a.projects[i+1:]...)
			break
		}
	}
	
	return nil
}

func (a *App) ToggleProject(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	for i := range a.projects {
		if a.projects[i].ID == id {
			a.projects[i].Active = !a.projects[i].Active
			break
		}
	}
	
	return nil
}

// BrowseForFolder opens a directory selection dialog
func (a *App) BrowseForFolder() (string, error) {
	selection, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Project Folder",
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to open directory dialog: %v", err)
	}
	
	if selection == "" {
		return "", fmt.Errorf("no folder selected")
	}
	
	// Convert to absolute path and clean it
	absPath, err := filepath.Abs(selection)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %v", err)
	}
	
	return absPath, nil
}

// ImportProjects scans a folder for project directories and imports them
func (a *App) ImportProjects(folderPath string) ([]Project, error) {
	var importedProjects []Project
	
	// For now, just import the selected folder as a project
	// In a real implementation, you might scan for git repos, package.json, etc.
	
	// Check if the folder already exists
	a.mu.RLock()
	exists := false
	for _, p := range a.projects {
		if p.Path == folderPath {
			exists = true
			break
		}
	}
	a.mu.RUnlock()
	
	if !exists {
		// Extract folder name as project name
		folderName := filepath.Base(folderPath)
		
		project := Project{
			ID:     uuid.New().String(),
			Name:   folderName,
			Path:   folderPath,
			Active: false,
		}
		
		a.mu.Lock()
		a.projects = append(a.projects, project)
		a.mu.Unlock()
		
		importedProjects = append(importedProjects, project)
	}
	
	return importedProjects, nil
}