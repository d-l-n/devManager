// Package git porta app/utils/git.py.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const createNoWindow = 0x08000000

// hideCmd evita flash de consola en Windows (paridad STARTF_USESHOWWINDOW|SW_HIDE).
func hideCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}

// HiddenCommand devuelve un exec.Cmd configurado para correr oculto
// (uso del App layer para Pull/Fetch/Stash con streaming).
func HiddenCommand(dir string, args []string) *exec.Cmd {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	hideCmd(cmd)
	return cmd
}

// IsRepo replica is_git_repo: existe <path>/.git (directorio o archivo worktree).
func IsRepo(projectPath string) bool {
	if projectPath == "" {
		return false
	}
	if fi, err := os.Stat(projectPath); err != nil || !fi.IsDir() {
		return false
	}
	_, err := os.Stat(filepath.Join(projectPath, ".git"))
	return err == nil
}

// GetGitInfo replica get_git_info.
func GetGitInfo(projectPath string) Status {
	var result Status
	if !IsRepo(projectPath) {
		return result
	}
	result.IsRepo = true

	// 1. Lectura r├ípida de .git/HEAD.
	headContent, err := os.ReadFile(filepath.Join(projectPath, ".git", "HEAD"))
	if err == nil {
		content := strings.TrimSpace(string(headContent))
		const prefix = "ref: refs/heads/"
		switch {
		case strings.HasPrefix(content, prefix):
			result.Branch = content[len(prefix):]
		case len(content) >= 7:
			result.Branch = content[:7] // Detached HEAD SHA
		}
	}

	// 2. Fallback y dirty status v├¡a CLI.
	if result.Branch == "" {
		if code, out, _ := RunGit(projectPath, []string{"rev-parse", "--abbrev-ref", "HEAD"}, 1500*time.Millisecond); code == 0 {
			result.Branch = strings.TrimSpace(out)
		}
	}
	if code, out, _ := RunGit(projectPath, []string{"status", "--porcelain"}, 2*time.Second); code == 0 {
		result.IsDirty = strings.TrimSpace(out) != ""
	}
	return result
}

// RunGit replica run_git: ejecuta git oculto con timeout.
// Devuelve (returncode, stdout, stderr).
func RunGit(projectPath string, args []string, timeout time.Duration) (int, string, string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = projectPath
	hideCmd(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if ctx.Err() == context.DeadlineExceeded {
		return -1, "", fmt.Sprintf("git command timed out after %v", timeout)
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode(), stdout.String(), stderr.String()
	}
	if errors.Is(err, exec.ErrNotFound) {
		return -1, "", "git executable not found in PATH"
	}
	return -1, "", err.Error()
}

// LastCommit replica last_commit de get_git_status_full.
type LastCommit struct {
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	DateRel string `json:"dateRel"`
}

// Status es el DTO que consume el frontend. Campos camelCase (convenci├│n
// app-level); models.Project conserva snake_case porque es schema de archivo.
type Status struct {
	IsRepo      bool        `json:"isRepo"`
	Branch      string      `json:"branch"`
	IsDirty     bool        `json:"isDirty"`
	Error       string      `json:"error,omitempty"`
	Ahead       int         `json:"ahead"`
	Behind      int         `json:"behind"`
	HasUpstream bool        `json:"hasUpstream"`
	LastCommit  *LastCommit `json:"lastCommit"`
}

// GetStatusFull replica get_git_status_full: extiende GetGitInfo con
// ahead/behind y metadata del ├║ltimo commit.
func GetStatusFull(projectPath string) Status {
	result := GetGitInfo(projectPath)
	result.Ahead = 0
	result.Behind = 0
	if !result.IsRepo {
		return result
	}

	code, out, _ := RunGit(projectPath,
		[]string{"rev-list", "--left-right", "--count", "@{upstream}...HEAD"},
		5*time.Second)
	if code == 0 && strings.Contains(out, "\t") {
		parts := strings.SplitN(strings.TrimSpace(out), "\t", 2)
		if len(parts) == 2 {
			left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			result.HasUpstream = true
			if n, err := strconv.Atoi(left); err == nil {
				result.Behind = n
			}
			if n, err := strconv.Atoi(right); err == nil {
				result.Ahead = n
			}
		}
	}

	code, out, _ = RunGit(projectPath,
		[]string{"log", "-1", "--pretty=format:%h|%s|%cr"},
		5*time.Second)
	if code == 0 && strings.TrimSpace(out) != "" {
		parts := strings.SplitN(out, "|", 3)
		if len(parts) == 3 {
			result.LastCommit = &LastCommit{
				Hash:    strings.TrimSpace(parts[0]),
				Subject: strings.TrimSpace(parts[1]),
				DateRel: strings.TrimSpace(parts[2]),
			}
		}
	}
	return result
}
