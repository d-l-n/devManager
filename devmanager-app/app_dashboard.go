package main

import (
	"path/filepath"
	"time"

	"github.com/d-l-n/devmanager/internal/dashboard"
)

// ---- Dashboard bindings (Issue #64) ----

const (
	// dashboardSampleInterval: frecuencia del sampler de uptime (60s).
	dashboardSampleInterval = dashboard.SampleInterval
	// dashboardHistoryWindow: ventana devuelta al frontend.
	dashboardHistoryWindow = 24 * time.Hour
)

// GetDashboardHistory devuelve el historial de uptime (ventana 24h) por
// proyecto, ordenado por nombre.
func (a *App) GetDashboardHistory() []dashboard.ProjectHistory {
	return a.historyStore.Since(dashboardHistoryWindow)
}

// startDashboardSampler crea el store y lanza la goroutine que muestrea
// uptime por proyecto cada 60s (patrón backup scheduler: chequeo inicial +
// ticker + ctx.Done para terminar).
func (a *App) startDashboardSampler() {
	a.historyStore = dashboard.NewStore(filepath.Join(filepath.Dir(a.settingsPath), "dashboard-history.json"))

	go func() {
		a.sampleDashboard() // sample inicial: sesiones cortas también dejan rastro
		ticker := time.NewTicker(dashboardSampleInterval)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.sampleDashboard()
			}
		}
	}()
}

// serverManagerRef copia inmutable del estado de un manager para samplear
// fuera del lock del App.
type serverManagerRef struct {
	running   bool
	startedAt time.Time
}

// sampleDashboard toma un sample por proyecto: running + uptime desde
// StartedAt del manager. CPU/RSS se añaden desde GetMonitorData si hay polling.
func (a *App) sampleDashboard() {
	if a.ctx == nil {
		return
	}
	now := time.Now()
	projects := a.cfg.Projects()

	// Snapshot bajo lock del mapa de servers (mismo patrón runningServersSnapshot).
	a.mu.Lock()
	servers := make(map[int]serverManagerRef, len(a.servers))
	for idx, sm := range a.servers {
		st, ok := sm.StartedAt()
		servers[idx] = serverManagerRef{running: ok, startedAt: st}
	}
	polling := a.settings.MonitorPolling
	a.mu.Unlock()

	for i, p := range projects {
		running := false
		var uptime int64
		var cpu, rss float64
		if ref, ok := servers[i]; ok && ref.running {
			running = true
			uptime = int64(now.Sub(ref.startedAt).Seconds())
			// CPU/RSS del sample: solo si el polling global está activo.
			if polling {
				if u := a.sampleUsage(p.Name); u != nil {
					cpu, rss = u.cpu, u.rss
				}
			}
		}
		_ = a.historyStore.Append(p.Name, dashboard.Sample{
			Ts: now.Unix(), Running: running, Uptime: uptime, CPU: cpu, RSS: rss,
		})
	}
}

// usageSample es CPU/RSS de un proyecto para el sampler.
type usageSample struct {
	cpu float64
	rss float64
}

// sampleUsage busca el ResRow del proyecto dentro de GetMonitorData (cacheado
// 500ms; el sampler lo llama 1/min así que siempre refresca).
func (a *App) sampleUsage(name string) *usageSample {
	data := a.GetMonitorData()
	for _, row := range data.ResRows {
		if row.Name == name {
			return &usageSample{cpu: row.CPU, rss: row.RSS}
		}
	}
	return nil
}

// flushDashboardHistory persiste el estado final al cerrar (shutdown).
func (a *App) flushDashboardHistory() {
	a.sampleDashboard()
}
