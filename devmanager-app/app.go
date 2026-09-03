package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/config"
	"github.com/d-l-n/devmanager/internal/logger"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/obscura"
	"github.com/d-l-n/devmanager/internal/playwright"
	"github.com/d-l-n/devmanager/internal/process"
	"github.com/d-l-n/devmanager/internal/scripts"
	"github.com/d-l-n/devmanager/internal/server"
)

type App struct {
	ctx context.Context
	mu  sync.Mutex

	cfg     *config.Manager
	servers map[int]*server.Manager

	playwrightManagers map[int]*playwright.Manager
	scriptManagers     map[int]*scripts.Manager
	obscuraManagers    map[int]*obscura.Manager

	traceRunner *process.Runner

	gitBusy map[int]bool // un comando git a la vez POR proyecto (paridad QProcess único del panel)

	configPath string

	settingsPath string
	settings     config.Settings

	trayOK    bool // spike tray: Register completó onTrayReady
	forceExit bool // quit real desde tray/atajo: OnBeforeClose no debe ocultar

	appLog     *logger.Ring // App Log global (captura stdout/stderr, ring 3000)
	restoreLog func()       // restaura os.Stdout/os.Stderr originales

	// Notificaciones nativas (paridad _notify de Python): cooldown 3s de
	// notificaciones de bandeja cuando la ventana está oculta + pendiente.
	lastTrayNotify time.Time
	pendingNotify  *pendingTrayNotify
	notifyMu       sync.Mutex
	windowHidden   bool   // oculta a bandeja via beforeClose (paridad isVisible)
	traySig        string // firma del menú de bandeja (evita rebuilds innecesarios)

	// Cache del monitor (Issue #44): debounce 500ms para que ráfagas de
	// GetMonitorData no apilen scans de puertos + árboles de proceso.
	monitorCache    MonitorData
	monitorCachedAt time.Time
	monitorValid    bool
	monitorMu       sync.Mutex
}

// pendingTrayNotify guarda la última notificación durante el cooldown;
// se muestra al vencer éste (paridad _flush_pending_tray_notify).
type pendingTrayNotify struct {
	title   string
	message string
	isError bool
}

func NewApp() *App {
	return &App{
		servers:            map[int]*server.Manager{},
		playwrightManagers: map[int]*playwright.Manager{},
		scriptManagers:     map[int]*scripts.Manager{},
		obscuraManagers:    map[int]*obscura.Manager{},
		gitBusy:            map[int]bool{},
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
		OnProjectsChanged: func() { wails.EventsEmit(a.ctx, "projects:changed") },
		OnError: func(msg string) {
			wails.EventsEmit(a.ctx, "config:error", map[string]string{"message": msg})
		},
	})

	// Settings persistentes en %APPDATA%\devManager\settings.json (spec §4),
	// independiente del CWD y del exe.
	settingsDir, err := os.UserConfigDir()
	if err != nil || settingsDir == "" {
		settingsDir = "."
	}
	a.settingsPath = filepath.Join(settingsDir, "devManager", "settings.json")
	a.mu.Lock()
	a.settings = config.LoadSettings(a.settingsPath)
	a.mu.Unlock()

	// App Log global (Task 14): ring de 3000 líneas capturando stdout/stderr.
	// El callback emite el evento; a.ctx ya está set. Null si Attach falla.
	a.appLog = logger.New(3000)
	a.appLog.SetOnLine(func(e logger.Entry) {
		if a.ctx != nil {
			wails.EventsEmit(a.ctx, "applog:line", e)
		}
	})
	a.restoreLog = a.appLog.Attach()

	// Spike tray (Fase 3 §5.1): el pump se lanza desde main() vía runTray;
	// onTrayReady marca trayOK y OnBeforeClose oculta salvo forceExit.
}

// beforeClose replica closeEvent del Python: ocultar a bandeja salvo salida real.
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	trayOK := a.trayOK
	force := a.forceExit
	a.mu.Unlock()
	if trayOK && !force {
		a.mu.Lock()
		a.windowHidden = true
		a.mu.Unlock()
		wails.WindowHide(ctx)
		return true // impide el cierre real
	}
	return false
}

// showMainWindow muestra/foca la ventana principal desde tray o click-to-focus
// (paridad _on_activated Trigger/DoubleClick del Python) y marca visible.
func (a *App) showMainWindow() {
	a.setWindowShown()
	if a.ctx != nil {
		wails.WindowUnminimise(a.ctx)
		wails.WindowShow(a.ctx)
	}
}

// stopAllRunners detiene servidores, playwright, scripts y el trace viewer.
// Usado por shutdown y por RestartApp. Los Stop() corren FUERA del lock
// (pueden bloquear hasta 5s esperando taskkill).
func (a *App) stopAllRunners() {
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
	trace := a.traceRunner
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
	if trace != nil && trace.IsRunning() {
		trace.Stop()
	}
}

// shutdown detiene servidores y managers de playwright/scripts al cerrar la
// ventana (paridad _real_exit).
func (a *App) shutdown(ctx context.Context) {
	a.stopAllRunners()
	if a.restoreLog != nil {
		a.restoreLog()
		a.restoreLog = nil
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
	om := a.obscuraManagers[index]
	a.mu.Unlock()
	if pm != nil {
		pm.Stop()
	}
	if scm != nil {
		scm.Stop()
	}
	if om != nil {
		om.Stop()
	}
	_ = a.cfg.RemoveProject(index)
}

func (a *App) TogglePin(index int) {
	_ = a.cfg.TogglePin(index)
}

// AutoAssignPorts replica _on_auto_assign_unique_ports: asigna puertos
// secuenciales únicos (desde 5173) a todos los proyectos con servidor
// habilitado; devuelve cuántos se modificaron (paridad count => toast).
func (a *App) AutoAssignPorts() int {
	return a.cfg.AutoAssignUniquePorts(5173)
}

// SaveDetectedPort confirma en projects.json el puerto detectado por el
// servidor. También actualiza el manager vivo para que los siguientes flujos
// usen el nuevo puerto sin reiniciar la aplicación.
func (a *App) SaveDetectedPort(index, port int) []string {
	if err := a.cfg.SaveDetectedPort(index, port); err != nil {
		return []string{err.Error()}
	}
	projects := a.cfg.Projects()
	a.mu.Lock()
	sm := a.servers[index]
	a.mu.Unlock()
	if sm != nil && index < len(projects) {
		sm.UpdateProject(projects[index])
	}
	return nil
}

// ReloadProjects replica config_manager.load(): relee projects.json desde
// disco y re-emite projects:changed para que el frontend refresque la lista.
// Los managers se reconstruyen bajo demanda vía ensureManagers (paridad
// _rebuild_managers de Python).
func (a *App) ReloadProjects() {
	a.cfg.Load()
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
			wails.EventsEmit(a.ctx, "server:state", map[string]interface{}{
				"index": index, "state": string(state),
			})
		},
		OnLog: func(line string, isError bool) {
			wails.EventsEmit(a.ctx, "server:log", map[string]interface{}{
				"index": index, "line": line, "isError": isError,
			})
		},
		OnReady: func() {
			wails.EventsEmit(a.ctx, "server:ready", map[string]interface{}{"index": index})
		},
		OnPortDetected: func(port int, url string) {
			wails.EventsEmit(a.ctx, "server:port_detected", map[string]interface{}{
				"index": index, "port": port, "url": url,
			})
		},
		OnPortMismatch: func(configured, detected int, activeURL string) {
			wails.EventsEmit(a.ctx, "server:port_mismatch", map[string]interface{}{
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
				wails.EventsEmit(a.ctx, "pw:state", map[string]interface{}{
					"index": index, "state": string(state),
				})
			},
			OnLog: func(line string, isError bool) {
				wails.EventsEmit(a.ctx, "pw:log", map[string]interface{}{
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
				wails.EventsEmit(a.ctx, "script:started", map[string]interface{}{
					"index": index, "name": name,
				})
			},
			OnScriptFinished: func(name string, code int) {
				wails.EventsEmit(a.ctx, "script:finished", map[string]interface{}{
					"index": index, "name": name, "exitCode": code,
				})
				if code != 0 {
					a.emitNotify(project.Name, fmt.Sprintf("Script '%s' exited with code %d", name, code), "error")
				}
			},
			OnLog: func(msg string, isError bool) {
				wails.EventsEmit(a.ctx, "script:log", map[string]interface{}{
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

// ---- Quit ----

// Quit cierra la app real (confirm lo hace el frontend); shutdown() detiene
// servidores y managers vía OnBeforeClose/shutdown.
func (a *App) Quit() {
	a.mu.Lock()
	a.forceExit = true
	a.mu.Unlock()
	wails.Quit(a.ctx)
}

// ---- Restart app ----

// RestartApp para todos los runners, relanza el propio exe detached y sale
// (paridad QProcess.startDetached(sys.executable)).
func (a *App) RestartApp() {
	a.stopAllRunners()
	exePath, err := os.Executable()
	if err == nil && exePath != "" {
		_ = exec.Command(exePath).Start()
	}
	wails.Quit(a.ctx)
}
