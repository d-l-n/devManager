package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/d-l-n/devmanager/internal/devtools"
)

// errNoProject replica el mensaje out of range del resto de bindings.
func errNoProject(index int) error {
	return fmt.Errorf("project index %d out of range", index)
}

// ---- Dev tools bindings (Issue #68) ----
//
// File browser + editor + búsqueda operan sobre el proyecto activo con paths
// clampados al root (ver internal/devtools.clampRoot). Los snippets viven en
// %APPDATA%/devManager/snippets.json (globales, no por proyecto).

func (a *App) devSnippetStore() *devtools.SnippetStore {
	if a.devSnippets == nil {
		a.devSnippets = devtools.NewSnippetStore(filepath.Join(filepath.Dir(a.settingsPath), "snippets.json"))
		a.devSnippets.Load()
	}
	return a.devSnippets
}

// DevListDir lista una carpeta del proyecto activo.
func (a *App) DevListDir(index int, rel string) ([]devtools.FileEntry, error) {
	p := a.currentProject(index)
	if p.Path == "" {
		return nil, errNoProject(index)
	}
	return devtools.ListDir(p.Path, rel)
}

// DevReadFile lee un archivo del proyecto activo.
func (a *App) DevReadFile(index int, rel string) (string, error) {
	p := a.currentProject(index)
	if p.Path == "" {
		return "", errNoProject(index)
	}
	return devtools.ReadFile(p.Path, rel)
}

// DevWriteFile escribe un archivo del proyecto activo.
func (a *App) DevWriteFile(index int, rel, content string) []string {
	p := a.currentProject(index)
	if p.Path == "" {
		return []string{errNoProject(index).Error()}
	}
	if err := devtools.WriteFile(p.Path, rel, content); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DevSearchFiles busca por nombre/contenido en el proyecto activo.
func (a *App) DevSearchFiles(index int, query string) []devtools.SearchResult {
	p := a.currentProject(index)
	if p.Path == "" || strings.TrimSpace(query) == "" {
		return []devtools.SearchResult{}
	}
	return devtools.SearchFiles(p.Path, query)
}

// DevGetSnippets lista los snippets guardados.
func (a *App) DevGetSnippets() []devtools.Snippet {
	return a.devSnippetStore().Snippets
}

// DevSaveSnippet inserta/actualiza un snippet.
func (a *App) DevSaveSnippet(sn devtools.Snippet) []string {
	if errs := a.devSnippetStore().SaveSnippet(sn); len(errs) > 0 {
		return errs
	}
	return nil
}

// DevDeleteSnippet elimina un snippet por ID.
func (a *App) DevDeleteSnippet(id string) []string {
	if errs := a.devSnippetStore().DeleteSnippet(id); len(errs) > 0 {
		return errs
	}
	return nil
}
