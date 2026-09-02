package sysmon

import (
	"errors"
	"strings"
	"testing"
	"time"
)

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
	cmd := pingCmd()
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
	cmd := pingCmd()
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

// killParamsFake devuelve primitivas deterministas (raíz 3 procesos) sin tocar
// procesos reales; cada test sobreescribe lo que necesite.
func killParamsFake() killParams {
	return killParams{
		preCheck:     func(int) string { return "" },
		tree:         func(int) []int { return []int{1234, 1235, 1236} },
		kill:         func(int) error { return nil },
		alive:        func(int) bool { return false },
		pollTimeout:  10 * time.Millisecond,
		pollInterval: 2 * time.Millisecond,
	}
}

func TestKillTreeInvalidPid(t *testing.T) {
	if ok, msg := KillTree(0); ok || msg != "invalid pid" {
		t.Errorf("KillTree(0): esperado (false, \"invalid pid\"), got (%v, %q)", ok, msg)
	}
	p := killParamsFake()
	if ok, _ := killTreeVerified(0, p); ok {
		t.Error("pid 0 debe fallar")
	}
	if ok, _ := killTreeVerified(-1, p); ok {
		t.Error("pid negativo debe fallar")
	}
}

func TestKillTreeTerminated(t *testing.T) {
	ok, msg := killTreeVerified(1234, killParamsFake())
	if !ok || msg != "Terminated 3 process(es)" {
		t.Errorf("esperado (true, \"Terminated 3 process(es)\"), got (%v, %q)", ok, msg)
	}
}

func TestKillTreeSurvivors(t *testing.T) {
	p := killParamsFake()
	p.alive = func(int) bool { return true }
	ok, msg := killTreeVerified(1234, p)
	if ok || msg != "3 process(es) survived termination" {
		t.Errorf("esperado (false, \"3 process(es) survived termination\"), got (%v, %q)", ok, msg)
	}
}

func TestKillTreeRootKillFails(t *testing.T) {
	p := killParamsFake()
	p.kill = func(int) error { return errors.New("taskkill falló") }
	ok, msg := killTreeVerified(1234, p)
	if ok || !strings.HasPrefix(msg, "Failed killing root pid 1234:") {
		t.Errorf("esperado prefijo \"Failed killing root pid 1234:\", got (%v, %q)", ok, msg)
	}
}
