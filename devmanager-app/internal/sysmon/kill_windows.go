//go:build windows

package sysmon

import (
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// runKillTree kills a process tree on Windows using taskkill /T /F.
// taskkill corre oculto para no abrir una consola visible desde la app GUI.
func runKillTree(pid int) error {
	cmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd.Run()
}
