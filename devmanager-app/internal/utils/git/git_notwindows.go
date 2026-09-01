//go:build !windows

package git

import "os/exec"

// hideCmd is a no-op on non-Windows platforms.
func hideCmd(cmd *exec.Cmd) {}
