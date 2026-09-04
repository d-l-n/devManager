package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// gexec corre un comando git de setup (init/commit/etc.) y falla el test si da error.
func gexec(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	hideCmd(cmd)
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

func TestGetDiffEmptyOnCleanRepo(t *testing.T) {
	repo := initRepo(t)
	diffs := GetDiff(repo)
	if len(diffs) != 0 {
		t.Errorf("repo limpio debe dar diff vacío, got %d files", len(diffs))
	}
}

func TestGetDiffDetectsChanges(t *testing.T) {
	repo := initRepo(t)
	write(t, filepath.Join(repo, "a.txt"), "bye\nworld\n")
	diffs := GetDiff(repo)
	if len(diffs) != 1 {
		t.Fatalf("esperado 1 archivo con cambios, got %d", len(diffs))
	}
	f := diffs[0]
	if !strings.Contains(f.Path, "a.txt") {
		t.Errorf("path esperado a.txt, got %q", f.Path)
	}
	var hasAdd, hasDel bool
	for _, l := range f.Lines {
		switch l.Kind {
		case "add":
			hasAdd = true
		case "del":
			hasDel = true
		}
	}
	if !hasAdd || !hasDel {
		t.Errorf("diff debe tener al menos una línea add y del: add=%v del=%v\n%+v", hasAdd, hasDel, f.Lines)
	}
}

func TestGetDiffNotARepo(t *testing.T) {
	if d := GetDiff(t.TempDir()); d != nil {
		t.Errorf("no-repo debe dar nil, got %+v", d)
	}
}

func TestListBranchesAndCreate(t *testing.T) {
	repo := initRepo(t)
	gexec(t, repo, "checkout", "-b", "feature/x")
	branches := ListBranches(repo)
	if len(branches) < 2 {
		t.Fatalf("esperado al menos master+feature, got %+v", branches)
	}
	var found, current bool
	for _, b := range branches {
		if b.Name == "feature/x" {
			found = true
		}
		if b.Name == "feature/x" && b.Current {
			current = true
		}
	}
	if !found || !current {
		t.Errorf("feature/x debe existir y ser current: found=%v current=%v", found, current)
	}
}

func TestRenameAndDeleteBranch(t *testing.T) {
	repo := initRepo(t)
	if err := CreateBranch(repo, "temp"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := RenameBranch(repo, "temp", "temp2"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if err := CheckoutBranch(repo, "temp2"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := DeleteBranch(repo, "master"); err != nil {
		t.Fatalf("delete master: %v", err)
	}
	for _, b := range ListBranches(repo) {
		if b.Name == "master" {
			t.Error("master no debe existir tras delete")
		}
	}
}

func TestTagsLifecycle(t *testing.T) {
	repo := initRepo(t)
	if err := CreateTag(repo, "v1.0.0"); err != nil {
		t.Fatalf("create tag: %v", err)
	}
	tags := ListTags(repo)
	if len(tags) != 1 || tags[0].Name != "v1.0.0" || tags[0].Hash == "" {
		t.Fatalf("esperado tag v1.0.0 con hash, got %+v", tags)
	}
	if err := DeleteTag(repo, "v1.0.0"); err != nil {
		t.Fatalf("delete tag: %v", err)
	}
	if len(ListTags(repo)) != 0 {
		t.Error("tags deben quedar vacías tras delete")
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
