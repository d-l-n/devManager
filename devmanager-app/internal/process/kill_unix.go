//go:build !windows

package process

import (
	"os/exec"
	"strconv"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"
)

// killProcessTree kills a process and its children on Unix.
// Uses SIGKILL on the direct children (gopsutil Children provides the tree).
func killProcessTree(pid int) {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return
	}
	// Kill children first (bottom-up)
	kids, _ := root.Children()
	for _, k := range kids {
		_ = syscall.Kill(int(k.Pid), syscall.SIGKILL)
	}
	// Then kill root
	_ = syscall.Kill(pid, syscall.SIGKILL)

	// Fallback: if kill didn't work, try pkill-style
	_ = exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
}
