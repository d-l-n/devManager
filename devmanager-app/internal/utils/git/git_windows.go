//go:build windows

package git

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideCmd evita flash de consola en Windows (paridad STARTF_USESHOWWINDOW|SW_HIDE).
func hideCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
