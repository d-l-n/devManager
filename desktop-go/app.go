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

	"desktop-go/internal/config"
	"desktop-go/internal/models"
	"desktop-go/internal/playwright"
	"desktop-go/internal/process"
	"desktop-go/internal/scripts"
	"desktop-go/internal/server"
	"desktop-go/internal/sysmon"
	"desktop-go/internal/utils/detection"
	"desktop-go/internal/utils/evidence"
	"desktop-go/internal/utils/git"
)

type App struct {
	ctx context.Context
	mu  sync.Mutex

	cfg     *config.Manager
	servers map[int]*server.Manager

	playwrightManagers map[int]*playwright.Manager
	scriptManagers     map[int]*scripts.Manager

	traceRunner *process.Runner

	gitBusy bool // un comando git global a la vez (paridad QProcess único del panel)

	configPath string

	settingsPath string
	settings     config.Settings
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

// emitNotify emite el evento "notify" consumido por el widget de toasts.
func (a *App) emitNotify(title, message, level string) {
	runtime.EventsEmit(a.ctx, "notify", map[string]string{
		"title": title, "message": message, "level": level,
	})
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
	projectName := projects[index].Name

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
		if code != 0 {
			a.emitNotify(projectName, fmt.Sprintf("Git %s failed (exit %d)", action, code), "error")
		}

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
// proyectos actuales, quedándose solo con los RUNNING. Orden estable por índice.
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

	// Mapa puerto activo → nombre de proyecto (solo servers RUNNING propios).
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
// valor normalizado. Inválido → []string{msg} sin guardar.
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
// servidores y managers vía OnBeforeClose/shutdown.
func (a *App) Quit() {
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
