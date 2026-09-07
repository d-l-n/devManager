//go:build windows

package process

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// hideConsole evita que un proceso hijo abra una consola visible desde la app
// GUI (paridad STARTF_USESHOWWINDOW|SW_HIDE). Aplica a comandos sin shell
// (StartArgv, taskkill) además de shellCommand.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// shellCommand wraps command with the platform shell.
// Windows: cmd.exe /d /s /c <command> con ventana oculta.
func shellCommand(command string) *exec.Cmd {
	cmd := exec.Command("cmd.exe", "/d", "/s", "/c", command)
	hideConsole(cmd)
	return cmd
}
