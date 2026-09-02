package process

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/testutil"
)

func TestStripANSI(t *testing.T) {
	cases := map[string]string{
		"\x1b[31mrojo\x1b[0m":        "rojo",
		"\x1b[1;32mok\x1b[0m fin":    "ok fin",
		"sin escapes":                "sin escapes",
		"\x1b]0;titulo\x07contenido": "contenido",
	}
	for in, want := range cases {
		if got := StripANSI(in); got != want {
			t.Errorf("StripANSI(%q) = %q, want %q", in, got, want)
		}
	}
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

func TestRunnerLifecycle(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	finished := make(chan struct{})

	cb := RunnerCallbacks{
		OnStdout:  func(l string) { mu.Lock(); lines = append(lines, l); mu.Unlock() },
		OnStarted: func() {},
		OnFinished: func(code int, status string) {
			mu.Lock()
			t.Logf("finished code=%d status=%s", code, status)
			mu.Unlock()
			close(finished)
		},
		OnError: func(desc string) { t.Logf("runner error: %s", desc) },
	}

	r := NewRunner(cb)
	if err := r.Start(testutil.PingCmdStr(), ".", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !r.IsRunning() {
		t.Fatal("debería estar running tras Start")
	}
	if r.PID() <= 0 {
		t.Fatal("PID debe ser positivo")
	}

	waitFor(t, 5*time.Second, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(lines) > 0
	})

	r.Stop()

	select {
	case <-finished:
	case <-time.After(8 * time.Second):
		t.Fatal("Stop no produjo OnFinished en 8s")
	}
	if r.IsRunning() {
		t.Error("tras Stop no debe seguir running")
	}
}

func TestRunnerNormalExit(t *testing.T) {
	finished := make(chan struct{})
	var status string
	var code int
	cb := RunnerCallbacks{
		OnFinished: func(c int, s string) { code, status = c, s; close(finished) },
	}
	r := NewRunner(cb)
	if err := r.Start(testutil.ExitCmdStr(0), ".", nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("proceso trivial debió terminar")
	}
	if status != "NormalExit" || code != 0 {
		t.Errorf("exit limpio esperado NormalExit/0, got %s/%d", status, code)
	}
}

func TestRunnerExtraEnv(t *testing.T) {
	finished := make(chan struct{})
	var mu sync.Mutex
	var out strings.Builder
	cb := RunnerCallbacks{
		OnStdout:   func(l string) { mu.Lock(); out.WriteString(l + "\n"); mu.Unlock() },
		OnFinished: func(int, string) { close(finished) },
	}
	r := NewRunner(cb)
	if err := r.Start(testutil.EchoEnvCmdStr("PORT"), ".", map[string]string{"PORT": "4321"}); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout")
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(out.String(), "PORT=4321") {
		t.Errorf("extra_env no aplicado, salida: %q", out.String())
	}
}
