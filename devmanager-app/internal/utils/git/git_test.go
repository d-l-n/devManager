package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// gexec corre un comando git de setup (init/commit/etc.) y falla el test si da error.
func gexec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v en %s: %v\n%s", args, dir, err, out)
	}
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// initRepo crea repo con 1 commit y devuelve su ruta.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gexec(t, dir, "init")
	gexec(t, dir, "config", "user.email", "test@test.dev")
	gexec(t, dir, "config", "user.name", "tester")
	write(t, filepath.Join(dir, "a.txt"), "hello\n")
	gexec(t, dir, "add", ".")
	gexec(t, dir, "commit", "-m", "first")
	return dir
}

func TestIsRepo(t *testing.T) {
	repo := initRepo(t)
	if !IsRepo(repo) {
		t.Error("repo con .git debe ser repo")
	}
	plain := t.TempDir()
	if IsRepo(plain) {
		t.Error("carpeta sin .git no debe ser repo")
	}
	if IsRepo(filepath.Join(plain, "no-existe")) {
		t.Error("ruta inexistente no debe ser repo")
	}
	if IsRepo("") {
		t.Error("ruta vacía no debe ser repo")
	}
}

func TestGetGitInfoCleanAndDirty(t *testing.T) {
	repo := initRepo(t)
	info := GetGitInfo(repo)
	if !info.IsRepo {
		t.Fatal("is_repo esperado true")
	}
	if info.Branch == "" {
		t.Error("branch debe leerse de .git/HEAD")
	}
	if info.IsDirty {
		t.Error("repo recién commiteado debe estar clean")
	}
	write(t, filepath.Join(repo, "a.txt"), "cambiado\n")
	info = GetGitInfo(repo)
	if !info.IsDirty {
		t.Error("cambio sin commitear debe reportarse dirty")
	}
}

func TestGetGitInfoDetachedHead(t *testing.T) {
	repo := initRepo(t)
	gexec(t, repo, "checkout", "--detach", "HEAD")
	info := GetGitInfo(repo)
	// HEAD contiene SHA: branch = primeros 7 chars.
	if len(info.Branch) != 7 {
		t.Errorf("detached HEAD debe dar sha de 7 chars, got %q", info.Branch)
	}
}

func TestGetGitInfoNotARepo(t *testing.T) {
	s := GetGitInfo(t.TempDir())
	if s.IsRepo || s.Branch != "" || s.IsDirty {
		t.Errorf("no-repo debe dar ceros: %+v", s)
	}
}

func TestGetStatusFullWithUpstream(t *testing.T) {
	root := t.TempDir()
	bare := filepath.Join(root, "origin.git")
	gexec(t, root, "init", "--bare", bare)

	work := filepath.Join(root, "work")
	gexec(t, root, "clone", bare, work)
	gexec(t, work, "config", "user.email", "test@test.dev")
	gexec(t, work, "config", "user.name", "tester")
	write(t, filepath.Join(work, "f.txt"), "1\n")
	gexec(t, work, "add", ".")
	gexec(t, work, "commit", "-m", "first")
	gexec(t, work, "push", "-u", "origin", "HEAD")

	st := GetStatusFull(work)
	if !st.HasUpstream {
		t.Fatalf("upstream esperado tras push -u: %+v", st)
	}
	if st.Ahead != 0 || st.Behind != 0 {
		t.Errorf("ahead/behind iniciales = %d/%d, want 0/0", st.Ahead, st.Behind)
	}
	if st.LastCommit == nil || st.LastCommit.Subject != "first" || st.LastCommit.Hash == "" || st.LastCommit.DateRel == "" {
		t.Errorf("last_commit mal parseado: %+v", st.LastCommit)
	}

	// Commit local sin push → ahead=1.
	write(t, filepath.Join(work, "f.txt"), "2\n")
	gexec(t, work, "add", ".")
	gexec(t, work, "commit", "-m", "second")
	st = GetStatusFull(work)
	if st.Ahead != 1 || st.Behind != 0 {
		t.Errorf("ahead/behind tras commit local = %d/%d, want 1/0", st.Ahead, st.Behind)
	}
}

func TestGetStatusFullNoUpstream(t *testing.T) {
	repo := initRepo(t)
	st := GetStatusFull(repo)
	if st.HasUpstream {
		t.Errorf("sin remote no debe haber upstream: %+v", st)
	}
	if st.Ahead != 0 || st.Behind != 0 {
		t.Errorf("ahead/behind default = %d/%d, want 0/0", st.Ahead, st.Behind)
	}
}

func TestRunGitSuccessAndFailure(t *testing.T) {
	repo := initRepo(t)
	code, out, errStr := RunGit(repo, []string{"status", "--porcelain"}, 5*time.Second)
	if code != 0 || errStr != "" {
		t.Errorf("git status limpio: code=%d err=%q out=%q", code, errStr, out)
	}

	plain := t.TempDir()
	code, _, stderr := RunGit(plain, []string{"status"}, 5*time.Second)
	if code == 0 {
		t.Error("git status fuera de repo debe fallar")
	}
	if strings.TrimSpace(stderr) == "" {
		t.Error("stderr debe contener mensaje de error de git")
	}
}
