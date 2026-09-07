//go:build !windows

package process

import "os/exec"

// hideConsole is a no-op on non-Windows platforms.
func hideConsole(cmd *exec.Cmd) {}

// shellCommand wraps command with the platform shell.
// Unix: /bin/sh -c <command>
func shellCommand(command string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", command)
}
