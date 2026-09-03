package main

import (
	"bytes"
	"encoding/base64"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/process"
	"github.com/d-l-n/devmanager/internal/utils/evidence"
)

// ---- Evidence bindings ----

const maxThumbnailBytes = 2 * 1024 * 1024

func (a *App) GetEvidence(index int) []evidence.File {
	project := a.currentProject(index)
	if project.Path == "" {
		return nil
	}
	found := evidence.Scan(project.Path, 200)
	if found == nil {
		return []evidence.File{}
	}
	return found
}

// pathUnderProject valida que target esté bajo algún project.Path configurado
// (case-insensitive, prefijo completo + separador; igualdad exacta permitida).
func (a *App) pathUnderProject(target string) bool {
	if strings.TrimSpace(target) == "" {
		return false
	}
	clean := filepath.Clean(target)
	for _, p := range a.cfg.Projects() {
		root := filepath.Clean(p.Path)
		if root == "" {
			continue
		}
		if strings.EqualFold(clean, root) {
			return true
		}
		if len(clean) > len(root) &&
			strings.EqualFold(clean[:len(root)], root) &&
			clean[len(root)] == os.PathSeparator {
			return true
		}
	}
	return false
}

// GetEvidenceThumbnail devuelve un data URL base64 (mime real detectado al
// decodificar) si el archivo decodifica como imagen y pesa <2MB; sino "".
// SIN resize: la galería escala por CSS (paridad visual aceptable).
func (a *App) GetEvidenceThumbnail(path string) string {
	if !a.pathUnderProject(path) {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) >= maxThumbnailBytes {
		return ""
	}
	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	mime := ""
	switch format {
	case "png":
		mime = "image/png"
	case "jpeg":
		mime = "image/jpeg"
	default:
		return ""
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

// openWithRundll32 abre target con el handler del sistema (argv simple,
// sin comillas embebidas — constraint global).
func openWithRundll32(target string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", target).Start()
}

func (a *App) OpenHTMLReport(index int) {
	project := a.currentProject(index)
	if project.Path == "" {
		return
	}
	report := evidence.FindHTMLReport(project.Path)
	if report == "" || !a.pathUnderProject(report) {
		return
	}
	openWithRundll32(report)
}

func (a *App) OpenExternally(path string) {
	if !a.pathUnderProject(path) {
		return
	}
	openWithRundll32(filepath.Clean(path))
}

func (a *App) OpenContainingFolder(path string) {
	if !a.pathUnderProject(path) {
		return
	}
	openWithRundll32(filepath.Dir(filepath.Clean(path)))
}

// OpenURL abre una URL en el navegador del sistema (paridad
// QDesktopServices.openUrl en _open_url_for_index).
func (a *App) OpenURL(url string) {
	if url == "" {
		return
	}
	openWithRundll32(url)
}

// ---- Acciones externas por proyecto ----

func (a *App) OpenInExplorer(index int) {
	project := a.currentProject(index)
	if project.Path == "" {
		return
	}
	openWithRundll32(project.Path)
}

func (a *App) OpenTerminal(index int) {
	project := a.currentProject(index)
	path := project.Path
	if path == "" {
		return
	}
	wt := exec.Command("wt.exe", "-d", path)
	if err := wt.Start(); err != nil {
		// Fallback argv-safe: comilla simple dentro de argumento único.
		_ = exec.Command("powershell", "-NoExit", "-Command",
			"Set-Location -LiteralPath '"+path+"'").Start()
	}
}

func (a *App) OpenVSCode(index int) {
	project := a.currentProject(index)
	if project.Path == "" {
		return
	}
	_ = exec.Command("cmd", "/c", "code", project.Path).Start()
}

func (a *App) OpenOpenCode(index int) {
	project := a.currentProject(index)
	if project.Path == "" {
		return
	}
	_ = exec.Command("cmd", "/c", "opencode", project.Path).Start()
}

// ---- Trace viewer ----

// OpenTraceViewer lanza `npx playwright show-trace <path>` vía Runner normal.
// Guard: si ya hay un visor abierto se notifica y no se lanza otro (paridad
// is_running de Python). shutdown()/RestartApp también lo detienen.
func (a *App) OpenTraceViewer(index int, path string) {
	project := a.currentProject(index)
	if project.Path == "" || !a.pathUnderProject(path) {
		return
	}

	var tr *process.Runner
	a.mu.Lock()
	if a.traceRunner != nil && a.traceRunner.IsRunning() {
		a.mu.Unlock()
		a.emitNotify(project.Name, "Trace viewer already open", "error")
		return
	}
	tr = process.NewRunner(process.RunnerCallbacks{
		OnStdout: func(line string) {
			wails.EventsEmit(a.ctx, "trace:log", map[string]interface{}{
				"index": index, "line": line, "isError": false,
			})
		},
		OnStderr: func(line string) {
			wails.EventsEmit(a.ctx, "trace:log", map[string]interface{}{
				"index": index, "line": line, "isError": true,
			})
		},
		OnError: func(desc string) {
			wails.EventsEmit(a.ctx, "trace:log", map[string]interface{}{
				"index": index, "line": desc, "isError": true,
			})
		},
	})
	a.traceRunner = tr
	a.mu.Unlock()

	command := `npx playwright show-trace "` + path + `"`
	if err := tr.Start(command, project.Path, nil); err != nil {
		a.emitNotify(project.Name, "Failed opening trace viewer: "+err.Error(), "error")
	}
}
