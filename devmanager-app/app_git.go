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
	"time"

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

// ---- Issue #63: diff, branches, tags (núcleo) ----

// projectPath devuelve la ruta del proyecto si el índice es válido.
func (a *App) projectPath(index int) string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return ""
	}
	return projects[index].Path
}

// GetGitDiff devuelve el diff del working tree del proyecto.
func (a *App) GetGitDiff(index int) []git.DiffFile {
	p := a.projectPath(index)
	if p == "" {
		return nil
	}
	return git.GetDiff(p)
}

// GitBranches lista ramas locales del proyecto.
func (a *App) GitBranches(index int) []git.Branch {
	p := a.projectPath(index)
	if p == "" {
		return nil
	}
	return git.ListBranches(p)
}

// GitCreateBranch crea una rama nueva desde HEAD.
func (a *App) GitCreateBranch(index int, name string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	return git.CreateBranch(p, strings.TrimSpace(name))
}

// GitRenameBranch renombra una rama local.
func (a *App) GitRenameBranch(index int, oldName, newName string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	return git.RenameBranch(p, oldName, strings.TrimSpace(newName))
}

// GitDeleteBranch borra una rama local (-d).
func (a *App) GitDeleteBranch(index int, name string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	return git.DeleteBranch(p, name)
}

// GitCheckout cambia a una rama o tag.
func (a *App) GitCheckout(index int, name string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	return git.CheckoutBranch(p, name)
}

// GitTags lista tags del proyecto.
func (a *App) GitTags(index int) []git.GitTag {
	p := a.projectPath(index)
	if p == "" {
		return nil
	}
	return git.ListTags(p)
}

// GitCreateTag crea una tag ligera en HEAD.
func (a *App) GitCreateTag(index int, name string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	return git.CreateTag(p, strings.TrimSpace(name))
}

// GitDeleteTag borra una tag local (y remota si existe origin).
func (a *App) GitDeleteTag(index int, name string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	return git.DeleteTag(p, name)
}

// GitPushTag empuja una tag a origin.
func (a *App) GitPushTag(index int, name string) error {
	p := a.projectPath(index)
	if p == "" {
		return fmt.Errorf("proyecto inválido")
	}
	code, _, stderr := git.RunGit(p, []string{"push", "origin", name}, 10*time.Second)
	if code != 0 {
		return fmt.Errorf("git push falló: %s", strings.TrimSpace(stderr))
	}
	return nil
}
