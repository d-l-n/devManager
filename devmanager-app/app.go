package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/google/uuid"

	"github.com/d-l-n/devmanager/internal/config"
	"github.com/d-l-n/devmanager/internal/logger"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/playwright"
	"github.com/d-l-n/devmanager/internal/process"
	"github.com/d-l-n/devmanager/internal/scripts"
	"github.com/d-l-n/devmanager/internal/server"
	"github.com/d-l-n/devmanager/internal/sysmon"
	"github.com/d-l-n/devmanager/internal/utils/detection"
	"github.com/d-l-n/devmanager/internal/utils/evidence"
	"github.com/d-l-n/devmanager/internal/utils/git"
)

type App struct {
	ctx context.Context
	mu  sync.Mutex

	cfg     *config.Manager
	servers map[int]*server.Manager

	playwrightManagers map[int]*playwright.Manager
	scriptManagers     map[int]*scripts.Manager

	traceRunner *process.Runner

	gitBusy map[int]bool // un comando git a la vez POR proyecto (paridad QProcess ├║nico del panel)

	configPath string

	settingsPath string
	settings     config.Settings

	trayOK    bool // spike tray: Register complet├│ onTrayReady
	forceExit bool // quit real desde tray/atajo: OnBeforeClose no debe ocultar

	appLog     *logger.Ring // App Log global (captura stdout/stderr, ring 3000)
	restoreLog func()       // restaura os.Stdout/os.Stderr originales

	// Notificaciones nativas (paridad _notify de Python): cooldown 3s de
	// notificaciones de bandeja cuando la ventana est├í oculta + pendiente.
	lastTrayNotify time.Time
	pendingNotify  *pendingTrayNotify
	notifyMu       sync.Mutex
	windowHidden   bool   // oculta a bandeja via beforeClose (paridad isVisible)
	traySig        string // firma del men├║ de bandeja (evita rebuilds innecesarios)
}

// pendingTrayNotify guarda la ├║ltima notificaci├│n durante el cooldown;
// se muestra al vencer ├®ste (paridad _flush_pending_tray_notify).
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
		OnProjectsChanged: func() { runtime.EventsEmit(a.ctx, "projects:changed") },
		OnError: func(msg string) {
			runtime.EventsEmit(a.ctx, "config:error", map[string]string{"message": msg})
		},
	})

	// Settings persistentes en %APPDATA%\devManager\settings.json (spec ┬º4),
	// independiente del CWD y del exe.
	settingsDir, err := os.UserConfigDir()
	if err != nil || settingsDir == "" {
		settingsDir = "."
	}
	a.settingsPath = filepath.Join(settingsDir, "devManager", "settings.json")
	a.mu.Lock()
	a.settings = config.LoadSettings(a.settingsPath)
	a.mu.Unlock()

	// App Log global (Task 14): ring de 3000 l├¡neas capturando stdout/stderr.
	// El callback emite el evento; a.ctx ya est├í set. Null si Attach falla.
	a.appLog = logger.New(3000)
	a.appLog.SetOnLine(func(e logger.Entry) {
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "applog:line", e)
		}
	})
	a.restoreLog = a.appLog.Attach()

	// Spike tray (Fase 3 ┬º5.1): el pump se lanza desde main() v├¡a runTray;
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
		runtime.WindowHide(ctx)
		return true // impide el cierre real
	}
	return false
}

// showMainWindow muestra/foca la ventana principal desde tray o click-to-focus
// (paridad _on_activated Trigger/DoubleClick del Python) y marca visible.
func (a *App) showMainWindow() {
	a.setWindowShown()
	if a.ctx != nil {
		runtime.WindowUnminimise(a.ctx)
		runtime.WindowShow(a.ctx)
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
	// Paridad Python stopÔåÆremove: parar playwright/scripts antes de borrar.
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

// AutoAssignPorts replica _on_auto_assign_unique_ports: asigna puertos
// secuenciales ├║nicos (desde 5173) a todos los proyectos con servidor
// habilitado; devuelve cu├íntos se modificaron (paridad count => toast).
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
// Los managers se reconstruyen bajo demanda v├¡a ensureManagers (paridad
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
// server.Manager con callbacksÔåÆeventos Wails y lo guarda en el mapa.
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

// ensureManagers crea bajo demanda los tres managers del ├¡ndice (paridad
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
				if code != 0 {
					a.emitNotify(project.Name, fmt.Sprintf("Script '%s' exited with code %d", name, code), "error")
				}
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

// emitNotify replica _notify del Python: ventana visible ÔåÆ toast in-app;
// oculta/minimizada ÔåÆ notificaci├│n nativa de bandeja con cooldown de 3s.
func (a *App) emitNotify(title, message, level string) {
	if a.ctx != nil {
		if a.windowVisible() {
			runtime.EventsEmit(a.ctx, "notify", map[string]string{
				"title": title, "message": message, "level": level,
			})
			return
		}
	}
	a.nativeNotify(title, message, level == "error" || level == "warning")
}

// windowVisible replica self.isVisible() del Python usando el flag de
// ocultar-a-bandeja (beforeClose). La ventana minimizada cuenta como visible.
func (a *App) windowVisible() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.windowHidden
}

// setWindowShown marca la ventana como visible (desde tray/show/click-to-focus).
func (a *App) setWindowShown() {
	a.mu.Lock()
	a.windowHidden = false
	a.mu.Unlock()
}

// nativeNotify muestra una notificaci├│n de bandeja con cooldown de 3s y
// buffer de una pendiente (paridad _notify/_flush_pending del Python).
func (a *App) nativeNotify(title, message string, isError bool) {
	a.notifyMu.Lock()
	now := time.Now()
	if now.Sub(a.lastTrayNotify) < 3*time.Second {
		a.pendingNotify = &pendingTrayNotify{title: title, message: message, isError: isError}
		a.notifyMu.Unlock()
		return
	}
	a.lastTrayNotify = now
	a.notifyMu.Unlock()

	a.showNativeBalloon(title, message, isError)

	// Programar el flush de la pendiente (paridad QTimer.singleShot(3200)).
	go func() {
		time.Sleep(3200 * time.Millisecond)
		a.notifyMu.Lock()
		defer a.notifyMu.Unlock()
		if a.pendingNotify != nil {
			p := a.pendingNotify
			a.pendingNotify = nil
			a.lastTrayNotify = time.Now()
			a.showNativeBalloon(p.title, p.message, p.isError)
		}
	}()
}

// showNativeBalloon emite una burbuja nativa de bandeja v├¡a PowerShell
// (System.Windows.Forms.NotifyIcon), fiel a QSystemTrayIcon.showMessage y
// sin necesidad de registrar AppUserModelID. Fallo ÔåÆ toast de respaldo.
func (a *App) showNativeBalloon(title, message string, isError bool) {
	icon := "Info"
	if isError {
		icon = "Error"
	}
	escQuote := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	t, m := escQuote(title), escQuote(message)
	ps := `[void][reflection.assembly]::loadwithpartialname('System.Windows.Forms');` +
		`[void][reflection.assembly]::loadwithpartialname('System.Drawing');` +
		`$n=New-Object system.windows.forms.notifyicon;` +
		`$n.icon=[system.drawing.systemicons]::Information;` +
		`$n.visible=$true;` +
		`$n.showballoontip(3500,'` + t + `','` + m + `',[system.windows.forms.tooltipicon]::` + icon + `);`
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", ps)
	if err := cmd.Start(); err != nil {
		// Respaldo: toast normal
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "notify", map[string]string{
				"title": title, "message": message, "level": "warning",
			})
		}
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

// ---- Backlog bindings (feature) ----
// Los items viven en models.Project.Backlog y se persisten vía config.Manager.

// emitBacklogChanged notifica al frontend para refrescar el panel del proyecto.
func (a *App) emitBacklogChanged(index int) {
	runtime.EventsEmit(a.ctx, "backlog:changed", map[string]int{"projectIndex": index})
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
	now := time.Now()
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
			p.Backlog[i].UpdatedAt = time.Now()
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

// GitAction corre Pull/Fetch/Stash as├¡ncrono con streaming de salida.
// Paridad GitPanel._run_command/_on_finished: un solo comando git a la vez
// POR proyecto; si ese proyecto ya tiene uno en curso se ignora la petici├│n.
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
	projectName := projects[index].Name

	a.mu.Lock()
	if a.gitBusy[index] {
		a.mu.Unlock()
		return // paridad: self._process is not None ÔåÆ ignorar
	}
	a.gitBusy[index] = true
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
		if code != 0 {
			a.emitNotify(projectName, fmt.Sprintf("Git %s failed (exit %d)", action, code), "error")
		}

		a.mu.Lock()
		a.gitBusy[index] = false
		a.mu.Unlock()
	}()
}

// runGitStreaming corre git y streamea stdout/stderr como eventos.
// Devuelve (exitCode, stashClean) donde stashClean replica la detecci├│n
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

// ---- Monitor bindings (paridad _refresh_monitor_data) ----

type PortRow struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	State     string `json:"state"` // "ours" | "foreign" | "free"
	OwnerName string `json:"ownerName"`
	OwnerPID  int    `json:"ownerPID"`
}

type ResRow struct {
	Name     string  `json:"name"`
	PID      int     `json:"pid"`
	Children int     `json:"children"`
	CPU      float64 `json:"cpu"`
	RSS      float64 `json:"rss"`
}

type MonitorData struct {
	PortRows []PortRow `json:"portRows"`
	ResRows  []ResRow  `json:"resRows"`
}

type runningServer struct {
	index   int
	project models.Project
	manager *server.Manager
}

// runningServersSnapshot copia bajo lock el mapa de managers junto a los
// proyectos actuales, qued├índose solo con los RUNNING. Orden estable por ├¡ndice.
func (a *App) runningServersSnapshot() []runningServer {
	a.mu.Lock()
	servers := make(map[int]*server.Manager, len(a.servers))
	for idx, sm := range a.servers {
		servers[idx] = sm
	}
	a.mu.Unlock()

	projects := a.cfg.Projects()
	out := make([]runningServer, 0, len(servers))
	for idx, sm := range servers {
		if idx < 0 || idx >= len(projects) || sm.State() != models.StateRunning {
			continue
		}
		out = append(out, runningServer{index: idx, project: projects[idx], manager: sm})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].index < out[j].index })
	return out
}

func (a *App) GetMonitorData() MonitorData {
	running := a.runningServersSnapshot()

	// Mapa puerto activo ÔåÆ nombre de proyecto (solo servers RUNNING propios).
	oursByPort := map[int]string{}
	for _, rs := range running {
		port := rs.manager.ActivePort()
		if port > 0 {
			if _, taken := oursByPort[port]; !taken {
				oursByPort[port] = rs.project.Name
			}
		}
	}

	data := MonitorData{PortRows: []PortRow{}, ResRows: []ResRow{}}
	projects := a.cfg.Projects()
	for i, p := range projects {
		if !p.Server.Enabled || p.Server.Port <= 0 {
			continue
		}
		row := PortRow{Index: i, Name: p.Name, Port: p.Server.Port, State: "free"}
		if owner, ok := oursByPort[p.Server.Port]; ok {
			row.State = "ours"
			row.OwnerName = owner
		} else if o := sysmon.GetPortOwner(p.Server.Port); o != nil {
			row.State = "foreign"
			row.OwnerName = o.Name
			row.OwnerPID = o.PID
		}
		data.PortRows = append(data.PortRows, row)
	}

	a.mu.Lock()
	polling := a.settings.MonitorPolling
	a.mu.Unlock()
	if polling {
		for _, rs := range running {
			pid := rs.manager.PID()
			if pid <= 0 {
				continue
			}
			u := sysmon.GetProcessTreeUsage(pid)
			if u == nil {
				continue
			}
			data.ResRows = append(data.ResRows, ResRow{
				Name: rs.project.Name, PID: u.PID, Children: u.Children,
				CPU: u.CPUPercent, RSS: u.RSSMB,
			})
		}
	}
	return data
}

// ---- Kill tree (confirm lo hace el frontend) ----

type NotifyResult struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

func (a *App) KillTree(pid int) NotifyResult {
	ok, msg := sysmon.KillTree(pid)
	level := "error"
	message := fmt.Sprintf("Failed killing PID %d: %s", pid, msg)
	title := "System"
	if ok {
		level = "success"
		message = fmt.Sprintf("Process tree %d terminated", pid)
	}
	a.emitNotify(title, message, level)
	return NotifyResult{Ok: ok, Message: msg}
}

// ---- App Log global (paridad app_logger.py) ----

// GetAppLog devuelve el historial capturado del App Log (ring buffer).
func (a *App) GetAppLog() []logger.Entry {
	if a.appLog == nil {
		return []logger.Entry{}
	}
	return a.appLog.History()
}

// ClearAppLog vac├¡a el App Log global.
func (a *App) ClearAppLog() {
	if a.appLog != nil {
		a.appLog.Clear()
		a.emitNotify("Application Log", "App log cleared", "info")
	}
}

// ---- Detecci├│n de config de proyecto (paridad detect_project_config) ----

// DetectProjectConfig expone la autodetecci├│n al frontend. existingPorts son
// los puertos ya configurados (evita colisiones en el di├ílogo de proyecto).
func (a *App) DetectProjectConfig(path string) detection.ProjectConfig {
	return detection.DetectProjectConfig(path, a.cfg.ConfiguredPorts())
}

// BrowseFolder abre un di├ílogo nativo de selecci├│n de carpeta (paridad
// QFileDialog.getExistingDirectory). Devuelve "" si el usuario cancela.
func (a *App) BrowseFolder() string {
	if a.ctx == nil {
		return ""
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Project Directory",
	})
	if err != nil {
		return ""
	}
	return dir
}

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

// pathUnderProject valida que target est├® bajo alg├║n project.Path configurado
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
// SIN resize: la galer├¡a escala por CSS (paridad visual aceptable).
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
// sin comillas embebidas ÔÇö constraint global).
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
		// Fallback argv-safe: comilla simple dentro de argumento ├║nico.
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

// OpenTraceViewer lanza `npx playwright show-trace <path>` v├¡a Runner normal.
// Guard: si ya hay un visor abierto se notifica y no se lanza otro (paridad
// is_running de Python). shutdown()/RestartApp tambi├®n lo detienen.
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
			runtime.EventsEmit(a.ctx, "trace:log", map[string]interface{}{
				"index": index, "line": line, "isError": false,
			})
		},
		OnStderr: func(line string) {
			runtime.EventsEmit(a.ctx, "trace:log", map[string]interface{}{
				"index": index, "line": line, "isError": true,
			})
		},
		OnError: func(desc string) {
			runtime.EventsEmit(a.ctx, "trace:log", map[string]interface{}{
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

// ---- Settings bindings ----

func (a *App) GetSettings() config.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SetSetting valida key/value, persiste y emite "settings:changed" con el
// valor normalizado. Inv├ílido ÔåÆ []string{msg} sin guardar.
func (a *App) SetSetting(key, value string) []string {
	s := a.GetSettings()
	normalized := ""
	switch key {
	case "theme":
		switch value {
		case "light", "dark", "oled":
			s.Theme = value
			normalized = value
		default:
			return []string{"Invalid theme value (expected light, dark or oled)"}
		}
	case "monitor_polling":
		b, err := parseStrictBool(value)
		if err != nil {
			return []string{"Invalid monitor_polling value (expected true or false)"}
		}
		s.MonitorPolling = b
		normalized = strconv.FormatBool(b)
	case "toasts_enabled":
		b, err := parseStrictBool(value)
		if err != nil {
			return []string{"Invalid toasts_enabled value (expected true or false)"}
		}
		s.ToastsEnabled = b
		normalized = strconv.FormatBool(b)
	default:
		return []string{"Unknown setting key: " + key}
	}

	if err := config.SaveSettings(a.settingsPath, s); err != nil {
		return []string{err.Error()}
	}
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, "settings:changed", map[string]string{
		"key": key, "value": normalized,
	})
	return nil
}

func parseStrictBool(v string) (bool, error) {
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", v)
	}
}

// ---- Quit ----

// Quit cierra la app real (confirm lo hace el frontend); shutdown() detiene
// servidores y managers v├¡a OnBeforeClose/shutdown.
func (a *App) Quit() {
	a.mu.Lock()
	a.forceExit = true
	a.mu.Unlock()
	runtime.Quit(a.ctx)
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
	runtime.Quit(a.ctx)
}
