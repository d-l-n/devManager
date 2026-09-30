package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/config"
	"github.com/d-l-n/devmanager/internal/dashboard"
	envpkg "github.com/d-l-n/devmanager/internal/env"
	"github.com/d-l-n/devmanager/internal/logger"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/obscura"
	"github.com/d-l-n/devmanager/internal/playwright"
	"github.com/d-l-n/devmanager/internal/process"
	"github.com/d-l-n/devmanager/internal/scripts"
	"github.com/d-l-n/devmanager/internal/server"
	"github.com/d-l-n/devmanager/internal/workflow"
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

	// Backups (Issue #71): serializa backup manual vs automático.
	backupMu sync.Mutex

	// Dashboard (Issue #64): historial de uptime persistido.
	historyStore *dashboard.Store

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

	// Workflows (Issue #65): historial de runs, dedupe de ejecuciones en
	// curso, último run por workflow (scheduler) y listener entrante.
	// wfMu serializa wfRunning/wfLastRun/ciStatus (nunca EventsEmit con lock).
	wfStore     *workflow.Store
	wfListener  *workflow.Listener
	wfMu        sync.Mutex
	wfRunning   map[string]bool
	wfLastRun   map[string]time.Time
	wfSchedStop chan struct{}
	ciStatus    map[string]CIStatusEntry
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
		wfRunning:          map[string]bool{},
		wfLastRun:          map[string]time.Time{},
		ciStatus:           map[string]CIStatusEntry{},
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

	// Backups automáticos (Issue #71): chequeo al arrancar (cubre "la app
	// estuvo cerrada") + ticker de 15 min mientras la app esté abierta.
	a.startBackupScheduler()

	// Dashboard (Issue #64): sampler de uptime cada 60s + store persistido.
	a.startDashboardSampler()

	// Workflows (Issue #65): historial + scheduler 60s + listener entrante.
	a.initWorkflows()
	a.startWorkflowScheduler()
	a.startWorkflowListener()

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
	a.stopWorkflowScheduler()
	a.stopWorkflowListener()
	a.flushDashboardHistory()
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

// SetActiveEnv cambia el entorno activo del proyecto (Fase 1 #67).
// Bloquea si el servidor está corriendo (running/starting/stopping).
func (a *App) SetActiveEnv(index int, env string) []string {
	a.mu.Lock()
	sm, exists := a.servers[index]
	a.mu.Unlock()
	if exists {
		switch sm.State() {
		case models.StateRunning, models.StateStarting, models.StateStopping:
			return []string{"Cannot switch environment while server is running"}
		}
	}
	if err := a.cfg.SetActiveEnv(index, env); err != nil {
		return []string{err.Error()}
	}
	projects := a.cfg.Projects()
	a.mu.Lock()
	sm2 := a.servers[index]
	a.mu.Unlock()
	if sm2 != nil && index < len(projects) {
		sm2.UpdateProject(projects[index])
	}
	wails.EventsEmit(a.ctx, "env:changed", map[string]interface{}{"index": index, "env": env})
	return nil
}

// resolveEnvFile resuelve el archivo dotenv dentro del proyecto.
// Rechaza rutas absolutas y escapes `..`. Nunca incluye valores.
func (a *App) resolveEnvFile(p models.Project, envName string) (string, error) {
	e, ok := p.Envs[envName]
	if !ok {
		return "", fmt.Errorf("unknown environment %q", envName)
	}
	file := e.EnvFile
	if file == "" {
		file = models.DefaultEnvFile(envName)
	}
	if filepath.IsAbs(file) {
		return "", fmt.Errorf("invalid env file %q", file)
	}
	for _, seg := range strings.Split(filepath.ToSlash(file), "/") {
		if seg == ".." {
			return "", fmt.Errorf("invalid env file %q", file)
		}
	}
	return filepath.Join(p.Path, file), nil
}

// serverRunning reporta si el servidor del índice está vivo
// (running/starting/stopping). Centraliza el bloqueo de Fase 1/2/3.
func (a *App) serverRunning(index int) bool {
	a.mu.Lock()
	sm, exists := a.servers[index]
	a.mu.Unlock()
	if !exists {
		return false
	}
	switch sm.State() {
	case models.StateRunning, models.StateStarting, models.StateStopping:
		return true
	}
	return false
}

// GetEnvVars devuelve las vars del entorno (copia; vacío si no existe).
// Fase 3: enmascara secretos por default; solo reveal=true explícito
// devuelve valores reales. Nunca emite valores a logs/eventos.
func (a *App) GetEnvVars(index int, envName string, reveal bool) map[string]string {
	vars, err := a.cfg.GetEnvVars(index, envName)
	if err != nil {
		return map[string]string{}
	}
	if reveal {
		return vars
	}
	secrets, err := a.cfg.GetSecrets(index, envName)
	if err != nil {
		return map[string]string{}
	}
	return models.MaskedVars(vars, secrets)
}

// GetSecrets devuelve la lista de keys secretas del entorno (solo keys,
// nunca valores).
func (a *App) GetSecrets(index int, envName string) []string {
	secrets, err := a.cfg.GetSecrets(index, envName)
	if err != nil {
		return []string{}
	}
	return secrets
}

// SetSecretKey marca una key como secreta. Bloquea con server running.
func (a *App) SetSecretKey(index int, envName, key string) []string {
	if a.serverRunning(index) {
		return []string{"Cannot edit environment variables while server is running"}
	}
	if err := a.cfg.SetSecretKey(index, envName, key); err != nil {
		return []string{err.Error()}
	}
	wails.EventsEmit(a.ctx, "env:changed", map[string]interface{}{"index": index, "env": envName})
	return nil
}

// UnsetSecretKey desmarca una key como secreta. Bloquea con server running.
func (a *App) UnsetSecretKey(index int, envName, key string) []string {
	if a.serverRunning(index) {
		return []string{"Cannot edit environment variables while server is running"}
	}
	if err := a.cfg.UnsetSecretKey(index, envName, key); err != nil {
		return []string{err.Error()}
	}
	wails.EventsEmit(a.ctx, "env:changed", map[string]interface{}{"index": index, "env": envName})
	return nil
}

// GetEnvDiff compara vars entre dos envs del proyecto (Fase 3).
// Valores secretos viajan ya enmascarados; nunca se loguean valores.
func (a *App) GetEnvDiff(index int, envA, envB string) []envpkg.DiffRow {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return nil
	}
	mask := func(envName string) map[string]string {
		vars, err := a.cfg.GetEnvVars(index, envName)
		if err != nil {
			return nil
		}
		secrets, _ := a.cfg.GetSecrets(index, envName)
		return models.MaskedVars(vars, secrets)
	}
	va := mask(envA)
	if va == nil {
		a.emitConfigError("Unknown environment for diff")
		return nil
	}
	vb := mask(envB)
	if vb == nil {
		a.emitConfigError("Unknown environment for diff")
		return nil
	}
	return envpkg.Diff(va, vb)
}

// SetEnvVars reemplaza las vars del entorno (Fase 2 #67).
// Bloquea si el servidor está corriendo (igual que SetActiveEnv).
func (a *App) SetEnvVars(index int, envName string, vars map[string]string) []string {
	if a.serverRunning(index) {
		return []string{"Cannot edit environment variables while server is running"}
	}
	if err := a.cfg.SetEnvVars(index, envName, vars); err != nil {
		return []string{err.Error()}
	}
	projects := a.cfg.Projects()
	a.mu.Lock()
	sm2 := a.servers[index]
	a.mu.Unlock()
	if sm2 != nil && index < len(projects) {
		sm2.UpdateProject(projects[index])
	}
	wails.EventsEmit(a.ctx, "env:changed", map[string]interface{}{"index": index, "env": envName})
	return nil
}

// LoadDotEnvFile lee <project.Path>/<EnvFile> y lo parsea.
// Archivo ausente o inválido → mapa vacío (el error de parseo se notifica
// sin valores).
func (a *App) LoadDotEnvFile(index int, envName string) map[string]string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return map[string]string{}
	}
	full, err := a.resolveEnvFile(projects[index], envName)
	if err != nil {
		return map[string]string{}
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return map[string]string{}
	}
	vars, err := envpkg.Parse(data)
	if err != nil {
		a.emitConfigError("Failed to parse env file")
		return map[string]string{}
	}
	return vars
}

// SaveDotEnvFile serializa vars a <project.Path>/<EnvFile>.
func (a *App) SaveDotEnvFile(index int, envName string, vars map[string]string) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	if vars == nil {
		vars = map[string]string{}
	}
	for k := range vars {
		if err := envpkg.ValidateKey(k); err != nil {
			return []string{err.Error()}
		}
	}
	full, err := a.resolveEnvFile(projects[index], envName)
	if err != nil {
		return []string{err.Error()}
	}
	if err := os.WriteFile(full, []byte(envpkg.Serialize(vars)), 0o644); err != nil {
		return []string{err.Error()}
	}
	return nil
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
			// Workflows (Issue #65): fan-out de eventos sin bloquear.
			switch state {
			case models.StateRunning:
				go a.FireWorkflowEvent(index, models.WorkflowEventServerStarted, nil)
			case models.StateStopped:
				go a.FireWorkflowEvent(index, models.WorkflowEventServerStopped, nil)
			}
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
			// Workflows (Issue #65): tests terminados → evento sin bloquear.
			OnFinished: func(exitCode int) {
				go a.FireWorkflowEvent(index, models.WorkflowEventTestsFinished,
					map[string]interface{}{"exitCode": exitCode})
			},
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
