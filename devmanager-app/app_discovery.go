package main

import (
	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/utils/detection"
)

// ---- Detección de config de proyecto (paridad detect_project_config) ----

// configuredProjectPaths returns a snapshot of the project paths currently
// registered in the manager so discovery can omit duplicates.
func (a *App) configuredProjectPaths() []string {
	projects := a.cfg.Projects()
	paths := make([]string, 0, len(projects))
	for _, project := range projects {
		paths = append(paths, project.Path)
	}
	return paths
}

// DetectProjectConfig expone la autodetección al frontend. existingPorts son
// los puertos ya configurados (evita colisiones en el diálogo de proyecto).
func (a *App) DetectProjectConfig(path string) detection.ProjectConfig {
	return detection.DetectProjectConfig(path, a.cfg.ConfiguredPorts())
}

// BrowseFolder abre un diálogo nativo de selección de carpeta (paridad
// QFileDialog.getExistingDirectory). Devuelve "" si el usuario cancela.
func (a *App) BrowseFolder() string {
	if a.ctx == nil {
		return ""
	}
	dir, err := wails.OpenDirectoryDialog(a.ctx, wails.OpenDialogOptions{
		Title: "Select Project Directory",
	})
	if err != nil {
		return ""
	}
	return dir
}

// BrowseWorkspaceFolder abre un diálogo nativo para seleccionar la carpeta
// contenedora cuyos proyectos directos se van a descubrir.
func (a *App) BrowseWorkspaceFolder() string {
	if a.ctx == nil {
		return ""
	}
	dir, err := wails.OpenDirectoryDialog(a.ctx, wails.OpenDialogOptions{
		Title: "Select Workspace Folder",
	})
	if err != nil {
		return ""
	}
	return dir
}

// DiscoverProjects finds unregistered projects directly inside root.
func (a *App) DiscoverProjects(root string) []detection.ProjectCandidate {
	return detection.DiscoverProjects(root, a.configuredProjectPaths())
}
