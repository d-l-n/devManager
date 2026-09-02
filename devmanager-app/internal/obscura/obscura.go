// Package obscura orquesta el uso del navegador headless de
// https://github.com/h4ckf0r0day/obscura como herramienta auxiliar.
//
// NO reemplaza al runner de pruebas Playwright: es un complemento para
// capturas, extracción de contenido y evaluación JS sobre la URL del proyecto.
package obscura

import (
	"strconv"
	"sync"

	"github.com/d-l-n/devmanager/internal/process"
)

type State string

const (
	StateIdle    State = "idle"
	StateRunning State = "running"
	StateError   State = "error"
)

type Callbacks struct {
	OnStateChange func(state State)
	OnLog         func(line string, isError bool)
}

type Manager struct {
	mu      sync.Mutex
	cb      Callbacks
	state   State
	runner  *process.Runner
	stopped bool
}

func NewManager(cb Callbacks) *Manager {
	m := &Manager{
		state: StateIdle,
		cb:    cb,
	}
	m.runner = process.NewRunner(process.RunnerCallbacks{
		OnStdout:   func(line string) { m.log(line, false) },
		OnStderr:   func(line string) { m.log("[ERROR] "+line, true) },
		OnFinished: m.onRunnerFinished,
		OnError:    func(desc string) { m.log("[ERROR] "+desc, true) },
	})
	return m
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state
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

func (m *Manager) isRunning() bool {
	return m.State() == StateRunning
}

// ensureBinary instala el binario si falta, con progreso en el log.
// Falla si intentamos lanzar una acción sin binario disponible.
func (m *Manager) ensureBinary() error {
	if HasBinary() {
		return nil
	}
	m.log("Obscura binary not found at "+BinPath()+", downloading "+Version+" ...", false)
	prev := -1
	err := Install(func(p int) {
		if p != prev {
			prev = p
			m.log("Downloading Obscura: "+strconv.Itoa(p)+"%", false)
		}
	})
	if err != nil {
		m.setState(StateError)
		m.log("[ERROR] "+err.Error()+". Manual: download from "+releaseBaseURL, true)
		return err
	}
	m.log("Obscura "+Version+" installed at "+BinPath(), false)
	return nil
}

// Screenshot captura url en outPath. Debe existir el directorio destino.
func (m *Manager) Screenshot(url, outPath string) error {
	if url == "" {
		return m.fail("No URL provided for screenshot")
	}
	if outPath == "" {
		return m.fail("No output path provided for screenshot")
	}
	if err := m.ensureBinary(); err != nil {
		return err
	}
	return m.runArgv([]string{BinPath(), "fetch", url, "--allow-private-network", "-s", outPath})
}

// Dump extrae el contenido de url en formato kind (text|markdown|links).
func (m *Manager) Dump(url, kind string) error {
	if url == "" {
		return m.fail("No URL provided for dump")
	}
	if kind == "" {
		return m.fail("No dump format provided (text|markdown|links)")
	}
	if err := m.ensureBinary(); err != nil {
		return err
	}
	return m.runArgv([]string{BinPath(), "fetch", url, "--allow-private-network", "-d", kind})
}

// Eval ejecuta una expresión JS en url.
func (m *Manager) Eval(url, js string) error {
	if url == "" {
		return m.fail("No URL provided for eval")
	}
	if js == "" {
		return m.fail("Empty JS expression")
	}
	if err := m.ensureBinary(); err != nil {
		return err
	}
	// StartArgv evita el quoting del shell: el JS pasa tal cual.
	return m.runArgv([]string{BinPath(), "fetch", url, "--allow-private-network", "-e", js})
}

// Fetch ejecuta un comando fetch libre: el usuario escribe flags + URL.
// Aquí sí se usa el shell (paridad con "Run Custom"): el usuario controla
// el comando completo después del binario.
func (m *Manager) Fetch(customArgs string) error {
	if err := m.ensureBinary(); err != nil {
		return err
	}
	if customArgs == "" {
		return m.fail("Empty fetch command")
	}
	cmd := "\"" + BinPath() + "\" fetch " + customArgs
	m.log("Running command: "+cmd, false)
	if m.isRunning() {
		return m.fail("Obscura is already running")
	}
	m.setState(StateRunning)
	if err := m.runner.Start(cmd, BinDir(), nil); err != nil {
		return m.fail("Failed to start: " + err.Error())
	}
	return nil
}

func (m *Manager) runArgv(argv []string) error {
	if m.isRunning() {
		return m.fail("Obscura is already running")
	}
	m.setState(StateRunning)
	m.mu.Lock()
	m.stopped = false
	m.mu.Unlock()
	m.log("Running: obscura "+joinForLog(argv[1:]), false)
	if err := m.runner.StartArgv(argv, BinDir(), nil); err != nil {
		return m.fail("Failed to start: " + err.Error())
	}
	return nil
}

func joinForLog(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += "\"" + a + "\""
	}
	return out
}

func (m *Manager) fail(desc string) error {
	m.setState(StateError)
	m.log("[ERROR] "+desc, true)
	return &execError{desc}
}

type execError struct{ desc string }

func (e *execError) Error() string { return e.desc }

// Stop replica stop(): mata el proceso y vuelve a idle.
func (m *Manager) Stop() {
	m.mu.Lock()
	m.stopped = true
	m.mu.Unlock()
	if m.runner.IsRunning() {
		m.runner.Stop()
	}
	m.setState(StateIdle)
	m.log("Obscura stopped by user.", false)
}

func (m *Manager) onRunnerFinished(exitCode int, status string) {
	m.mu.Lock()
	stopped := m.stopped
	m.mu.Unlock()

	if stopped {
		m.mu.Lock()
		m.stopped = false
		m.mu.Unlock()
		m.setState(StateIdle)
		return
	}

	if exitCode == 0 {
		m.setState(StateIdle)
		m.log("Obscura finished (exit code 0).", false)
	} else {
		m.setState(StateError)
		m.log("Obscura process finished with exit code "+strconv.Itoa(exitCode)+" ("+status+")", true)
	}
}