package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/utils/git"
)

// ---- Git bindings ----

func (a *App) GetGitStatus(index int) git.Status {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return git.Status{}
	}
	return git.GetStatusFull(projects[index].Path)
}

var gitActions = map[string][]string{
	"Pull":  {"pull", "--ff-only"},
	"Fetch": {"fetch", "--all", "--prune"},
	"Stash": {"stash"},
}

// GitAction corre Pull/Fetch/Stash asíncrono con streaming de salida.
// Paridad GitPanel._run_command/_on_finished: un solo comando git a la vez
// POR proyecto; si ese proyecto ya tiene uno en curso se ignora la petición.
func (a *App) GitAction(index int, action string) {
	args, ok := gitActions[action]
	if !ok {
		return
	}
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return
	}
	path := projects[index].Path
	projectName := projects[index].Name

	a.mu.Lock()
	if a.gitBusy[index] {
		a.mu.Unlock()
		return // paridad: self._process is not None → ignorar
	}
	a.gitBusy[index] = true
	a.mu.Unlock()

	go func() {
		code, stashClean := a.runGitStreaming(index, action, path, args)

		// Paridad: "[git pull] exited with code N" va al Logs tab.
		wails.EventsEmit(a.ctx, "git:output", map[string]interface{}{
			"index":   index,
			"text":    "[git " + strings.ToLower(action) + "] exited with code " + strconv.Itoa(code),
			"isError": code != 0,
		})
		wails.EventsEmit(a.ctx, "git:finished", map[string]interface{}{
			"index": index, "name": action, "exitCode": code, "cleanStash": stashClean,
		})
		if code != 0 {
			a.emitNotify(projectName, fmt.Sprintf("Git %s failed (exit %d)", action, code), "error")
		}

		a.mu.Lock()
		a.gitBusy[index] = false
		a.mu.Unlock()
	}()
}

// runGitStreaming corre git y streamea stdout/stderr como eventos.
// Devuelve (exitCode, stashClean) donde stashClean replica la detección
// de "No local changes" del panel Qt para el strip de resultado.
func (a *App) runGitStreaming(index int, action, path string, args []string) (int, bool) {
	cmd := git.HiddenCommand(path, args)
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return -1, false
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return -1, false
	}
	if err := cmd.Start(); err != nil {
		wails.EventsEmit(a.ctx, "git:output", map[string]interface{}{
			"index": index, "text": err.Error(), "isError": true,
		})
		return -1, false
	}

	var stdoutBuf bytes.Buffer
	stream := func(f interface{ Read([]byte) (int, error) }, isError bool, buf *bytes.Buffer) {
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			buf.WriteString(line + "\n")
			wails.EventsEmit(a.ctx, "git:output", map[string]interface{}{
				"index": index, "text": line, "isError": isError,
			})
		}
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); stream(stdoutPipe, false, &stdoutBuf) }()
	go func() { defer wg.Done(); stream(stderrPipe, true, &bytes.Buffer{}) }()
	wg.Wait()

	code := 0
	if err := cmd.Wait(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	cleanStash := action == "Stash" && code == 0 && strings.Contains(stdoutBuf.String(), "No local changes")
	return code, cleanStash
}
