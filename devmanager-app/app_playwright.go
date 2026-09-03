package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/obscura"
	"github.com/d-l-n/devmanager/internal/playwright"
	"github.com/d-l-n/devmanager/internal/utils/detection"
)

// ---- Playwright bindings ----

func (a *App) RunTests(index int) {
	if _, pm, _ := a.ensureManagers(index); pm != nil {
		pm.RunTests()
	}
}

func (a *App) RunUI(index int) {
	if _, pm, _ := a.ensureManagers(index); pm != nil {
		pm.RunUI()
	}
}

func (a *App) RunDebug(index int) {
	if _, pm, _ := a.ensureManagers(index); pm != nil {
		pm.RunDebug()
	}
}

func (a *App) ShowReport(index int) {
	if _, pm, _ := a.ensureManagers(index); pm != nil {
		pm.ShowReport()
	}
}

func (a *App) StopPlaywright(index int) {
	a.mu.Lock()
	pm := a.playwrightManagers[index]
	a.mu.Unlock()
	if pm != nil {
		pm.Stop()
	}
}

type PlaywrightStatus struct {
	State string `json:"state"`
}

func (a *App) GetPlaywrightStatus(index int) PlaywrightStatus {
	a.mu.Lock()
	pm := a.playwrightManagers[index]
	a.mu.Unlock()
	if pm == nil {
		return PlaywrightStatus{State: string(playwright.StateIdle)}
	}
	return PlaywrightStatus{State: string(pm.State())}
}

// ---- Scripts bindings ----

func (a *App) GetScripts(index int) []detection.Script {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return nil
	}
	return detection.GetProjectScripts(projects[index].Path)
}

func (a *App) RunScript(index int, name string, command string) {
	if _, _, scm := a.ensureManagers(index); scm != nil {
		scm.RunScript(name, command)
	}
}

func (a *App) StopScript(index int) {
	a.mu.Lock()
	scm := a.scriptManagers[index]
	a.mu.Unlock()
	if scm != nil {
		scm.Stop()
	}
}

type ScriptStatus struct {
	Running    bool   `json:"running"`
	ActiveName string `json:"activeName"`
}

func (a *App) GetScriptStatus(index int) ScriptStatus {
	a.mu.Lock()
	scm := a.scriptManagers[index]
	a.mu.Unlock()
	if scm == nil {
		return ScriptStatus{}
	}
	return ScriptStatus{Running: scm.IsRunning(), ActiveName: scm.ActiveScriptName()}
}

// ---- Obscura bindings ----
// Obscura (github.com/h4ckf0r0day/obscura) como herramienta auxiliar:
// screenshots, dump text/markdown y eval JS sobre la URL del proyecto.
// El binario se descarga on-demand a %APPDATA%\devManager\obscura (verificado
// por SHA-256); NUNCA se trackea en el repo.

// ensureObscuraManager crea el manager del índice bajo demanda (paridad
// ensureManagers). El callback emite eventos Wails fuera del lock.
func (a *App) ensureObscuraManager(index int) *obscura.Manager {
	a.mu.Lock()
	om := a.obscuraManagers[index]
	a.mu.Unlock()
	if om != nil {
		return om
	}
	om = obscura.NewManager(obscura.Callbacks{
		OnStateChange: func(state obscura.State) {
			wails.EventsEmit(a.ctx, "obs:state", map[string]interface{}{
				"index": index, "state": string(state),
			})
		},
		OnLog: func(line string, isError bool) {
			wails.EventsEmit(a.ctx, "obs:log", map[string]interface{}{
				"index": index, "line": line, "isError": isError,
			})
		},
	})
	a.mu.Lock()
	a.obscuraManagers[index] = om
	a.mu.Unlock()
	return om
}

type ObscuraStatus struct {
	State        string `json:"state"`
	BinaryExists bool   `json:"binaryExists"`
	BinaryPath   string `json:"binaryPath"`
}

func (a *App) GetObscuraStatus(index int) ObscuraStatus {
	om := a.ensureObscuraManager(index)
	if om == nil {
		return ObscuraStatus{State: string(obscura.StateIdle)}
	}
	return ObscuraStatus{
		State:        string(om.State()),
		BinaryExists: obscura.HasBinary(),
		BinaryPath:   obscura.BinPath(),
	}
}

// ObscuraScreenshot guarda un screenshot de url en test-results/obscura/ del
// proyecto (el tab Evidence ya escanea test-results/ → visible sin cambios).
func (a *App) ObscuraScreenshot(index int, url string) error {
	om := a.ensureObscuraManager(index)
	if om == nil {
		return fmt.Errorf("project index %d out of range", index)
	}
	project := a.currentProject(index)
	if project.Path == "" {
		return fmt.Errorf("project index %d out of range", index)
	}
	outDir := filepath.Join(project.Path, "test-results", "obscura")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("crear directorio de evidencias: %w", err)
	}
	outPath := filepath.Join(outDir, "obscura_"+time.Now().Format("20060102_150405")+".png")
	return om.Screenshot(url, outPath)
}

func (a *App) ObscuraDump(index int, url, format string) error {
	om := a.ensureObscuraManager(index)
	if om == nil {
		return fmt.Errorf("project index %d out of range", index)
	}
	return om.Dump(url, format)
}

func (a *App) ObscuraEval(index int, url, js string) error {
	om := a.ensureObscuraManager(index)
	if om == nil {
		return fmt.Errorf("project index %d out of range", index)
	}
	return om.Eval(url, js)
}

func (a *App) ObscuraFetch(index int, command string) error {
	om := a.ensureObscuraManager(index)
	if om == nil {
		return fmt.Errorf("project index %d out of range", index)
	}
	return om.Fetch(command)
}

func (a *App) StopObscura(index int) {
	a.mu.Lock()
	om := a.obscuraManagers[index]
	a.mu.Unlock()
	if om != nil {
		om.Stop()
	}
}
