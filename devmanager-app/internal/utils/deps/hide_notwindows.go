//go:build !windows

package deps

import "os/exec"

// hideCmd es un no-op en plataformas sin ventana de consola.
func hideCmd(cmd *exec.Cmd) {}
