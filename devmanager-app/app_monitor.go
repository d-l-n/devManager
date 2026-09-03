package main

import (
	"fmt"
	"sort"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/server"
	"github.com/d-l-n/devmanager/internal/sysmon"
)

// ---- Monitor bindings (paridad _refresh_monitor_data) ----

type PortRow struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Port      int    `json:"port"`
	State     string `json:"state"` // "ours" | "foreign" | "free"
	OwnerName string `json:"ownerName"`
	OwnerPID  int    `json:"ownerPID"`
}

type ResRow struct {
	Name     string  `json:"name"`
	PID      int     `json:"pid"`
	Children int     `json:"children"`
	CPU      float64 `json:"cpu"`
	RSS      float64 `json:"rss"`
}

type MonitorData struct {
	PortRows []PortRow `json:"portRows"`
	ResRows  []ResRow  `json:"resRows"`
}

type runningServer struct {
	index   int
	project models.Project
	manager *server.Manager
}

// runningServersSnapshot copia bajo lock el mapa de managers junto a los
// proyectos actuales, quedándose solo con los RUNNING. Orden estable por índice.
func (a *App) runningServersSnapshot() []runningServer {
	a.mu.Lock()
	servers := make(map[int]*server.Manager, len(a.servers))
	for idx, sm := range a.servers {
		servers[idx] = sm
	}
	a.mu.Unlock()

	projects := a.cfg.Projects()
	out := make([]runningServer, 0, len(servers))
	for idx, sm := range servers {
		if idx < 0 || idx >= len(projects) || sm.State() != models.StateRunning {
			continue
		}
		out = append(out, runningServer{index: idx, project: projects[idx], manager: sm})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].index < out[j].index })
	return out
}

const monitorDebounce = 500 * time.Millisecond

func (a *App) GetMonitorData() MonitorData {
	a.monitorMu.Lock()
	if a.monitorValid && time.Since(a.monitorCachedAt) < monitorDebounce {
		cached := a.monitorCache
		a.monitorMu.Unlock()
		return cached
	}
	a.monitorMu.Unlock()

	running := a.runningServersSnapshot()

	// Mapa puerto activo → nombre de proyecto (solo servers RUNNING propios).
	oursByPort := map[int]string{}
	for _, rs := range running {
		port := rs.manager.ActivePort()
		if port > 0 {
			if _, taken := oursByPort[port]; !taken {
				oursByPort[port] = rs.project.Name
			}
		}
	}

	data := MonitorData{PortRows: []PortRow{}, ResRows: []ResRow{}}
	projects := a.cfg.Projects()
	for i, p := range projects {
		if !p.Server.Enabled || p.Server.Port <= 0 {
			continue
		}
		row := PortRow{Index: i, Name: p.Name, Port: p.Server.Port, State: "free"}
		if owner, ok := oursByPort[p.Server.Port]; ok {
			row.State = "ours"
			row.OwnerName = owner
		} else if o := sysmon.GetPortOwner(p.Server.Port); o != nil {
			row.State = "foreign"
			row.OwnerName = o.Name
			row.OwnerPID = o.PID
		}
		data.PortRows = append(data.PortRows, row)
	}

	a.mu.Lock()
	polling := a.settings.MonitorPolling
	a.mu.Unlock()
	if polling {
		for _, rs := range running {
			pid := rs.manager.PID()
			if pid <= 0 {
				continue
			}
			u := sysmon.GetProcessTreeUsage(pid)
			if u == nil {
				continue
			}
			data.ResRows = append(data.ResRows, ResRow{
				Name: rs.project.Name, PID: u.PID, Children: u.Children,
				CPU: u.CPUPercent, RSS: u.RSSMB,
			})
		}
	}
	a.monitorMu.Lock()
	a.monitorCache = data
	a.monitorCachedAt = time.Now()
	a.monitorValid = true
	a.monitorMu.Unlock()
	return data
}

// ---- Kill tree (confirm lo hace el frontend) ----

type NotifyResult struct {
	Ok      bool   `json:"ok"`
	Message string `json:"message"`
}

func (a *App) KillTree(pid int) NotifyResult {
	ok, msg := sysmon.KillTree(pid)
	level := "error"
	message := fmt.Sprintf("Failed killing PID %d: %s", pid, msg)
	title := "System"
	if ok {
		level = "success"
		message = fmt.Sprintf("Process tree %d terminated", pid)
	}
	a.emitNotify(title, message, level)
	return NotifyResult{Ok: ok, Message: msg}
}
