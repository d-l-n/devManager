package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"desktop-go/internal/models"
)

func TestAddStateListenerReceivesTransitions(t *testing.T) {
	rec := &recorder{}
	m := NewManager(newTestProject(0, 1000), rec.callbacks())

	var mu sync.Mutex
	var seen []models.ServerState
	m.AddStateListener(func(s models.ServerState) {
		mu.Lock()
		seen = append(seen, s)
		mu.Unlock()
	})

	m.Start()
	waitForState(t, m, models.StateRunning, 5*time.Second)
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)

	mu.Lock()
	defer mu.Unlock()
	hasStarting, hasStopped := false, false
	for _, s := range seen {
		if s == models.StateStarting {
			hasStarting = true
		}
		if s == models.StateStopped {
			hasStopped = true
		}
	}
	if !hasStarting || !hasStopped {
		t.Errorf("listener debe ver starting y stopped, saw %v", seen)
	}
}

func TestAddReadyListenerFiresWhenRunning(t *testing.T) {
	m := NewManager(newTestProject(5173, 5000), (&recorder{}).callbacks())
	m.probePortFn = func(host string, port int) bool { return false }
	m.waitPortFn = func(ctx context.Context, timeout time.Duration) error { return nil }

	readyCh := make(chan struct{}, 4)
	m.AddReadyListener(func() { readyCh <- struct{}{} })

	m.Start()
	select {
	case <-readyCh:
	case <-time.After(5 * time.Second):
		t.Fatal("ready listener no disparado al llegar a RUNNING")
	}
	m.Stop()
	waitForState(t, m, models.StateStopped, 5*time.Second)
}
