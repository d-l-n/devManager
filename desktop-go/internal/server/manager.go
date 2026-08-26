// Package server porta app/server/manager.py.
// El contexto stopCtx se cancela en Stop(): interrumpe la espera de puerto.
package server

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"desktop-go/internal/models"
	"desktop-go/internal/process"
	"desktop-go/internal/utils/detection"
	"desktop-go/internal/utils/ports"
)

type Callbacks struct {
	OnStateChange  func(state models.ServerState)
	OnLog          func(line string, isError bool)
	OnReady        func()
	OnPortDetected func(port int, url string)
	OnPortMismatch func(configured, detected int, activeURL string)
}

type Manager struct {
	mu sync.Mutex

	project         models.Project
	state           models.ServerState
	startedAt       time.Time
	activePort      int
	activeURL       string
	isMismatch      bool
	portWasOccupied bool
	stopRequested   bool
	failureReason   string

	runner         *process.Runner
	cb             Callbacks

	stateListeners []func(models.ServerState)
	readyListeners []func()

	stopCtx      context.Context
	cancelWait   context.CancelFunc
	restartTimer *time.Timer

	// Inyección para tests. Defaults: ports.IsPortOpen / ports.WaitForPort.
	probePortFn func(host string, port int) bool
	waitPortFn  func(ctx context.Context, timeout time.Duration) error
}

// hostFromURL replica la extracción de host del Python:
// 'http://localhost:5173' → 'localhost'.
func hostFromURL(rawURL string) string {
	host := rawURL
	if host == "" {
		return "127.0.0.1"
	}
	if i := strings.Index(host, "://"); i >= 0 {
		host = host[i+3:]
	}
	if i := strings.IndexAny(host, "/:"); i >= 0 {
		host = host[:i]
	}
	if host == "" {
		return "127.0.0.1"
	}
	return host
}

func NewManager(project models.Project, cb Callbacks) *Manager {
	return &Manager{
		project:     project,
		state:       models.StateStopped,
		activePort:  project.Server.Port,
		activeURL:   project.Server.URL,
		cb:          cb,
		probePortFn: ports.IsPortOpen,
	}
}

func (m *Manager) State() models.ServerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
}

func (m *Manager) ActivePort() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activePort
}

func (m *Manager) ActiveURL() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeURL
}

func (m *Manager) FailureReason() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.failureReason
}

// AddStateListener registra un oyente adicional de cambios de estado
// (usado por playwright.Manager para observar al servidor).
func (m *Manager) AddStateListener(fn func(models.ServerState)) {
	m.mu.Lock()
	m.stateListeners = append(m.stateListeners, fn)
	m.mu.Unlock()
}

// AddReadyListener registra un oyente adicional de servidor listo.
func (m *Manager) AddReadyListener(fn func()) {
	m.mu.Lock()
	m.readyListeners = append(m.readyListeners, fn)
	m.mu.Unlock()
}

// StartedAt replica started_at: timestamp al entrar RUNNING; false si parado.
func (m *Manager) StartedAt() (time.Time, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.startedAt, !m.startedAt.IsZero()
}

func (m *Manager) UpdateProject(p models.Project) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.project = p
	m.activePort = p.Server.Port
	m.activeURL = p.Server.URL
	m.isMismatch = false
}

func (m *Manager) log(line string, isError bool) {
	if m.cb.OnLog != nil {
		m.cb.OnLog(line, isError)
	}
}

// setState requiere m.mu NO tomado.
func (m *Manager) setState(s models.ServerState) {
	m.mu.Lock()
	if m.state == s {
		m.mu.Unlock()
		return
	}
	if s == models.StateRunning && m.startedAt.IsZero() {
		m.startedAt = time.Now()
	}
	m.state = s
	onChange := m.cb.OnStateChange
	listeners := make([]func(models.ServerState), len(m.stateListeners))
	copy(listeners, m.stateListeners)
	m.mu.Unlock()
	if onChange != nil {
		onChange(s)
	}
	for _, fn := range listeners {
		fn(s)
	}
}

// fireReady dispara OnReady y los readyListeners registrados (fuera del lock).
func (m *Manager) fireReady() {
	m.mu.Lock()
	onReady := m.cb.OnReady
	listeners := make([]func(), len(m.readyListeners))
	copy(listeners, m.readyListeners)
	m.mu.Unlock()
	if onReady != nil {
		onReady()
	}
	for _, fn := range listeners {
		fn()
	}
}

func (m *Manager) enterRunning() {
	m.mu.Lock()
	if m.startedAt.IsZero() {
		m.startedAt = time.Now()
	}
	m.mu.Unlock()
	m.setState(models.StateRunning)
}

// Start replica ServerManager.start().
func (m *Manager) Start() {
	m.mu.Lock()
	if m.state != models.StateStopped && m.state != models.StateError {
		m.mu.Unlock()
		return
	}
	if !m.project.Server.Enabled {
		m.mu.Unlock()
		m.log("Server not enabled", true)
		return
	}

	m.failureReason = ""
	m.activePort = m.project.Server.Port
	m.activeURL = m.project.Server.URL
	m.isMismatch = false

	configuredPort := m.project.Server.Port
	host := hostFromURL(m.project.Server.URL)
	command := m.project.Server.Command
	cwd := m.project.Path
	startupTimeoutMs := m.project.Server.StartupTimeout
	m.mu.Unlock()

	startupTimeout := time.Duration(startupTimeoutMs) * time.Millisecond
	if startupTimeout <= 0 {
		startupTimeout = 30 * time.Second
	}

	portWasOccupied := configuredPort > 0 && m.probePortFn(host, configuredPort)

	m.mu.Lock()
	m.portWasOccupied = portWasOccupied
	m.mu.Unlock()

	if portWasOccupied {
		m.log(fmt.Sprintf(
			"⚠️ [Port Notice] Configured port %d is already in use by another process. "+
				"The server may bind to a dynamic fallback port.", configuredPort), true)
	}

	cmd := command
	if configuredPort > 0 {
		cmd = ports.BuildServerCommand(command, configuredPort)
	}
	m.log(fmt.Sprintf("Starting server: %s in %s", cmd, cwd), false)

	extraEnv := map[string]string{}
	if configuredPort > 0 {
		extraEnv["PORT"] = strconv.Itoa(configuredPort)
		extraEnv["VITE_PORT"] = strconv.Itoa(configuredPort)
		extraEnv["SERVER_PORT"] = strconv.Itoa(configuredPort)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m.mu.Lock()
	m.stopCtx = ctx
	m.cancelWait = cancel
	waitFn := m.waitPortFn
	m.mu.Unlock()

	runner := process.NewRunner(process.RunnerCallbacks{
		OnStdout:  func(line string) { m.onRunnerOutput(line, false) },
		OnStderr:  func(line string) { m.onRunnerOutput("[ERROR] "+line, true) },
		OnStarted: func() { m.onRunnerStarted(ctx, startupTimeout, waitFn) },
		OnFinished: func(exitCode int, status string) {
			cancel()
			m.onRunnerFinished(exitCode, status)
		},
		OnError: func(desc string) { m.log(desc, true) },
	})

	m.mu.Lock()
	m.runner = runner
	m.mu.Unlock()

	m.setState(models.StateStarting)
	if err := runner.Start(cmd, cwd, extraEnv); err != nil {
		cancel()
		m.mu.Lock()
		m.failureReason = "Failed to start: " + err.Error()
		m.mu.Unlock()
		m.setState(models.StateError)
	}
}

// Stop replica ServerManager.stop(): marca intención ANTES de matar para que
// taskkill /F no se reporte como crash.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.state == models.StateStopped {
		m.mu.Unlock()
		return
	}
	m.stopRequested = true
	m.failureReason = ""
	if m.restartTimer != nil {
		m.restartTimer.Stop()
		m.restartTimer = nil
	}
	cancel := m.cancelWait
	runner := m.runner
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.setState(models.StateStopping)
	if runner != nil {
		runner.Stop()
	}
	m.mu.Lock()
	m.startedAt = time.Time{}
	m.mu.Unlock()
	m.setState(models.StateStopped)
}

// Restart replica restart(): stop + arranque diferido 500ms si estaba vivo.
func (m *Manager) Restart() {
	m.mu.Lock()
	needsDelay := m.state == models.StateRunning ||
		m.state == models.StateStarting || m.state == models.StateError
	m.mu.Unlock()

	if needsDelay {
		m.Stop()
		m.mu.Lock()
		m.restartTimer = time.AfterFunc(500*time.Millisecond, m.Start)
		m.mu.Unlock()
	} else {
		m.Start()
	}
}

// onRunnerStarted replica _on_runner_started.
func (m *Manager) onRunnerStarted(ctx context.Context, startupTimeout time.Duration, waitFn func(context.Context, time.Duration) error) {
	m.mu.Lock()
	m.stopRequested = false // launch real: un exit anómalo vuelve a ser significativo
	port := m.project.Server.Port
	occupied := m.portWasOccupied
	url := m.project.Server.URL
	configuredTimeoutMs := m.project.Server.StartupTimeout
	m.mu.Unlock()

	if port > 0 && !occupied {
		host := hostFromURL(url)
		go func() {
			var err error
			if waitFn != nil {
				err = waitFn(ctx, startupTimeout)
			} else {
				err = ports.WaitForPort(ctx, host, port, startupTimeout, 250*time.Millisecond)
			}
			select {
			case <-ctx.Done():
				return // stop durante la espera: no tocar estado
			default:
			}
			if err == nil {
				m.onPortReady()
				return
			}
			if m.State() == models.StateStarting {
				m.mu.Lock()
				m.failureReason = fmt.Sprintf("Startup timeout after %dms", configuredTimeoutMs)
				m.mu.Unlock()
				m.setState(models.StateError)
				m.log(fmt.Sprintf("Server startup timeout after %dms", configuredTimeoutMs), true)
			}
		}()
	} else if port <= 0 {
		m.enterRunning()
		m.fireReady()
	}
}

func (m *Manager) onPortReady() {
	m.enterRunning()
	m.fireReady()
	port := m.ActivePort()
	m.log(fmt.Sprintf("Server is ready on port %d", port), false)
}

// onRunnerOutput replica _on_runner_output: log en vivo + detección dinámica
// de puerto + transición STARTING→RUNNING por log.
func (m *Manager) onRunnerOutput(line string, isError bool) {
	m.log(line, isError)

	m.mu.Lock()
	state := m.state
	m.mu.Unlock()
	if state != models.StateStarting && state != models.StateRunning {
		return
	}

	detected := detection.ExtractPortFromLog(line)
	if detected == 0 {
		return
	}

	var emitDetected bool
	var detectedURL string

	m.mu.Lock()
	configuredPort := m.project.Server.Port
	hasChanged := m.activePort != detected
	isDifferentFromConfig := configuredPort > 0 && detected != configuredPort

	m.activePort = detected
	m.activeURL = fmt.Sprintf("http://localhost:%d", detected)
	detectedURL = m.activeURL

	if isDifferentFromConfig && !m.isMismatch {
		m.isMismatch = true
		emitDetected = true
	} else if hasChanged || detected == configuredPort {
		emitDetected = true
	}
	mismatchNow := m.isMismatch && isDifferentFromConfig
	m.mu.Unlock()

	if mismatchNow {
		if m.cb.OnPortMismatch != nil {
			m.cb.OnPortMismatch(configuredPort, detected, detectedURL)
		}
		m.log(fmt.Sprintf("⚠️ [PORT MISMATCH] Server started on port %d, differing from configured port %d", detected, configuredPort), true)
		m.log(fmt.Sprintf("🔗 Active URL redirected to: %s", detectedURL), false)
	} else if hasChanged {
		m.log(fmt.Sprintf("[Auto-Detect] Server bound to active port %d", detected), false)
	}

	if emitDetected && m.cb.OnPortDetected != nil {
		m.cb.OnPortDetected(detected, detectedURL)
	}

	if m.State() == models.StateStarting {
		m.enterRunning()
		m.fireReady()
	}
}

// onRunnerFinished replica _on_runner_finished.
func (m *Manager) onRunnerFinished(exitCode int, status string) {
	m.mu.Lock()
	prevState := m.state
	stopReq := m.stopRequested
	m.startedAt = time.Time{}
	m.mu.Unlock()

	if stopReq || (status == "NormalExit" && exitCode == 0) {
		m.mu.Lock()
		m.stopRequested = false
		m.mu.Unlock()
		m.setState(models.StateStopped)
	} else {
		reason := "Server failed to start"
		if prevState == models.StateRunning {
			reason = "Server process crashed"
		}
		m.mu.Lock()
		m.failureReason = reason
		m.mu.Unlock()
		m.setState(models.StateError)
	}

	m.log(fmt.Sprintf("Server process finished with exit code %d (%s)", exitCode, status), status != "NormalExit")
}
