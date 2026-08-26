package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"desktop-go/internal/config"
	"desktop-go/internal/models"
	"desktop-go/internal/playwright"
	"desktop-go/internal/scripts"
	"desktop-go/internal/server"
	"desktop-go/internal/utils/detection"
	"desktop-go/internal/utils/git"
)

type App struct {
	ctx context.Context
	mu  sync.Mutex

	cfg     *config.Manager
	servers map[int]*server.Manager

	playwrightManagers map[int]*playwright.Manager
	scriptManagers     map[int]*scripts.Manager

	gitBusy bool // un comando git global a la vez (paridad QProcess único del panel)

	configPath string
}

func NewApp() *App {
	return &App{
		servers:            map[int]*server.Manager{},
		playwrightManagers: map[int]*playwright.Manager{},
		scriptManagers:     map[int]*scripts.Manager{},
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// Paridad Python (main.py): projects.json junto al ejecutable/script,
	// independiente del CWD de lanzamiento.
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	a.configPath = filepath.Join(filepath.Dir(exePath), "projects.json")

	a.cfg, _ = config.NewManager(a.configPath, config.Options{
		OnProjectsChanged: func() { runtime.EventsEmit(a.ctx, "projects:changed") },
		OnError: func(msg string) {
			runtime.EventsEmit(a.ctx, "config:error", map[string]string{"message": msg})
		},
	})
}

// shutdown detiene servidores y managers de playwright/scripts al cerrar la
// ventana (paridad _real_exit).
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	servers := make([]*server.Manager, 0, len(a.servers))
	for _, sm := range a.servers {
		servers = append(servers, sm)
	}
	pms := make([]*playwright.Manager, 0, len(a.playwrightManagers))
	for _, pm := range a.playwrightManagers {
		pms = append(pms, pm)
	}
	scms := make([]*scripts.Manager, 0, len(a.scriptManagers))
	for _, scm := range a.scriptManagers {
		scms = append(scms, scm)
	}
	a.mu.Unlock()
	for _, sm := range servers {
		sm.Stop()
	}
	for _, pm := range pms {
		pm.Stop()
	}
	for _, scm := range scms {
		scm.Stop()
	}
}

func (a *App) GetProjects() []models.Project {
	return a.cfg.Projects()
}

func (a *App) AddProject(p models.Project) []string {
	errs := p.Validate()
	if len(errs) > 0 {
		return errs
	}
	if err := a.cfg.AddProject(p); err != nil {
		return []string{err.Error()}
	}
	return nil
}

func (a *App) UpdateProject(index int, p models.Project) []string {
	errs := p.Validate()
	if len(errs) > 0 {
		return errs
	}
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	a.mu.Lock()
	sm, ok := a.servers[index]
	a.mu.Unlock()
	if ok {
		sm.UpdateProject(p)
	}
	return nil
}

func (a *App) RemoveProject(index int) {
	// Paridad UI Python: no se puede remover con servidor corriendo.
	// Se mira el ESTADO real del manager, no su mera existencia en el mapa
	// (el manager persiste tras Stop para conservar logs/estado).
	a.mu.Lock()
	sm, exists := a.servers[index]
	a.mu.Unlock()
	if exists {
		switch sm.State() {
		case models.StateRunning, models.StateStarting, models.StateStopping:
			a.emitConfigError("Cannot remove project while its server is running")
			return
		}
	}
	// Paridad Python stop→remove: parar playwright/scripts antes de borrar.
	a.mu.Lock()
	pm := a.playwrightManagers[index]
	scm := a.scriptManagers[index]
	a.mu.Unlock()
	if pm != nil {
		pm.Stop()
	}
	if scm != nil {
		scm.Stop()
	}
	_ = a.cfg.RemoveProject(index)
}

func (a *App) TogglePin(index int) {
	_ = a.cfg.TogglePin(index)
}

func (a *App) StartServer(index int) {
	if sm, _, _ := a.ensureManagers(index); sm != nil {
		sm.Start()
	}
}

func (a *App) StopServer(index int) {
	a.mu.Lock()
	sm := a.servers[index]
	a.mu.Unlock()
	if sm != nil {
		sm.Stop()
	}
}

func (a *App) RestartServer(index int) {
	if sm, _, _ := a.ensureManagers(index); sm != nil {
		sm.Restart()
	}
}

type ServerStatus struct {
	State         string  `json:"state"`
	ActivePort    int     `json:"activePort"`
	ActiveURL     string  `json:"activeUrl"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
	FailureReason string  `json:"failureReason"`
	Running       bool    `json:"running"`
}

func (a *App) GetServerStatus(index int) ServerStatus {
	a.mu.Lock()
	sm := a.servers[index]
	a.mu.Unlock()
	if sm == nil {
		return ServerStatus{State: string(models.StateStopped)}
	}
	st := ServerStatus{
		State:         string(sm.State()),
		ActivePort:    sm.ActivePort(),
		ActiveURL:     sm.ActiveURL(),
		FailureReason: sm.FailureReason(),
		Running:       sm.State() == models.StateRunning || sm.State() == models.StateStarting,
	}
	if at, ok := sm.StartedAt(); ok {
		st.UptimeSeconds = time.Since(at).Seconds()
	}
	return st
}

func (a *App) currentProject(index int) models.Project {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return models.Project{}
	}
	return projects[index]
}

// createServerManager extrae el cuerpo original de managerFor: crea el
// server.Manager con callbacks→eventos Wails y lo guarda en el mapa.
func (a *App) createServerManager(index int) *server.Manager {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return nil
	}
	project := projects[index]

	cb := server.Callbacks{
		OnStateChange: func(state models.ServerState) {
			runtime.EventsEmit(a.ctx, "server:state", map[string]interface{}{
				"index": index, "state": string(state),
			})
		},
		OnLog: func(line string, isError bool) {
			runtime.EventsEmit(a.ctx, "server:log", map[string]interface{}{
				"index": index, "line": line, "isError": isError,
			})
		},
		OnReady: func() {
			runtime.EventsEmit(a.ctx, "server:ready", map[string]interface{}{"index": index})
		},
		OnPortDetected: func(port int, url string) {
			runtime.EventsEmit(a.ctx, "server:port_detected", map[string]interface{}{
				"index": index, "port": port, "url": url,
			})
		},
		OnPortMismatch: func(configured, detected int, activeURL string) {
			runtime.EventsEmit(a.ctx, "server:port_mismatch", map[string]interface{}{
				"index": index, "configured": configured, "detected": detected, "url": activeURL,
			})
		},
	}

	sm := server.NewManager(project, cb)

	a.mu.Lock()
	a.servers[index] = sm
	a.mu.Unlock()
	return sm
}

// ensureManagers crea bajo demanda los tres managers del índice (paridad
// _rebuild_managers de Python) y devuelve el triple. Los callbacks emiten
// SIEMPRE fuera del lock (nunca EventsEmit con a.mu tomado).
func (a *App) ensureManagers(index int) (*server.Manager, *playwright.Manager, *scripts.Manager) {
	a.mu.Lock()
	sm, ok := a.servers[index]
	a.mu.Unlock()
	if !ok {
		sm = a.createServerManager(index)
	}
	if sm == nil {
		return nil, nil, nil
	}

	a.mu.Lock()
	pm := a.playwrightManagers[index]
	scm := a.scriptManagers[index]
	a.mu.Unlock()

	project := a.currentProject(index)

	if pm == nil {
		pm = playwright.NewManager(project, sm, playwright.Callbacks{
			OnStateChange: func(state playwright.State) {
				runtime.EventsEmit(a.ctx, "pw:state", map[string]interface{}{
					"index": index, "state": string(state),
				})
			},
			OnLog: func(line string, isError bool) {
				runtime.EventsEmit(a.ctx, "pw:log", map[string]interface{}{
					"index": index, "line": line, "isError": isError,
				})
			},
			OnFinished: func(int) {},
		})
		a.mu.Lock()
		a.playwrightManagers[index] = pm
		a.mu.Unlock()
	}
	if scm == nil {
		scm = scripts.NewManager(project, scripts.Callbacks{
			OnScriptStarted: func(name string) {
				runtime.EventsEmit(a.ctx, "script:started", map[string]interface{}{
					"index": index, "name": name,
				})
			},
			OnScriptFinished: func(name string, code int) {
				runtime.EventsEmit(a.ctx, "script:finished", map[string]interface{}{
					"index": index, "name": name, "exitCode": code,
				})
			},
			OnLog: func(msg string, isError bool) {
				runtime.EventsEmit(a.ctx, "script:log", map[string]interface{}{
					"index": index, "line": msg, "isError": isError,
				})
			},
		})
		a.mu.Lock()
		a.scriptManagers[index] = scm
		a.mu.Unlock()
	}
	return sm, pm, scm
}

func (a *App) emitConfigError(message string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "config:error", map[string]string{"message": message})
	} else {
		fmt.Println("config error:", message)
	}
}

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

// ---- Git bindings ----

func (a *App) GetGitStatus(index int) git.Status {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return git.Status{}
	}
	return git.GetStatusFull(projects[index].Path)
}

var gitActions = map[string][]string{
	"Pull":  {"pull", "--ff-only"},
	"Fetch": {"fetch", "--all", "--prune"},
	"Stash": {"stash"},
}

// GitAction corre Pull/Fetch/Stash asíncrono con streaming de salida.
// Paridad GitPanel._run_command/_on_finished: un solo comando git global a
// la vez; si hay uno en curso se ignora la petición.
func (a *App) GitAction(index int, action string) {
	args, ok := gitActions[action]
	if !ok {
		return
	}
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return
	}
	path := projects[index].Path

	a.mu.Lock()
	if a.gitBusy {
		a.mu.Unlock()
		return // paridad: self._process is not None → ignorar
	}
	a.gitBusy = true
	a.mu.Unlock()

	go func() {
		code, stashClean := a.runGitStreaming(index, action, path, args)

		// Paridad: "[git pull] exited with code N" va al Logs tab.
		runtime.EventsEmit(a.ctx, "git:output", map[string]interface{}{
			"index":   index,
			"text":    "[git " + strings.ToLower(action) + "] exited with code " + strconv.Itoa(code),
			"isError": code != 0,
		})
		runtime.EventsEmit(a.ctx, "git:finished", map[string]interface{}{
			"index": index, "name": action, "exitCode": code, "cleanStash": stashClean,
		})

		a.mu.Lock()
		a.gitBusy = false
		a.mu.Unlock()
	}()
}

// runGitStreaming corre git y streamea stdout/stderr como eventos.
// Devuelve (exitCode, stashClean) donde stashClean replica la detección
// de "No local changes" del panel Qt para el strip de resultado.
func (a *App) runGitStreaming(index int, action, path string, args []string) (int, bool) {
	cmd := git.HiddenCommand(path, args)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return -1, false
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return -1, false
	}
	if err := cmd.Start(); err != nil {
		runtime.EventsEmit(a.ctx, "git:output", map[string]interface{}{
			"index": index, "text": err.Error(), "isError": true,
		})
		return -1, false
	}

	var stdoutBuf bytes.Buffer
	stream := func(f interface{ Read([]byte) (int, error) }, isError bool, buf *bytes.Buffer) {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")
			runtime.EventsEmit(a.ctx, "git:output", map[string]interface{}{
				"index": index, "text": line, "isError": isError,
			})
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); stream(stdoutPipe, false, &stdoutBuf) }()
	go func() { defer wg.Done(); stream(stderrPipe, true, &bytes.Buffer{}) }()
	wg.Wait()

	code := 0
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	cleanStash := action == "Stash" && code == 0 && strings.Contains(stdoutBuf.String(), "No local changes")
	return code, cleanStash
}
