//go:build windows

package process

import (
	"os/exec"
	"strconv"
)

// killProcessTree kills a process and its children on Windows.
func killProcessTree(pid int) {
	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	_ = kill.Run()
}
