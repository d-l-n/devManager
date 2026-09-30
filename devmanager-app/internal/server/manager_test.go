package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/testutil"
)

func newTestProject(port, timeoutMs int) models.Project {
	return models.Project{
		Name: "t", Path: ".",
		Server: models.ServerConfig{
			Enabled: true, Command: testutil.PingCmdStr(),
			Port: port, URL: "http://localhost", StartupTimeout: timeoutMs,
		},
	}
}

type recorder struct {
	mu       sync.Mutex
	states   []models.ServerState
	logs     []string
	ready    int
	detects  []int
	mismatch []string
}

func (r *recorder) callbacks() Callbacks {
	return Callbacks{
		OnStateChange:  func(s models.ServerState) { r.mu.Lock(); r.states = append(r.states, s); r.mu.Unlock() },
		OnLog:          func(l string, e bool) { r.mu.Lock(); r.logs = append(r.logs, l); r.mu.Unlock() },
		OnReady:        func() { r.mu.Lock(); r.ready++; r.mu.Unlock() },
		OnPortDetected: func(p int, u string) { r.mu.Lock(); r.detects = append(r.detects, p); r.mu.Unlock() },
		OnPortMismatch: func(c, d int, u string) { r.mu.Lock(); r.mismatch = append(r.mismatch, u); r.mu.Unlock() },
	}
}

func (r *recorder) hasState(s models.ServerState) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, st := range r.states {
		if st == s {
			return true
		}
	}
	return false
}

func (r *recorder) readyCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.ready
}

func (r *recorder) mismatchCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.mismatch)
}

func (r *recorder) countLogs(substr string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, l := range r.logs {
		if strings.Contains(l, substr) {
			n++
		}
	}
	return n
}

func waitForState(t *testing.T, m *Manager, want models.ServerState, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.State() == want {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("estado %s no alcanzado en %v (actual %s)", want, timeout, m.State())
}

// proceso vivo: evita que OnFinished pase a Stopped antes de la detección.
func newAliveProject(port int) models.Project {
	p := newTestProject(port, 5000)
	p.Server.Command = testutil.PingCmdStr()
	return p
}

// waitBloqueado bloquea hasta que el contexto se cancele (simula un server
// que tarda más que la simulación y que la espera aborta por ctx.Done).
func waitBloqueado(ctx context.Context, timeout time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

// TestStartPortOccupiedTimesOutToError: si el puerto configurado ya estaba
// ocupado al lanzar no hay gate de puerto que sondear (el ocupante es ajeno),
// así que la única señal de arranque es la detección por log. Al vencer el
// timeout debe caer a ERROR igual que la espera de puerto, en vez de quedarse
// en STARTING para siempre (bug: Playwright esperaba un server que nunca
// llegaba a ready y no cancelaba nunca).
func TestStartPortOccupiedTimesOutToError(t *testing.T) {
	rec := &recorder{}
	proj := newTestProject(5173, 300)
	proj.Server.Command = testutil.PingCmdStr() // vivo, sin URL: nunca detecta puerto
	m := NewManager(proj, rec.callbacks())
	m.probePortFn = func(host string, port int) bool { return true } // ocupado pre-start
	m.waitPortFn = func(ctx context.Context, timeout time.Duration) error {
		t.Error("waitPortFn no debe usarse con el puerto ocupado (el puerto es ajeno)")
		return nil
	}

	m.Start()
	waitForState(t, m, models.StateError, 5*time.Second)

	if rec.readyCount() != 0 {
		t.Errorf("OnReady no debe dispararse con el puerto ocupado, got %d", rec.readyCount())
	}
	if !strings.Contains(m.FailureReason(), "Startup timeout") {
		t.Errorf("failureReason = %q, want Startup timeout", m.FailureReason())
	}
	if got := rec.countLogs("Server startup timeout"); got != 1 {
		t.Errorf("log de timeout debe salir una vez, got %d", got)
	}
	if got := rec.countLogs("[Port Notice]"); got != 1 {
		t.Errorf("aviso de puerto ocupado debe salir una vez, got %d", got)
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

// TestStartPortOccupiedReadyByLog: con el puerto ocupado el arranque depende de
// la detección por log, así que un server que sí imprime su URL debe pasar a
// RUNNING (OnReady una vez, sin failureReason) y no esperar el timeout.
func TestStartPortOccupiedReadyByLog(t *testing.T) {
	rec := &recorder{}
	port := testutil.FreePort(t)
	m := NewManager(newTestProject(port, 5000), rec.callbacks())
	m.probePortFn = func(host string, p int) bool { return true } // ocupado pre-start
	m.waitPortFn = func(ctx context.Context, timeout time.Duration) error {
		t.Error("waitPortFn no debe usarse con el puerto ocupado (el puerto es ajeno)")
		return nil
	}

	m.Start()
	start := time.Now()
	m.onRunnerOutput(fmt.Sprintf("Vite dev server running at http://localhost:%d", port), false)
	waitForState(t, m, models.StateRunning, 5*time.Second)

	// El gate por log debe resolverse por detección, no por vencimiento.
	if elapsed := time.Since(start); elapsed >= 2*time.Second {
		t.Errorf("ready tardó %v: el gate no debe esperar el timeout", elapsed)
	}

	// Y la goroutine del gate debe salir sin marcar ERROR después del ready.
	time.Sleep(300 * time.Millisecond)
	if rec.readyCount() != 1 {
		t.Errorf("OnReady debe dispararse una vez, got %d", rec.readyCount())
	}
	if m.FailureReason() != "" {
		t.Errorf("failureReason = %q, want vacío", m.FailureReason())
	}

	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

// TestLogDetectionCancelsPendingWait: la detección de puerto por log debe
// cancelar la espera pendiente para que OnReady no se dispare dos veces
// (bug #5).
func TestLogDetectionCancelsPendingWait(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newAliveProject(5173), rec.callbacks())
	m.probePortFn = func(host string, port int) bool { return false }
	m.waitPortFn = waitBloqueado
	m.Start()

	m.onRunnerOutput("Vite dev server running at http://localhost:5173", false)
	waitForState(t, m, models.StateRunning, 5*time.Second)
	// Da tiempo a la goroutine de espera para abortar por ctx.Done.
	time.Sleep(150 * time.Millisecond)
	if rec.readyCount() != 1 {
		t.Errorf("OnReady debe dispararse exactamente una vez, got %d", rec.readyCount())
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

// TestStderrDoesNotTriggerPortDetection: stderr nunca debe participar en la
// detección de puerto ni en transiciones de estado (bug #6); stdout sí.
func TestStderrDoesNotTriggerPortDetection(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newAliveProject(5173), rec.callbacks())
	m.probePortFn = func(host string, port int) bool { return false }
	m.waitPortFn = waitBloqueado
	m.Start()

	// Línea de stderr con puerto: solo log, sin OnReady ni RUNNING.
	m.onRunnerError("[ERROR] App listening at http://localhost:5173")
	if m.State() != models.StateStarting {
		t.Fatalf("stderr no debe cambiar de estado, state=%s", m.State())
	}
	if rec.readyCount() != 0 {
		t.Errorf("stderr no debe disparar OnReady, got %d", rec.readyCount())
	}

	// La misma línea por stdout sí detecta el puerto.
	m.onRunnerOutput("App listening at http://localhost:5173", false)
	waitForState(t, m, models.StateRunning, 5*time.Second)
	if rec.readyCount() != 1 {
		t.Errorf("stdout debe disparar OnReady una vez, got %d", rec.readyCount())
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

// TestPortMismatchNotifiesOnce: el aviso de puerto divergente debe emitirse
// una sola vez, no en cada línea idéntica (bug #7).
func TestPortMismatchNotifiesOnce(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newAliveProject(5173), rec.callbacks())
	m.probePortFn = func(host string, port int) bool { return false }
	m.waitPortFn = waitBloqueado
	m.Start()

	m.onRunnerOutput("App listening at http://localhost:4173", false)
	m.onRunnerOutput("App listening at http://localhost:4173", false)

	if got := rec.mismatchCount(); got != 1 {
		t.Errorf("OnPortMismatch debe emitirse una vez, got %d", got)
	}
	if got := rec.countLogs("[PORT MISMATCH]"); got != 1 {
		t.Errorf("log [PORT MISMATCH] debe salir una vez, got %d", got)
	}
	if got := rec.countLogs("Active URL redirected to"); got != 1 {
		t.Errorf("log de redirect debe salir una vez, got %d", got)
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

func TestStartImmediateRunningWhenNoPort(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newTestProject(0, 1000), rec.callbacks())
	m.Start()
	waitForState(t, m, models.StateRunning, 5*time.Second)
	if rec.ready < 1 {
		t.Error("OnReady debe dispararse")
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

func TestStartDisabledDoesNotRun(t *testing.T) {
	p := newTestProject(5173, 1000)
	p.Server.Enabled = false
	m := NewManager(p, (&recorder{}).callbacks())
	m.Start()
	time.Sleep(200 * time.Millisecond)
	if m.State() != models.StateStopped {
		t.Errorf("disabled no debe arrancar, state=%s", m.State())
	}
}

func TestStopRequestedSuppressesCrashError(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newTestProject(0, 1000), rec.callbacks())
	m.Start()
	waitForState(t, m, models.StateRunning, 5*time.Second)
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
	time.Sleep(300 * time.Millisecond)
	if m.State() == models.StateError {
		t.Error("taskkill tras Stop no debe reportar ERROR")
	}
	if m.FailureReason() != "" {
		t.Errorf("failure_reason vacío tras stop, got %q", m.FailureReason())
	}
}

func TestCrashEntersError(t *testing.T) {
	p := newTestProject(0, 1000)
	p.Server.Command = testutil.ExitCmdStr(3)
	rec := &recorder{}
	m := NewManager(p, rec.callbacks())
	m.Start()
	waitForState(t, m, models.StateError, 5*time.Second)
	if m.FailureReason() == "" {
		t.Error("crash sin stop debe fijar failure_reason")
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

func TestPortReadyFlow(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newTestProject(5173, 5000), rec.callbacks())
	m.probePortFn = func(host string, port int) bool { return false }                    // puerto libre pre-start
	m.waitPortFn = func(ctx context.Context, timeout time.Duration) error { return nil } // listo al instante
	m.Start()
	waitForState(t, m, models.StateRunning, 5*time.Second)
	if _, ok := m.StartedAt(); !ok {
		t.Error("started_at debe fijarse al entrar RUNNING")
	}
	m.Stop()
}

func TestPortTimeoutEntersError(t *testing.T) {
	m := NewManager(newTestProject(5173, 250), (&recorder{}).callbacks())
	m.probePortFn = func(host string, port int) bool { return false }
	m.waitPortFn = func(ctx context.Context, timeout time.Duration) error {
		return context.DeadlineExceeded
	}
	m.Start()
	waitForState(t, m, models.StateError, 5*time.Second)
	if m.FailureReason() == "" {
		t.Error("failure_reason debe describir el timeout")
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

func TestRestartFromRunning(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newTestProject(0, 1000), rec.callbacks())
	m.Start()
	waitForState(t, m, models.StateRunning, 5*time.Second)
	m.Restart()
	waitForState(t, m, models.StateRunning, 8*time.Second)
	if !rec.hasState(models.StateStopping) && !rec.hasState(models.StateStopped) {
		t.Error("restart debe transitar estados de parada primero")
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}

func TestUptimeResetsOnStop(t *testing.T) {
	m := NewManager(newTestProject(0, 1000), (&recorder{}).callbacks())
	m.Start()
	waitForState(t, m, models.StateRunning, 5*time.Second)
	if _, ok := m.StartedAt(); !ok {
		t.Fatal("uptime activo esperado")
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
	if _, ok := m.StartedAt(); ok {
		t.Error("uptime debe resetear al parar")
	}
}

func TestBuildExtraEnvVarsWin(t *testing.T) {
	extra, warns := buildExtraEnv(5173, map[string]string{"FOO": "bar", "API_URL": "https://x"})
	if extra["FOO"] != "bar" || extra["API_URL"] != "https://x" {
		t.Errorf("vars deben inyectarse: %v", extra)
	}
	if extra["PORT"] != "5173" || extra["VITE_PORT"] != "5173" || extra["SERVER_PORT"] != "5173" {
		t.Errorf("puertos configurados: %v", extra)
	}
	if len(warns) != 0 {
		t.Errorf("sin warns esperados: %v", warns)
	}
}

func TestBuildExtraEnvPortKeysNeverOverride(t *testing.T) {
	extra, warns := buildExtraEnv(5173, map[string]string{"PORT": "9999", "vite_port": "9999", "FOO": "x"})
	if extra["PORT"] != "5173" || extra["VITE_PORT"] != "5173" || extra["SERVER_PORT"] != "5173" {
		t.Errorf("keys de puerto no deben pisar: %v", extra)
	}
	if len(warns) != 2 {
		t.Errorf("esperaba 2 warns (PORT + vite_port), got %v", warns)
	}
	for _, w := range warns {
		if strings.Contains(w, "9999") {
			t.Errorf("warn no debe incluir valores: %q", w)
		}
	}
	// Mismo puerto que el configurado → sin warn.
	if _, warns := buildExtraEnv(5173, map[string]string{"PORT": "5173"}); len(warns) != 0 {
		t.Errorf("puerto coincidente no debe advertir: %v", warns)
	}
}

func TestBuildExtraEnvEmpty(t *testing.T) {
	extra, warns := buildExtraEnv(0, nil)
	if len(extra) != 0 || len(warns) != 0 {
		t.Errorf("sin puerto ni vars → vacío: %v %v", extra, warns)
	}
}
