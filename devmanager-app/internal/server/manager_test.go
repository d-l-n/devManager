package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

func newTestProject(port, timeoutMs int) models.Project {
	return models.Project{
		Name: "t", Path: ".",
		Server: models.ServerConfig{
			Enabled: true, Command: "echo hola",
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
	p.Server.Command = "cmd /c exit 3" // muere inmediatamente con código != 0
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
