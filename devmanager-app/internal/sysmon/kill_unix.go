//go:build !windows

package sysmon

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/shirou/gopsutil/v3/process"
)

// runKillTree kills a process tree on Unix using SIGKILL.
func runKillTree(pid int) error {
	root, err := process.NewProcess(int32(pid))
	if err != nil {
		return fmt.Errorf("process %d not found: %w", pid, err)
	}
	// Kill children first (bottom-up)
	kids, _ := root.Children()
	for _, k := range kids {
		_ = syscall.Kill(int(k.Pid), syscall.SIGKILL)
	}
	// Kill root
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil {
		// Fallback to kill command
		return exec.Command("kill", "-9", strconv.Itoa(pid)).Run()
	}
	return nil
}
