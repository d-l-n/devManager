package testadv

import (
	"strings"
	"sync"
	"testing"
)

func TestPlanMatrixRunSoloPending(t *testing.T) {
	cells := BuildMatrix([]string{"chromium", "firefox"}, []string{"dev", "prod"})
	cells = ApplyResults(cells, map[string]MatrixCell{
		MatrixKey("chromium", "dev"): {Status: "passed"},
	})
	plan := PlanMatrixRun("Alpha", "npx playwright test", cells, false, 0)
	if plan.MaxWorker != 4 {
		t.Fatalf("default workers = %d, want 4", plan.MaxWorker)
	}
	if len(plan.Cells) != 3 {
		t.Fatalf("solo pending: got %d celdas, want 3", len(plan.Cells))
	}
	for _, c := range plan.Cells {
		if c.Status != "pending" {
			t.Fatalf("celda no pending en plan: %+v", c)
		}
	}
	// allCells=true incluye las ya pasadas.
	all := PlanMatrixRun("Alpha", "cmd", cells, true, 12)
	if len(all.Cells) != 4 {
		t.Fatalf("allCells: got %d, want 4", len(all.Cells))
	}
	if all.MaxWorker != 8 {
		t.Fatalf("workers acotado a 8: got %d", all.MaxWorker)
	}
}

func TestRunMatrixPlanEjecutaTodoYEnParalelo(t *testing.T) {
	cells := BuildMatrix([]string{"chromium", "firefox"}, []string{"dev", "prod"})
	plan := PlanMatrixRun("Alpha", "run-tests", cells, true, 4)

	var mu sync.Mutex
	concurrent := 0
	maxConcurrent := 0
	seen := map[string]bool{}
	// Barrier: cada runner espera a que los 4 workers entren, forzando el
	// solape real (sin sleeps ni flakiness de timing).
	inside := sync.WaitGroup{}
	inside.Add(4)
	release := make(chan struct{})
	go func() {
		inside.Wait()
		close(release)
	}()

	results := RunMatrixPlan(plan, ".", func(cmd string, env []string, cwd string) (int, error) {
		mu.Lock()
		concurrent++
		if concurrent > maxConcurrent {
			maxConcurrent = concurrent
		}
		mu.Unlock()
		var browser, envName string
		for _, kv := range env {
			if strings.HasPrefix(kv, "DM_TEST_BROWSER=") {
				browser = strings.TrimPrefix(kv, "DM_TEST_BROWSER=")
			}
			if strings.HasPrefix(kv, "DM_TEST_ENV=") {
				envName = strings.TrimPrefix(kv, "DM_TEST_ENV=")
			}
		}
		mu.Lock()
		seen[MatrixKey(browser, envName)] = true
		mu.Unlock()
		inside.Done()   // aviso: este worker llegó
		<-release       // espera al barrier de 4
		mu.Lock()
		concurrent--
		mu.Unlock()
		return 0, nil
	})

	if len(results) != 4 {
		t.Fatalf("results = %d, want 4", len(results))
	}
	for _, r := range results {
		if r.ExitCode != 0 || r.Error != "" {
			t.Fatalf("celda fallida: %+v", r)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("celdas ejecutadas = %d, want 4", len(seen))
	}
	if maxConcurrent < 4 {
		t.Fatalf("no hubo paralelismo real (max concurrent = %d, want 4)", maxConcurrent)
	}
}

func TestRunMatrixPlanCapturaFallos(t *testing.T) {
	cells := BuildMatrix([]string{"chromium"}, []string{"dev"})
	plan := PlanMatrixRun("A", "cmd", cells, true, 1)
	results := RunMatrixPlan(plan, ".", func(cmd string, env []string, cwd string) (int, error) {
		return 3, nil // exit code 3 sin error de arranque
	})
	if len(results) != 1 || results[0].ExitCode != 3 {
		t.Fatalf("exit code no capturado: %+v", results)
	}
	// Comando vacío rechazado.
	results = RunMatrixPlan(PlanMatrixRun("A", "", cells, true, 1), ".", nil)
	if len(results) != 1 || results[0].Error == "" {
		t.Fatalf("comando vacío debe fallar: %+v", results)
	}
}
