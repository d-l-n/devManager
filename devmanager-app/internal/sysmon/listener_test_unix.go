//go:build !windows

package sysmon

import (
	"net"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// startListenerProc starts a real TCP listener for tests on Unix.
func startListenerProc(t *testing.T, port int) *exec.Cmd {
	t.Helper()
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("failed to listen on port %d: %v", port, err)
	}
	t.Cleanup(func() { _ = ln.Close() })

	// Spawn a dummy process that keeps the port alive via the listener
	cmd := exec.Command("sh", "-c", "sleep 60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if GetPortOwner(port) != nil {
			return cmd
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("listener no llegó a abrir el puerto")
	return nil
}
