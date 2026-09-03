package main

import (
	"path/filepath"
	"testing"

	"github.com/d-l-n/devmanager/internal/config"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/wailsapp/wails/v2/pkg/options"
)

func TestSingleInstanceLockConfiguration(t *testing.T) {
	app := &App{}
	lock := singleInstanceLock(app)

	if lock == nil || lock.UniqueId == "" || lock.OnSecondInstanceLaunch == nil {
		t.Fatal("single-instance lock must define an identifier and second-launch handler")
	}

	lock.OnSecondInstanceLaunch(options.SecondInstanceData{})
	if app.windowHidden {
		t.Fatal("second launch must focus the existing instance")
	}
}

// ---- parseStrictBool (Issue #39) ----

func TestParseStrictBoolValid(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"false", false},
	}
	for _, tt := range tests {
		got, err := parseStrictBool(tt.input)
		if err != nil {
			t.Errorf("parseStrictBool(%q) unexpected error: %v", tt.input, err)
		}
		if got != tt.want {
			t.Errorf("parseStrictBool(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseStrictBoolInvalid(t *testing.T) {
	for _, input := range []string{"", "TRUE", "False", "1", "0", "yes", "no", "t", "f"} {
		_, err := parseStrictBool(input)
		if err == nil {
			t.Errorf("parseStrictBool(%q) should return error", input)
		}
	}
}

// ---- pathUnderProject (Issue #39) ----

func newTestManager(t *testing.T, paths []string) *config.Manager {
	t.Helper()
	p := filepath.Join(t.TempDir(), "projects.json")
	m, _ := config.NewManager(p, config.Options{})
	for _, path := range paths {
		proj, _ := models.ParseProject([]byte(`{"name":"test","path":"` + path + `"}`))
		m.AddProject(proj)
	}
	return m
}

func TestPathUnderProjectExactMatch(t *testing.T) {
	a := &App{}
	a.cfg = newTestManager(t, []string{"/projects/myapp"})
	if !a.pathUnderProject("/projects/myapp") {
		t.Error("exact project root should be under project")
	}
}

func TestPathUnderProjectSubdir(t *testing.T) {
	a := &App{}
	a.cfg = newTestManager(t, []string{"/projects/myapp"})
	if !a.pathUnderProject("/projects/myapp/src/index.js") {
		t.Error("subdirectory should be under project")
	}
}

func TestPathUnderProjectUnrelated(t *testing.T) {
	a := &App{}
	a.cfg = newTestManager(t, []string{"/projects/myapp"})
	if a.pathUnderProject("/other/path/file.txt") {
		t.Error("unrelated path should not be under project")
	}
}

func TestPathUnderProjectEmpty(t *testing.T) {
	a := &App{}
	a.cfg = newTestManager(t, []string{"/projects/myapp"})
	if a.pathUnderProject("") {
		t.Error("empty path should not be under project")
	}
	if a.pathUnderProject("  ") {
		t.Error("whitespace-only path should not be under project")
	}
}

func TestPathUnderProjectMultipleProjects(t *testing.T) {
	a := &App{}
	a.cfg = newTestManager(t, []string{"/projects/app-a", "/projects/app-b"})
	if !a.pathUnderProject("/projects/app-a/src/main.go") {
		t.Error("should match first project")
	}
	if !a.pathUnderProject("/projects/app-b/package.json") {
		t.Error("should match second project")
	}
	if a.pathUnderProject("/projects/app-c/index.js") {
		t.Error("should not match unregistered project")
	}
}

func TestPathUnderProjectNoProjects(t *testing.T) {
	a := &App{}
	a.cfg = newTestManager(t, nil)
	if a.pathUnderProject("/anything") {
		t.Error("no projects registered → nothing should match")
	}
}

// ---- ServerStatus (Issue #39) ----

func TestServerStatusZeroValue(t *testing.T) {
	ss := ServerStatus{}
	if ss.State != "" {
		t.Error("zero ServerStatus should have empty state")
	}
	if ss.Running {
		t.Error("zero ServerStatus should not be running")
	}
	if ss.UptimeSeconds != 0 {
		t.Error("zero ServerStatus should have 0 uptime")
	}
}
