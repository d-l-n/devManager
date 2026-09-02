//go:build !windows

package process

import "os/exec"

// shellCommand wraps command with the platform shell.
// Unix: /bin/sh -c <command>
func shellCommand(command string) *exec.Cmd {
	return exec.Command("/bin/sh", "-c", command)
}
