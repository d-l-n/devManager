//go:build windows

package sysmon

import (
	"os/exec"
	"strconv"
	"testing"
	"time"
)

// startListenerProc starts a real TCP listener (PowerShell TcpListener) for tests.
func startListenerProc(t *testing.T, port int) *exec.Cmd {
	t.Helper()
	script := `$l=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,` + strconv.Itoa(port) + `);$l.Start();Start-Sleep -Seconds 60;$l.Stop()`
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
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
