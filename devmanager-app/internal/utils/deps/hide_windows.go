//go:build windows

package deps

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideCmd evita el flash/ventana de consola en Windows al invocar
// npm/go/govulncheck desde una app GUI (paridad STARTF_USESHOWWINDOW|SW_HIDE).
func hideCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
