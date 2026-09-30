package testadv

import (
	"fmt"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// RunPlan es la ejecución paralela de la matriz navegador x entorno: cada
// celda pending se materializa como un comando con proyecto/browsers/env en
// env vars DM_TEST_* (el runner real las consume; acá solo orquestamos y
// capturamos resultados en el histórico).
type RunPlan struct {
	Project   string   `json:"project"`
	Command   string   `json:"command"`
	Cells     []MatrixCell `json:"cells"`
	MaxWorker int      `json:"maxWorkers"`
}

// PlanMatrixRun genera el plan de ejecución de las celdas pendientes (o de
// todas si allCells). El comando base recibe DM_TEST_BROWSER/DM_TEST_ENV por
// celda. MaxWorker se acota a 1-8 (0 → 4).
func PlanMatrixRun(project, command string, cells []MatrixCell, allCells bool, maxWorkers int) RunPlan {
	if maxWorkers <= 0 || maxWorkers > 8 {
		if maxWorkers > 8 {
			maxWorkers = 8
		} else {
			maxWorkers = 4
		}
	}
	targets := []MatrixCell{}
	for _, c := range cells {
		if allCells || c.Status == "pending" {
			targets = append(targets, c)
		}
	}
	sort.Slice(targets, func(i, j int) bool { return MatrixKey(targets[i].Browser, targets[j].Env) < MatrixKey(targets[j].Browser, targets[i].Env) })
	return RunPlan{Project: project, Command: command, Cells: targets, MaxWorker: maxWorkers}
}

// RunMatrixResult es el resultado de una celda ejecutada.
type RunMatrixResult struct {
	Browser  string `json:"browser"`
	Env      string `json:"env"`
	ExitCode int    `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

// RunMatrixPlan ejecuta las celdas del plan con un pool de MaxWorker workers
// y devuelve los resultados en orden estable (mismo orden que plan.Cells).
// Cada celda corre `command` con DM_TEST_BROWSER/DM_TEST_ENV y el cwd del
// proyecto; el shell lo resuelve el runner de scripts (aquí exec directo).
func RunMatrixPlan(plan RunPlan, cwd string, runner func(cmd string, env []string, cwd string) (int, error)) []RunMatrixResult {
	if runner == nil {
		runner = defaultRunner
	}
	results := make([]RunMatrixResult, len(plan.Cells))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < plan.MaxWorker; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				c := plan.Cells[i]
				env := []string{
					"DM_TEST_BROWSER=" + c.Browser,
					"DM_TEST_ENV=" + c.Env,
				}
				code, err := runner(plan.Command, env, cwd)
				results[i] = RunMatrixResult{Browser: c.Browser, Env: c.Env, ExitCode: code}
				if err != nil {
					results[i].Error = err.Error()
				}
			}
		}()
	}
	for i := range plan.Cells {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return results
}

// defaultRunner ejecuta el comando con env extra vía `sh -c`/`cmd /c`.
// Separado para poder inyectar un runner falso en tests.
func defaultRunner(command string, env []string, cwd string) (int, error) {
	if strings.TrimSpace(command) == "" {
		return 1, fmt.Errorf("empty command")
	}
	cmd := shellCommand(command)
	if cmd == nil {
		return 1, fmt.Errorf("unsupported platform")
	}
	cmd.Dir = cwd
	cmd.Env = env // entorno mínimo: solo las DM_TEST_* (determinista)
	return runExitCode(cmd)
}

// shellCommand envuelve el comando para la plataforma actual.
func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

// runExitCode extrae el exit code sin propagar el error de salida.
func runExitCode(cmd *exec.Cmd) (int, error) {
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	if ee, ok := err.(*exec.ExitError); ok {
		return ee.ExitCode(), nil
	}
	return 1, err
}
