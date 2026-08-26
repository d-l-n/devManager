// Package sysmon porta app/process/monitor.py: dueños de puerto,
// uso CPU/RAM por árbol de procesos y kill_tree vía taskkill.
//
// Paridad CPU%: los objetos *process.Process se cachean por pid para que
// la baseline interna persista entre polls. El primer poll reporta ≈0;
// valores reales desde el segundo (cadencia 3s en MonitorPanel).
package sysmon

import (
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// PortOwner replica monitor.PortOwner.
type PortOwner struct {
	PID  int
	Name string
}

// ProcessUsage replica monitor.ProcessUsage.
type ProcessUsage struct {
	PID        int
	Name       string
	CPUPercent float64
	RSSMB      float64
	Children   int
}

const maxProcCache = 128

type procEntry struct {
	proc *process.Process
	last time.Time
}

var (
	procMu    sync.Mutex
	procCache = map[int]*procEntry{}
)

// getCachedProcess devuelve el proceso cacheado o lo crea primando la
// baseline de CPU. Evicción de pids muertos al llegar al máximo.
func getCachedProcess(pid int) *process.Process {
	procMu.Lock()
	defer procMu.Unlock()
	if e, ok := procCache[pid]; ok {
		e.last = time.Now()
		return e.proc
	}
	if len(procCache) >= maxProcCache {
		for k := range procCache {
			exists, _ := process.PidExists(int32(k))
			if !exists {
				delete(procCache, k)
			}
		}
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil
	}
	_, _ = proc.Percent(0) // prime baseline (primera lectura ≈0, paridad psutil)
	entry := &procEntry{proc: proc, last: time.Now()}
	procCache[pid] = entry
	return proc
}

// GetPortOwner devuelve el proceso escuchando en port, o nil si libre/error.
func GetPortOwner(port int) *PortOwner {
	if port <= 0 {
		return nil
	}
	conns, err := net.Connections("inet")
	if err != nil {
		return nil
	}
	for _, c := range conns {
		if c.Status != "LISTEN" || c.Laddr.Port != uint32(port) || c.Pid <= 0 {
			continue
		}
		name := fmt.Sprintf("PID %d", c.Pid)
		if proc, err := process.NewProcess(c.Pid); err == nil {
			if n, err := proc.Name(); err == nil && n != "" {
				name = n
			}
		}
		return &PortOwner{PID: int(c.Pid), Name: name}
	}
	return nil
}

// childrenRecursive emula psutil children(recursive=True): BFS sobre hijos directos.
func childrenRecursive(root *process.Process) []*process.Process {
	var out []*process.Process
	pending := []*process.Process{root}
	seen := map[int32]bool{root.Pid: true}
	for len(pending) > 0 {
		cur := pending[0]
		pending = pending[1:]
		kids, err := cur.Children()
		if err != nil {
			continue
		}
		for _, k := range kids {
			if seen[k.Pid] {
				continue
			}
			seen[k.Pid] = true
			out = append(out, k)
			pending = append(pending, k)
		}
	}
	return out
}

// GetProcessTreeUsage agrega CPU%/RSS del árbol. CPU válida desde 2º poll.
func GetProcessTreeUsage(pid int) *ProcessUsage {
	if pid <= 0 {
		return nil
	}
	root := getCachedProcess(pid)
	if root == nil {
		return nil
	}
	procs := append([]*process.Process{root}, childrenRecursive(root)...)

	totalCPU := 0.0
	var totalRSS uint64
	for _, p := range procs {
		cached := getCachedProcess(int(p.Pid))
		if cached == nil {
			continue
		}
		if pct, err := cached.Percent(0); err == nil {
			totalCPU += pct
		}
		if mi, err := cached.MemoryInfo(); err == nil && mi != nil {
			totalRSS += mi.RSS
		}
	}

	name, _ := root.Name()
	return &ProcessUsage{
		PID:        pid,
		Name:       name,
		CPUPercent: math.Round(totalCPU*10) / 10,
		RSSMB:      math.Round(float64(totalRSS)/(1024*1024)*10) / 10,
		Children:   len(procs) - 1,
	}
}

// KillTree mata el proceso y sus hijos con taskkill /T /F.
func KillTree(pid int) (bool, string) {
	if pid <= 0 {
		return false, "invalid pid"
	}
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return false, err.Error()
	}
	return true, fmt.Sprintf("Terminated process tree %d", pid)
}
