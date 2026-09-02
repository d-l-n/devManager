//go:build windows

package process

import "os/exec"

// shellCommand wraps command with the platform shell.
// Windows: cmd.exe /d /s /c <command>
func shellCommand(command string) *exec.Cmd {
	return exec.Command("cmd.exe", "/d", "/s", "/c", command)
}
