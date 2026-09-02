//go:build windows

package sysmon

import (
	"os/exec"
	"strconv"
)

// runKillTree kills a process tree on Windows using taskkill /T /F.
func runKillTree(pid int) error {
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}
