package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Listener es el webhook entrante: POST /hook/{workflowId} en localhost.
// Arranca/stop graceful; si el puerto está ocupado Start devuelve error y el
// caller lo loguea sin bloquear el boot.
type Listener struct {
	mu     sync.Mutex
	addr   string
	server *http.Server
	onHook func(workflowID string, payload map[string]interface{})
}

// NewListener crea el listener en host:port con el callback de disparo.
func NewListener(host string, port int, onHook func(string, map[string]interface{})) *Listener {
	return &Listener{
		addr:   fmt.Sprintf("%s:%d", host, port),
		onHook: onHook,
	}
}

// Start bindea y sirve en background. Error inmediato si no puede bindear.
func (l *Listener) Start() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hook/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/hook/")
		id = strings.TrimSpace(strings.SplitN(id, "/", 2)[0])
		if id == "" {
			http.Error(w, "missing workflow id", http.StatusBadRequest)
			return
		}
		var payload map[string]interface{}
		if r.Body != nil {
			raw, _ := io.ReadAll(io.LimitReader(r.Body, 64*1024))
			if len(raw) > 0 {
				_ = json.Unmarshal(raw, &payload)
			}
		}
		if payload == nil {
			payload = map[string]interface{}{}
		}
		cb := l.onHook
		if cb != nil {
			go cb(id, payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	srv := &http.Server{
		Addr:              l.addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	ln, err := net.Listen("tcp", l.addr)
	if err != nil {
		return err
	}
	l.mu.Lock()
	l.server = srv
	actual := ln.Addr().String()
	l.mu.Unlock()
	go func() {
		_ = srv.Serve(ln)
	}()
	l.mu.Lock()
	l.addr = actual
	l.mu.Unlock()
	return nil
}

// Stop cierra graceful (timeout 3s).
func (l *Listener) Stop() {
	l.mu.Lock()
	srv := l.server
	l.server = nil
	l.mu.Unlock()
	if srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}
}

// Addr devuelve host:port efectivo.
func (l *Listener) Addr() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.addr
}
