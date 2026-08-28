// Package scripts porta app/scripts/manager.py: un proceso de script por
// proyecto, con prefijo [nombre] en el log y stop con intenci├│n expl├¡cita.
package scripts

import (
	"strconv"
	"sync"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/process"
)

type Callbacks struct {
	OnScriptStarted  func(name string)
	OnScriptFinished func(name string, exitCode int)
	OnLog            func(msg string, isError bool)
}

type Manager struct {
	mu sync.Mutex

	project models.Project
	cb      Callbacks

	runner        *process.Runner
	activeName    string
	activeCommand string
	stopRequested bool
}

func NewManager(project models.Project, cb Callbacks) *Manager {
	m := &Manager{project: project, cb: cb}
	m.runner = process.NewRunner(process.RunnerCallbacks{
		OnStarted:  m.onRunnerStarted,
		OnStdout:   func(line string) { m.emitPrefixed(line, false) },
		OnStderr:   func(line string) { m.emitPrefixed(line, true) },
		OnFinished: m.onRunnerFinished,
		OnError:    func(desc string) { m.emitError(desc) },
	})
	return m
}

func (m *Manager) UpdateProject(p models.Project) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.project = p
}

func (m *Manager) IsRunning() bool { return m.runner.IsRunning() }

// ActiveScriptName replica active_script_name: vac├¡o cuando no corre.
func (m *Manager) ActiveScriptName() string {
	if !m.IsRunning() {
		return ""
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeName
}

// RunScript replica run_script: un solo script activo por proyecto.
func (m *Manager) RunScript(name, command string) {
	if m.IsRunning() {
		m.mu.Lock()
		active := m.activeName
		m.mu.Unlock()
		m.log("ÔÜá´©Å Cannot start '"+name+"': script '"+active+"' is already running.", true)
		return
	}

	m.mu.Lock()
	m.activeName = name
	m.activeCommand = command
	m.stopRequested = false
	path := m.project.Path
	m.mu.Unlock()

	m.log("ÔÜí Starting script '"+name+"': "+command, false)
	_ = m.runner.Start(command, path, nil)
}

// Stop replica stop(): marca intenci├│n ANTES de matar (taskkill => CrashExit
// no cuenta como fallo del script).
func (m *Manager) Stop() {
	if !m.IsRunning() {
		return
	}
	m.mu.Lock()
	name := m.activeName
	if name == "" {
		name = "Script"
	}
	m.stopRequested = true
	m.mu.Unlock()

	m.log("­ƒøæ Stopping script '"+name+"'...", false)
	m.runner.Stop()
}

func (m *Manager) log(msg string, isError bool) {
	if m.cb.OnLog != nil {
		m.cb.OnLog(msg, isError)
	}
}

// emitPrefixed replica los lambdas output_ready/error_ready: "[name] msg".
func (m *Manager) emitPrefixed(msg string, isError bool) {
	name := m.activeNameForLogs()
	m.log("["+name+"] "+msg, isError)
}

// emitError replica el lambda process_error: "[name] ERROR: msg".
func (m *Manager) emitError(desc string) {
	name := m.activeNameForLogs()
	m.log("["+name+"] ERROR: "+desc, true)
}

func (m *Manager) activeNameForLogs() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeName
}

func (m *Manager) onRunnerStarted() {
	m.mu.Lock()
	name := m.activeName
	m.mu.Unlock()
	if name != "" && m.cb.OnScriptStarted != nil {
		m.cb.OnScriptStarted(name)
	}
}

// onRunnerFinished replica _on_finished.
//
// DIVERGENCIA INTENCIONAL vs Python (#8): en Python _on_finished no distingue
// stop; el runner moribundo (taskkill => exit no-0) pasa el c├│digo REAL a
// script_finished, lanzando notificaci├│n de fallo aunque el usuario cancelara.
// Aqu├¡, si hubo Stop(), reportamos script_finished(name, 0) ÔåÆ cierre limpio sin
// notificaci├│n de fallo cosm├®tica. Decisi├│n expl├¡cita de producto.
func (m *Manager) onRunnerFinished(exitCode int, statusDesc string) {
	m.mu.Lock()
	finishedName := m.activeName
	if finishedName == "" {
		finishedName = "Script"
	}
	wasStopped := m.stopRequested
	m.mu.Unlock()

	if wasStopped {
		m.mu.Lock()
		m.stopRequested = false
		m.activeName = ""
		m.activeCommand = ""
		m.mu.Unlock()
		m.log("­ƒøæ Script '"+finishedName+"' stopped by user.", false)
		if m.cb.OnScriptFinished != nil {
			m.cb.OnScriptFinished(finishedName, 0)
		}
		return
	}

	msg := ""
	isErr := exitCode != 0
	if exitCode == 0 {
		msg = "Ô£ô Script '" + finishedName + "' finished (code 0)"
	} else {
		msg = "Ô£ò Script '" + finishedName + "' exited with code " + strconv.Itoa(exitCode) + " (" + statusDesc + ")"
	}
	m.log(msg, isErr)

	if m.cb.OnScriptFinished != nil {
		m.cb.OnScriptFinished(finishedName, exitCode)
	}
	m.mu.Lock()
	m.activeName = ""
	m.activeCommand = ""
	m.mu.Unlock()
}
