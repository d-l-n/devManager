//go:build !windows

package workflow

import (
	"context"
	"os/exec"
)

// shellCommand envuelve command en /bin/sh -c.
func shellCommand(ctx context.Context, command, dir string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Dir = dir
	return cmd
}
