package sysmon

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// levanta un listener real (PowerShell TcpListener) para tener dueño de puerto real
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

func TestGetPortOwner(t *testing.T) {
	port := 58131 // rango efímero poco probable
	startListenerProc(t, port)
	owner := GetPortOwner(port)
	if owner == nil || owner.PID <= 0 {
		t.Fatalf("dueño esperado en puerto %d, got %+v", port, owner)
	}
	if owner.Name == "" {
		t.Error("nombre de proceso no debe ser vacío")
	}
	if free := GetPortOwner(58132); free != nil {
		t.Errorf("puerto libre debe dar nil, got %+v", free)
	}
	if GetPortOwner(0) != nil || GetPortOwner(-1) != nil {
		t.Error("puerto inválido debe dar nil")
	}
}

func TestGetProcessTreeUsage(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	pid := cmd.Process.Pid

	first := GetProcessTreeUsage(pid)
	if first == nil {
		t.Fatal("primera lectura no debe ser nil")
	} // baseline: cpu 0 aceptada
	time.Sleep(1200 * time.Millisecond)
	second := GetProcessTreeUsage(pid)
	if second == nil {
		t.Fatal("segunda lectura no debe ser nil")
	}
	if second.CPUPercent < 0 || second.RSSMB <= 0 {
		t.Errorf("rss debe ser >0, got %+v", second)
	}
	if GetProcessTreeUsage(0) != nil {
		t.Error("pid 0 debe dar nil")
	}
	if dead := GetProcessTreeUsage(999999); dead != nil {
		t.Error("pid inexistente debe dar nil")
	}
}

func TestKillTree(t *testing.T) {
	cmd := exec.Command("ping", "-n", "30", "127.0.0.1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	ok, msg := KillTree(pid)
	if !ok {
		t.Fatalf("kill falló: %s", msg)
	}
	if strings.TrimSpace(msg) == "" {
		t.Error("mensaje no vacío esperado")
	}
	time.Sleep(300 * time.Millisecond)
	if ok2, _ := KillTree(999999); ok2 {
		t.Error("kill de pid inexistente debe fallar")
	}
}
