package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"desktop-go/internal/config"
	"desktop-go/internal/models"
	"desktop-go/internal/server"
)

type App struct {
	ctx context.Context
	mu  sync.Mutex

	cfg     *config.Manager
	servers map[int]*server.Manager

	configPath string
}

func NewApp() *App {
	return &App{servers: map[int]*server.Manager{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.configPath = "projects.json"

	a.cfg, _ = config.NewManager(a.configPath, config.Options{
		OnProjectsChanged: func() { runtime.EventsEmit(a.ctx, "projects:changed") },
		OnError: func(msg string) {
			runtime.EventsEmit(a.ctx, "config:error", map[string]string{"message": msg})
		},
	})
}

// shutdown detiene todos los servidores al cerrar la ventana.
func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	servers := make([]*server.Manager, 0, len(a.servers))
	for _, sm := range a.servers {
		servers = append(servers, sm)
	}
	a.mu.Unlock()
	for _, sm := range servers {
		sm.Stop()
	}
}

func (a *App) GetProjects() []models.Project {
	return a.cfg.Projects()
}

func (a *App) AddProject(p models.Project) []string {
	errs := p.Validate()
	if len(errs) > 0 {
		return errs
	}
	if err := a.cfg.AddProject(p); err != nil {
		return []string{err.Error()}
	}
	return nil
}

func (a *App) UpdateProject(index int, p models.Project) []string {
	errs := p.Validate()
	if len(errs) > 0 {
		return errs
	}
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	a.mu.Lock()
	sm, ok := a.servers[index]
	a.mu.Unlock()
	if ok {
		sm.UpdateProject(p)
	}
	return nil
}

func (a *App) RemoveProject(index int) {
	// Paridad UI Python: no se puede remover con servidor corriendo.
	a.mu.Lock()
	_, running := a.servers[index]
	a.mu.Unlock()
	if running {
		a.emitConfigError("Cannot remove project while its server is running")
		return
	}
	_ = a.cfg.RemoveProject(index)
}

func (a *App) TogglePin(index int) {
	_ = a.cfg.TogglePin(index)
}

func (a *App) StartServer(index int) {
	sm := a.managerFor(index)
	if sm != nil {
		sm.Start()
	}
}

func (a *App) StopServer(index int) {
	a.mu.Lock()
	sm := a.servers[index]
	a.mu.Unlock()
	if sm != nil {
		sm.Stop()
	}
}

func (a *App) RestartServer(index int) {
	sm := a.managerFor(index)
	if sm != nil {
		sm.Restart()
	}
}

type ServerStatus struct {
	State         string  `json:"state"`
	ActivePort    int     `json:"activePort"`
	ActiveURL     string  `json:"activeUrl"`
	UptimeSeconds float64 `json:"uptimeSeconds"`
	FailureReason string  `json:"failureReason"`
	Running       bool    `json:"running"`
}

func (a *App) GetServerStatus(index int) ServerStatus {
	a.mu.Lock()
	sm := a.servers[index]
	a.mu.Unlock()
	if sm == nil {
		return ServerStatus{State: string(models.StateStopped)}
	}
	st := ServerStatus{
		State:         string(sm.State()),
		ActivePort:    sm.ActivePort(),
		ActiveURL:     sm.ActiveURL(),
		FailureReason: sm.FailureReason(),
		Running:       sm.State() == models.StateRunning || sm.State() == models.StateStarting,
	}
	if at, ok := sm.StartedAt(); ok {
		st.UptimeSeconds = time.Since(at).Seconds()
	}
	return st
}

// managerFor crea el server.Manager bajo demanda y conecta sus callbacks
// a eventos Wails (equivalente a la conexión signals del MainWindow Qt).
func (a *App) managerFor(index int) *server.Manager {
	a.mu.Lock()
	if sm, ok := a.servers[index]; ok {
		a.mu.Unlock()
		return sm
	}
	a.mu.Unlock()

	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return nil
	}
	project := projects[index]

	cb := server.Callbacks{
		OnStateChange: func(state models.ServerState) {
			runtime.EventsEmit(a.ctx, "server:state", map[string]interface{}{
				"index": index, "state": string(state),
			})
		},
		OnLog: func(line string, isError bool) {
			runtime.EventsEmit(a.ctx, "server:log", map[string]interface{}{
				"index": index, "line": line, "isError": isError,
			})
		},
		OnReady: func() {
			runtime.EventsEmit(a.ctx, "server:ready", map[string]interface{}{"index": index})
		},
		OnPortDetected: func(port int, url string) {
			runtime.EventsEmit(a.ctx, "server:port_detected", map[string]interface{}{
				"index": index, "port": port, "url": url,
			})
		},
		OnPortMismatch: func(configured, detected int, activeURL string) {
			runtime.EventsEmit(a.ctx, "server:port_mismatch", map[string]interface{}{
				"index": index, "configured": configured, "detected": detected, "url": activeURL,
			})
		},
	}

	sm := server.NewManager(project, cb)

	a.mu.Lock()
	a.servers[index] = sm
	a.mu.Unlock()
	return sm
}

func (a *App) emitConfigError(message string) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "config:error", map[string]string{"message": message})
	} else {
		fmt.Println("config error:", message)
	}
}
