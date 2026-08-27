package scripts

import (
	"sync"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

type rec struct {
	mu       sync.Mutex
	logs     []string
	errs     []bool
	started  []string
	finished []struct {
		name string
		code int
	}
}

func (r *rec) callbacks() Callbacks {
	return Callbacks{
		OnScriptStarted: func(n string) { r.mu.Lock(); r.started = append(r.started, n); r.mu.Unlock() },
		OnScriptFinished: func(n string, c int) {
			r.mu.Lock()
			r.finished = append(r.finished, struct {
				name string
				code int
			}{n, c})
			r.mu.Unlock()
		},
		OnLog: func(msg string, isErr bool) {
			r.mu.Lock()
			r.logs = append(r.logs, msg)
			r.errs = append(r.errs, isErr)
			r.mu.Unlock()
		},
	}
}

func (r *rec) hasLog(sub string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, l := range r.logs {
		if len(l) >= len(sub) && containsAt(l, sub) {
			return true
		}
	}
	return false
}

func containsAt(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func newMgr(t *testing.T) (*Manager, *rec) {
	t.Helper()
	r := &rec{}
	p, _ := models.ParseProject([]byte(`{"name":"t","path":"."}`))
	return NewManager(p, r.callbacks()), r
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

func TestRunScriptSuccess(t *testing.T) {
	m, r := newMgr(t)
	m.RunScript("build", "cmd /c exit 0")
	waitFor(t, 8*time.Second, func() bool { return !m.IsRunning() })
	waitFor(t, 2*time.Second, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return len(r.finished) == 1 && r.finished[0].name == "build" && r.finished[0].code == 0
	})
	if !r.hasLog(`⚡ Starting script 'build': cmd /c exit 0`) {
		t.Errorf("log de arranque ausente: %v", r.logs)
	}
	if !r.hasLog(`✓ Script 'build' finished (code 0)`) {
		t.Errorf("log de éxito ausente: %v", r.logs)
	}
	if m.ActiveScriptName() != "" {
		t.Errorf("active name debe limpiarse, got %q", m.ActiveScriptName())
	}
}

func TestRunScriptFailure(t *testing.T) {
	m, r := newMgr(t)
	m.RunScript("lint", "cmd /c exit 3")
	waitFor(t, 8*time.Second, func() bool { return !m.IsRunning() })
	if !r.hasLog(`✕ Script 'lint' exited with code 3 (CrashExit)`) {
		t.Errorf("log de fallo ausente: %v", r.logs)
	}
	r.mu.Lock()
	last := r.finished[len(r.finished)-1]
	r.mu.Unlock()
	if last.name != "lint" || last.code != 3 {
		t.Errorf("finished = %v, want lint/3", last)
	}
}

func TestConcurrentRunRejected(t *testing.T) {
	m, r := newMgr(t)
	m.RunScript("long", "ping -n 30 127.0.0.1")
	waitFor(t, 8*time.Second, func() bool { return m.IsRunning() })

	m.RunScript("other", "cmd /c exit 0")
	if !r.hasLog(`⚠️ Cannot start 'other': script 'long' is already running.`) {
		t.Errorf("warning concurrente ausente: %v", r.logs)
	}
	if m.ActiveScriptName() != "long" {
		t.Errorf("activo debe seguir siendo long, got %q", m.ActiveScriptName())
	}

	m.Stop()
	waitFor(t, 8*time.Second, func() bool { return !m.IsRunning() })
}

func TestStopByUserEmitsFinishedZero(t *testing.T) {
	m, r := newMgr(t)
	m.RunScript("watch", "ping -n 30 127.0.0.1")
	waitFor(t, 8*time.Second, func() bool { return m.IsRunning() })

	m.Stop()
	waitFor(t, 8*time.Second, func() bool { return !m.IsRunning() })
	if !r.hasLog(`🛑 Stopping script 'watch'...`) {
		t.Errorf("log stopping ausente: %v", r.logs)
	}
	if !r.hasLog(`🛑 Script 'watch' stopped by user.`) {
		t.Errorf("log stopped-by-user ausente: %v", r.logs)
	}
	r.mu.Lock()
	last := r.finished[len(r.finished)-1]
	r.mu.Unlock()
	if last.name != "watch" || last.code != 0 {
		t.Errorf("finished tras stop = %v, want watch/0", last)
	}
}

func TestOutputPrefixedWithScriptName(t *testing.T) {
	m, r := newMgr(t)
	m.RunScript("hello", "cmd /c echo HOLA-SCRIPT")
	waitFor(t, 8*time.Second, func() bool { return !m.IsRunning() })
	if !r.hasLog("[hello] HOLA-SCRIPT") {
		t.Errorf("stdout sin prefijo [name]: %v", r.logs)
	}
}

func TestStopWhenIdleIsNoop(t *testing.T) {
	m, r := newMgr(t)
	m.Stop()
	time.Sleep(150 * time.Millisecond)
	r.mu.Lock()
	n := len(r.logs)
	r.mu.Unlock()
	if n != 0 {
		t.Errorf("stop en idle no debe loguear, got %v", r.logs)
	}
}
