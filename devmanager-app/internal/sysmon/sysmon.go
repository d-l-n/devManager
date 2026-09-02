// Package sysmon porta app/process/monitor.py: dueños de puerto,
// uso CPU/RAM por árbol de procesos y kill_tree vía taskkill.
//
// Paridad CPU%: los objetos *process.Process se cachean por pid para que
// la baseline interna persista entre polls. El primer poll reporta ~0;
// valores reales desde el segundo (cadencia 3s en MonitorPanel).
package sysmon

import (
	"fmt"
	"math"
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
		// First pass: evict dead pids
		for k := range procCache {
			exists, _ := process.PidExists(int32(k))
			if !exists {
				delete(procCache, k)
			}
		}
		// Second pass: if still full, evict oldest (LRU) entry
		if len(procCache) >= maxProcCache {
			var oldestKey int
			var oldestTime time.Time
			first := true
			for k, e := range procCache {
				if first || e.last.Before(oldestTime) {
					oldestKey = k
					oldestTime = e.last
					first = false
				}
			}
			delete(procCache, oldestKey)
		}
	}
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil
	}
	_, _ = proc.Percent(0) // prime baseline (primera lectura ~0, paridad psutil)
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

// Constantes del poll de supervivencia tras el kill (paridad wait_procs 3s).
const (
	killPollTimeout  = 3 * time.Second
	killPollInterval = 100 * time.Millisecond
)

// killParams inyecta las primitivas de kill_tree para testear sin procesos reales.
type killParams struct {
	// preCheck emula el paso 2 de kill_tree: mensaje de error si el raíz no
	// existe o está sin acceso; "" si alcanzable.
	preCheck func(pid int) string
	// tree devuelve los pids del árbol a matar (raíz + hijos recursivos).
	tree func(pid int) []int
	// kill ejecuta la matanza del árbol.
	kill func(pid int) error
	// aliveReporta si el proceso pid sigue vivo tras el kill.
	alive func(pid int) bool
	// pollTimeout/pollInterval gobiernan la verificación post-kill.
	pollTimeout  time.Duration
	pollInterval time.Duration
}

// KillTree mata el proceso y su árbol y verifica que no
// queden supervivientes (paridad monitor.kill_tree).
func KillTree(pid int) (bool, string) {
	return killTreeVerified(pid, killParams{
		preCheck:     preCheckRoot,
		tree:         killTreePids,
		kill:         runKillTree,
		alive:        pidAlive,
		pollTimeout:  killPollTimeout,
		pollInterval: killPollInterval,
	})
}

// killTreeVerified implementa kill_tree con primitivas inyectables.
func killTreeVerified(pid int, p killParams) (bool, string) {
	if pid <= 0 {
		return false, "invalid pid"
	}
	timeout := p.pollTimeout
	if timeout <= 0 {
		timeout = killPollTimeout
	}
	interval := p.pollInterval
	if interval <= 0 {
		interval = killPollInterval
	}
	if msg := p.preCheck(pid); msg != "" {
		return false, msg
	}
	tree := p.tree(pid)
	if err := p.kill(pid); err != nil {
		return false, fmt.Sprintf("Failed killing root pid %d: %v", pid, err)
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !p.alive(pid) {
			return true, fmt.Sprintf("Terminated %d process(es)", len(tree))
		}
		time.Sleep(interval)
	}
	var survivors int
	for _, tp := range tree {
		if p.alive(tp) {
			survivors++
		}
	}
	return false, fmt.Sprintf("%d process(es) survived termination", survivors)
}

// preCheckRoot emula el paso 2 de kill_tree: raíz inexistente o sin acceso.
func preCheckRoot(pid int) string {
	if _, err := process.NewProcess(int32(pid)); err != nil {
		exists, _ := process.PidExists(int32(pid))
		if exists {
			return fmt.Sprintf("Access denied killing pid %d", pid)
		}
		return fmt.Sprintf("No such process (pid %d)", pid)
	}
	return ""
}

// killTreePids devuelve los pids a matar: raíz + hijos recursivos.
func killTreePids(pid int) []int {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return nil
	}
	pids := []int{int(root.Pid)}
	for _, c := range childrenRecursive(root) {
		pids = append(pids, int(c.Pid))
	}
	return pids
}

// runKillTree is defined per-platform in kill_windows.go / kill_unix.go.

// pidAlive reporta si el proceso sigue existiendo (paridad pid_exists).
func pidAlive(pid int) bool {
	exists, _ := process.PidExists(int32(pid))
	return exists
}
