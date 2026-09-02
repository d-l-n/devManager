package playwright

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/server"
	"github.com/d-l-n/devmanager/internal/testutil"
)

type rec struct {
	mu       sync.Mutex
	logs     []string
	logErr   []bool
	states   []State
	finished []int
}

func (r *rec) callbacks() Callbacks {
	return Callbacks{
		OnStateChange: func(s State) { r.mu.Lock(); r.states = append(r.states, s); r.mu.Unlock() },
		OnLog:         func(l string, e bool) { r.mu.Lock(); r.logs = append(r.logs, l); r.logErr = append(r.logErr, e); r.mu.Unlock() },
		OnFinished:    func(c int) { r.mu.Lock(); r.finished = append(r.finished, c); r.mu.Unlock() },
	}
}

func (r *rec) hasLog(sub string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.logs {
		if contains(l, sub) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || index(s, sub) >= 0)
}

func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// newPair crea playwright.Manager + server.Manager acoplados con proceso
// de servidor REAL. DESVIACIÓN vs plan: probePortFn/waitPortFn son campos
// unexported de server.Manager y no son inyectables desde este paquete;
// la puerta de ready se controla con el comando real del servidor.
func newPair(t *testing.T, proj models.Project) (*Manager, *server.Manager, *rec) {
	t.Helper()
	r := &rec{}
	sm := server.NewManager(proj, server.Callbacks{})
	pm := NewManager(proj, sm, r.callbacks())
	return pm, sm, r
}

// waitForServerState espera el estado del server.Manager (tipo distinto de
// playwright.State; el helper del plan no compila cross-package).
func waitForServerState(t *testing.T, sm *server.Manager, want models.ServerState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if sm.State() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("estado servidor %s no alcanzado en %v (actual %s)", want, timeout, sm.State())
}

func baseProject(serverEnabled bool, pwEnabled bool, pwCommand string) models.Project {
	return models.Project{
		Name: "t", Path: ".",
		Server: models.ServerConfig{
			Enabled: serverEnabled, Command: "echo srv",
			Port: 5173, URL: "http://localhost:5173", StartupTimeout: 5000,
		},
		Playwright: models.PlaywrightConfig{
			Enabled: pwEnabled, Command: pwCommand,
			UICommand: pwCommand, DebugCommand: pwCommand,
			ReportCommand: "",
		},
	}
}

func waitForState(t *testing.T, m *Manager, want State, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.State() == want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("estado %s no alcanzado en %v (actual %s)", want, timeout, m.State())
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("condición no alcanzada en timeout")
}

func TestEmptyCommandLogsAndStaysIdle(t *testing.T) {
	pm, sm, r := newPair(t, baseProject(false, true, ""))
	pm.RunTests()
	if pm.State() != StateIdle {
		t.Errorf("estado = %s, want idle", pm.State())
	}
	if !r.hasLog("Command is empty") {
		t.Errorf("log esperado no encontrado: %v", r.logs)
	}
	sm.Stop()
}

func TestDisabledPlaywrightRejected(t *testing.T) {
	pm, sm, r := newPair(t, baseProject(false, false, testutil.EchoCmdStr("hi")))
	pm.RunTests()
	if pm.State() != StateIdle {
		t.Errorf("estado = %s, want idle", pm.State())
	}
	if !r.hasLog("Playwright is not enabled") {
		t.Errorf("log esperado no encontrado: %v", r.logs)
	}
	sm.Stop()
}

func TestDirectRunServerDisabled(t *testing.T) {
	pm, sm, _ := newPair(t, baseProject(false, true, testutil.ExitCmdStr(0)))
	pm.RunTests()
	waitForState(t, pm, StatePassed, 10*time.Second)
	sm.Stop()
}

func TestFinishNonZeroIsFailed(t *testing.T) {
	pm, sm, r := newPair(t, baseProject(false, true, testutil.ExitCmdStr(3)))
	pm.RunTests()
	waitForState(t, pm, StateFailed, 10*time.Second)
	waitFor(t, 2*time.Second, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return len(r.finished) == 1 && r.finished[0] == 3
	})
	if !r.hasLog("Playwright process finished with exit code 3 (CrashExit)") {
		t.Errorf("log final ausente: %v", r.logs)
	}
	sm.Stop()
}

func TestAutoStartWaitsForServerThenRuns(t *testing.T) {
	// Gate real: el servidor tarda ~2s en "abrir puerto" (ping + echo de URL
	// detectable por ExtractPortFromLog), dejando STARTING observable.
	proj := models.Project{
		Name: "t", Path: ".",
		Server: models.ServerConfig{
			Enabled: true,
			Command: testutil.SlowEchoCmdStr(2, "Local: http://localhost:5173/"),
			Port:    5173, URL: "http://localhost:5173", StartupTimeout: 15000,
		},
		Playwright: models.PlaywrightConfig{
			Enabled: true, Command: testutil.EchoCmdStr("pw-done"),
			UICommand: testutil.EchoCmdStr("pw-done"), DebugCommand: testutil.EchoCmdStr("pw-done"),
		},
	}
	pm, sm, r := newPair(t, proj)

	pm.RunTests()
	waitForState(t, pm, StateStarting, 5*time.Second)
	if !r.hasLog("Waiting for server to be ready...") {
		t.Errorf("log de espera ausente: %v", r.logs)
	}
	waitForState(t, pm, StatePassed, 20*time.Second)
	sm.Stop()
	waitForServerState(t, sm, models.StateStopped, 5*time.Second)
}

func TestCancelledWhenServerFailsToStart(t *testing.T) {
	// Servidor real que nunca abre el puerto; StartupTimeout corto fuerza
	// la transición a ERROR (paridad del gate inyectado del plan).
	proj := baseProject(true, true, testutil.EchoCmdStr("never"))
	proj.Server.Command = testutil.PingCmdStr()
	proj.Server.StartupTimeout = 800
	pm, sm, r := newPair(t, proj)

	pm.RunTests()
	waitForState(t, pm, StateError, 10*time.Second)
	if !r.hasLog("Server failed to start or timed out. Playwright execution cancelled.") {
		t.Errorf("log de cancelación ausente: %v", r.logs)
	}
	sm.Stop()
}

func TestStopWhileRunningGoesIdle(t *testing.T) {
	pm, sm, r := newPair(t, baseProject(false, true, testutil.PingCmdStr()))
	pm.RunTests()
	waitForState(t, pm, StateRunning, 10*time.Second)
	pm.Stop()
	waitForState(t, pm, StateIdle, 8*time.Second)
	if !r.hasLog("stopped by user") {
		t.Errorf("log stopped-by-user ausente: %v", r.logs)
	}
	sm.Stop()
}

func TestAlreadyRunningRejected(t *testing.T) {
	pm, sm, r := newPair(t, baseProject(false, true, testutil.PingCmdStr()))
	pm.RunTests()
	waitForState(t, pm, StateRunning, 10*time.Second)
	before := len(r.logs)
	pm.RunUI() // segundo intento mientras corre
	time.Sleep(200 * time.Millisecond)
	if !r.hasLog("Playwright is already running") {
		t.Errorf("log already-running ausente: %v", r.logs)
	}
	pm.Stop()
	waitForState(t, pm, StateIdle, 8*time.Second)
	_ = before
	sm.Stop()
}

func TestShowReportEmptyCommand(t *testing.T) {
	pm, sm, r := newPair(t, baseProject(false, true, ""))
	pm.ShowReport()
	if !r.hasLog("No report command configured") {
		t.Errorf("log esperado no encontrado: %v", r.logs)
	}
	sm.Stop()
}

func TestUpdateProjectChangesCommands(t *testing.T) {
	pm, sm, _ := newPair(t, baseProject(false, true, testutil.EchoCmdStr("v1")))
	updated := baseProject(false, true, testutil.EchoCmdStr(fmt.Sprintf("v%d", 2)))
	pm.UpdateProject(updated)
	// Sin aserción directa: UpdateProject no debe panear ni cambiar estado.
	if pm.State() != StateIdle {
		t.Errorf("tras update estado = %s, want idle", pm.State())
	}
	sm.Stop()
}
