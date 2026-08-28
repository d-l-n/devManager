// Package playwright porta app/playwright/manager.py.
//
// Orquesta la ejecuci├│n de pruebas Playwright con auto-start del servidor:
// si el servidor est├í habilitado y no RUNNING, guarda el comando pendiente,
// arranca el servidor y espera su se├▒al ready (listener) para ejecutar.
package playwright

import (
	"strconv"
	"sync"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/process"
	"github.com/d-l-n/devmanager/internal/server"
)

type State string

const (
	StateIdle     State = "idle"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StatePassed   State = "passed"
	StateFailed   State = "failed"
	StateError    State = "error"
)

type Callbacks struct {
	OnStateChange func(state State)
	OnLog         func(line string, isError bool)
	OnFinished    func(exitCode int) // paridad tests_finished
}

type Manager struct {
	mu sync.Mutex

	project models.Project
	server  *server.Manager
	state   State

	pendingCommand string
	stopRequested  bool

	runner       *process.Runner
	reportRunner *process.Runner
	cb           Callbacks
}

func NewManager(project models.Project, srv *server.Manager, cb Callbacks) *Manager {
	m := &Manager{
		project: project,
		server:  srv,
		state:   StateIdle,
		cb:      cb,
	}

	m.runner = process.NewRunner(process.RunnerCallbacks{
		OnStdout:   func(line string) { m.log(line, false) },
		OnStderr:   func(line string) { m.log("[ERROR] "+line, true) },
		OnFinished: m.onRunnerFinished,
		OnError:    func(desc string) { m.log("[ERROR] "+desc, true) },
	})

	// Paridad Python: el report runner NO conecta process_finished;
	// show-report vive hasta que el usuario lo cierra.
	m.reportRunner = process.NewRunner(process.RunnerCallbacks{
		OnStdout: func(line string) { m.log(line, false) },
		OnStderr: func(line string) { m.log("[ERROR] "+line, true) },
		OnError:  func(desc string) { m.log("[ERROR] "+desc, true) },
	})

	// Observaci├│n del servidor (equivalente a connect(state_changed)/connect(ready)).
	srv.AddStateListener(m.onServerStateChanged)
	srv.AddReadyListener(m.onServerReady)
	return m
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) UpdateProject(p models.Project) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.project = p
}

func (m *Manager) log(line string, isError bool) {
	if m.cb.OnLog != nil {
		m.cb.OnLog(line, isError)
	}
}

func (m *Manager) setState(s State) {
	m.mu.Lock()
	if m.state == s {
		m.mu.Unlock()
		return
	}
	m.state = s
	onChange := m.cb.OnStateChange
	m.mu.Unlock()
	if onChange != nil {
		onChange(s)
	}
}

func (m *Manager) RunTests() { m.executeOrWaitForServer(m.project.Playwright.Command) }
func (m *Manager) RunUI()    { m.executeOrWaitForServer(m.project.Playwright.UICommand) }
func (m *Manager) RunDebug() { m.executeOrWaitForServer(m.project.Playwright.DebugCommand) }

// ShowReport replica show_report.
func (m *Manager) ShowReport() {
	cmd := m.project.Playwright.ReportCommand
	if cmd == "" {
		m.log("No report command configured", false)
		return
	}
	m.log("Executing report command: "+cmd, false)
	if err := m.reportRunner.Start(cmd, m.project.Path, nil); err != nil {
		// Runner ya notific├│ por OnError; aqu├¡ no cambia estado (paridad).
		_ = err
	}
}

// Stop replica stop(): arma stopRequested SIEMPRE para que un finish tard├¡o
// del runner moribundo no cuente como resultado de tests.
func (m *Manager) Stop() {
	m.mu.Lock()
	m.stopRequested = true
	m.pendingCommand = ""
	m.mu.Unlock()
	if m.runner.IsRunning() {
		m.runner.Stop()
	}
	m.setState(StateIdle)
}

// executeOrWaitForServer replica _execute_or_wait_for_server.
func (m *Manager) executeOrWaitForServer(command string) {
	if command == "" {
		m.log("Command is empty", false)
		return
	}
	if m.State() == StateRunning {
		m.log("Playwright is already running", false)
		return
	}
	m.mu.Lock()
	project := m.project
	m.mu.Unlock()

	if !project.Playwright.Enabled {
		m.log("Playwright is not enabled", false)
		return
	}

	if project.Server.Enabled && m.server.State() != models.StateRunning {
		m.mu.Lock()
		m.pendingCommand = command
		m.mu.Unlock()
		m.setState(StateStarting)
		m.server.Start() // no-op si ya STARTING (guard interno)
		m.log("Waiting for server to be ready...", false)
		return
	}
	m.runCommand(command)
}

// onServerReady replica _on_server_ready.
func (m *Manager) onServerReady() {
	m.mu.Lock()
	pending := m.pendingCommand
	m.pendingCommand = ""
	m.mu.Unlock()
	if pending != "" {
		m.runCommand(pending)
	}
}

// onServerStateChanged replica _on_server_state_changed: cancela si el
// servidor cae a ERROR/STOPPED con comando pendiente.
func (m *Manager) onServerStateChanged(s models.ServerState) {
	m.mu.Lock()
	pending := m.pendingCommand
	m.mu.Unlock()
	if pending == "" {
		return
	}
	if s == models.StateError || s == models.StateStopped {
		m.mu.Lock()
		m.pendingCommand = ""
		m.mu.Unlock()
		m.setState(StateError)
		m.log("Server failed to start or timed out. Playwright execution cancelled.", false)
	}
}

// runCommand replica _run_command.
func (m *Manager) runCommand(command string) {
	m.mu.Lock()
	m.stopRequested = false
	project := m.project
	m.mu.Unlock()

	m.setState(StateRunning)
	m.log("Running command: "+command, false)

	extraEnv := map[string]string{}
	if project.Server.Enabled && m.server.ActiveURL() != "" {
		activeURL := m.server.ActiveURL()
		extraEnv["BASE_URL"] = activeURL
		extraEnv["PLAYWRIGHT_TEST_BASE_URL"] = activeURL
		if port := m.server.ActivePort(); port > 0 {
			extraEnv["PORT"] = strconv.Itoa(port)
		}
	}

	if err := m.runner.Start(command, project.Path, extraEnv); err != nil {
		// OnError ya registr├│ el fallo en el log (paridad process_error).
		_ = err
	}
}

// onRunnerFinished se basa en _on_runner_finished.
//
// DIVERGENCIA INTENCIONAL vs Python (#8): en Python, tras Stop() el runner
// moribundo (taskkill => exit no-0) marca StateFailed y emite tests_finished,
// lanzando notificaci├│n de tests fallidos aunque el usuario simplemente haya
// cancelado. Aqu├¡ mantenemos un cierre limpio: si el usuario par├│, terminamos
// en StateIdle y NO emitimos OnFinished/tests_finished. Decisi├│n expl├¡cita de
// producto para evitar la notificaci├│n de fallo cosm├®tico.
func (m *Manager) onRunnerFinished(exitCode int, status string) {
	m.mu.Lock()
	stopped := m.stopRequested
	m.mu.Unlock()

	if stopped {
		m.mu.Lock()
		m.stopRequested = false
		m.mu.Unlock()
		m.setState(StateIdle)
		m.log("Playwright run stopped by user (exit code "+strconv.Itoa(exitCode)+").", false)
		return
	}

	if exitCode == 0 {
		m.setState(StatePassed)
	} else {
		m.setState(StateFailed)
	}
	if m.cb.OnFinished != nil {
		m.cb.OnFinished(exitCode)
	}
	m.log("Playwright process finished with exit code "+strconv.Itoa(exitCode)+" ("+status+")", false)
}
