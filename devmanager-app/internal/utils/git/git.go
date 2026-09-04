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
	"time"
)

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

	// 1. Lectura rápida de .git/HEAD.
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

	// 2. Fallback y dirty status vía CLI.
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

// Status es el DTO que consume el frontend. Campos camelCase (convención
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

// DiffLine es una línea de un diff, clasificada para resaltado.
type DiffLine struct {
	Kind string `json:"kind"` // ctx | add | del | hunk
	Text string `json:"text"`
}

// DiffFile agrupa líneas de diff de un archivo.
type DiffFile struct {
	Path  string     `json:"path"`
	Lines []DiffLine `json:"lines"`
}

// GetDiff devuelve el diff del working tree (git diff, sin --cached).
// Vacío si no es repo o no hay cambios.
func GetDiff(projectPath string) []DiffFile {
	if !IsRepo(projectPath) {
		return nil
	}
	code, out, _ := RunGit(projectPath, []string{"diff", "--no-color"}, 5*time.Second)
	if code != 0 {
		return nil
	}
	return parseDiff(out)
}

// parseDiff convierte salida de git diff a []DiffFile agrupando por archivo.
func parseDiff(raw string) []DiffFile {
	var files []DiffFile
	var cur *DiffFile
	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			if cur != nil {
				files = append(files, *cur)
			}
			name := strings.TrimPrefix(line, "diff --git ")
			name = strings.TrimPrefix(name, "a/")
			if i := strings.Index(name, " b/"); i >= 0 {
				name = name[:i]
			}
			cur = &DiffFile{Path: name}
		case cur == nil:
			continue
		case strings.HasPrefix(line, "@@"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: "hunk", Text: line})
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
			// marcadores de cabecera de archivo; se omiten como contexto
			cur.Lines = append(cur.Lines, DiffLine{Kind: "hunk", Text: line})
		case strings.HasPrefix(line, "+"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: "add", Text: line})
		case strings.HasPrefix(line, "-"):
			cur.Lines = append(cur.Lines, DiffLine{Kind: "del", Text: line})
		default:
			cur.Lines = append(cur.Lines, DiffLine{Kind: "ctx", Text: line})
		}
	}
	if cur != nil {
		files = append(files, *cur)
	}
	return files
}

// Branch es una rama local del repo.
type Branch struct {
	Name    string `json:"name"`
	Current bool   `json:"current"`
}

// ListBranches lista ramas locales; la actual marcada. Vacío si no es repo.
func ListBranches(projectPath string) []Branch {
	if !IsRepo(projectPath) {
		return nil
	}
	code, out, _ := RunGit(projectPath, []string{"branch", "--no-color"}, 3*time.Second)
	if code != 0 {
		return nil
	}
	var branches []Branch
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		cur := strings.HasPrefix(line, "*")
		name := strings.TrimSpace(strings.TrimPrefix(line, "*"))
		// quita estados tipo " (detached)"
		if i := strings.Index(name, " "); i >= 0 {
			name = name[:i]
		}
		branches = append(branches, Branch{Name: name, Current: cur})
	}
	return branches
}

// CreateBranch crea una rama nueva a partir de HEAD.
func CreateBranch(projectPath, name string) error {
	code, _, stderr := RunGit(projectPath, []string{"branch", name}, 3*time.Second)
	if code != 0 {
		return errors.New(strings.TrimSpace(stderr))
	}
	return nil
}

// RenameBranch renombra una rama local.
func RenameBranch(projectPath, oldName, newName string) error {
	code, _, stderr := RunGit(projectPath, []string{"branch", "-m", oldName, newName}, 3*time.Second)
	if code != 0 {
		return errors.New(strings.TrimSpace(stderr))
	}
	return nil
}

// DeleteBranch borra una rama local (-d, falla si no está mergeada).
func DeleteBranch(projectPath, name string) error {
	code, _, stderr := RunGit(projectPath, []string{"branch", "-d", name}, 3*time.Second)
	if code != 0 {
		return errors.New(strings.TrimSpace(stderr))
	}
	return nil
}

// CheckoutBranch cambia a una rama o tag existente.
func CheckoutBranch(projectPath, name string) error {
	code, _, stderr := RunGit(projectPath, []string{"checkout", name}, 5*time.Second)
	if code != 0 {
		return errors.New(strings.TrimSpace(stderr))
	}
	return nil
}

// GitTag es una tag con metadata del commit al que apunta.
type GitTag struct {
	Name    string `json:"name"`
	Hash    string `json:"hash"`
	Subject string `json:"subject"`
	DateRel string `json:"dateRel"`
}

// ListTags lista tags ordenadas (--sort=-creatordate). Vacío si no es repo.
func ListTags(projectPath string) []GitTag {
	if !IsRepo(projectPath) {
		return nil
	}
	code, out, _ := RunGit(projectPath, []string{
		"for-each-ref", "refs/tags",
		"--sort=-creatordate",
		"--format=%(refname:short)|%(objectname:short)|%(subject)|%(creatordate:relative)",
	}, 3*time.Second)
	if code != 0 {
		return nil
	}
	var tags []GitTag
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(strings.TrimRight(line, "\r"), "|", 4)
		if len(parts) != 4 {
			continue
		}
		tags = append(tags, GitTag{
			Name:    strings.TrimSpace(parts[0]),
			Hash:    strings.TrimSpace(parts[1]),
			Subject: strings.TrimSpace(parts[2]),
			DateRel: strings.TrimSpace(parts[3]),
		})
	}
	return tags
}

// CreateTag crea una tag ligera apuntando a HEAD.
func CreateTag(projectPath, name string) error {
	code, _, stderr := RunGit(projectPath, []string{"tag", name}, 3*time.Second)
	if code != 0 {
		return errors.New(strings.TrimSpace(stderr))
	}
	return nil
}

// DeleteTag borra una tag local; si existe en origin la borra también.
func DeleteTag(projectPath, name string) error {
	code, _, stderr := RunGit(projectPath, []string{"tag", "-d", name}, 3*time.Second)
	if code != 0 {
		return errors.New(strings.TrimSpace(stderr))
	}
	// intento de borrado remoto; no falla en local si no hay remote
	RunGit(projectPath, []string{"push", "origin", ":refs/tags/" + name}, 3*time.Second)
	return nil
}

// GetStatusFull replica get_git_status_full: extiende GetGitInfo con
// ahead/behind y metadata del último commit.
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
