//go:build windows

package workflow

import (
	"context"
	"os/exec"
	"syscall"
)

// shellCommand envuelve command en cmd.exe /d /s /c con ventana oculta
// (paridad process.shellCommand: sin flashes de consola en la app GUI).
func shellCommand(ctx context.Context, command, dir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "cmd.exe", "/d", "/s", "/c", command)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}
	return cmd
}
