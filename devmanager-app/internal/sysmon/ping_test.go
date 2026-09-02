//go:build !windows

package sysmon

import "os/exec"

// pingCmd returns a long-running ping command suitable for tests on Unix.
func pingCmd() *exec.Cmd {
	return exec.Command("ping", "-c", "30", "127.0.0.1")
}
