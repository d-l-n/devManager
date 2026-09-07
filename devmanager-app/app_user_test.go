package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/config"
	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/playwright"
	"github.com/d-l-n/devmanager/internal/scripts"
	"github.com/d-l-n/devmanager/internal/server"
	"github.com/d-l-n/devmanager/internal/testutil"
)

// newUserTestApp crea un App con cfg apuntando a un proyecto temp con el
// comando por defecto. Las tests que necesitan otra UserConfig la reemplazan.
func newUserTestApp(t *testing.T) *App {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "projects.json")
	cfg, err := config.NewManager(cfgPath, config.Options{})
	if err != nil {
		t.Fatalf("config manager: %v", err)
	}
	p := models.Project{
		Name: "t",
		Path: t.TempDir(),
		User: models.UserConfig{Enabled: true, Command: "cmd /c echo create-user"},
	}
	if err := cfg.AddProject(p); err != nil {
		t.Fatalf("add project: %v", err)
	}
	return &App{
		cfg:                cfg,
		servers:            map[int]*server.Manager{},
		playwrightManagers: map[int]*playwright.Manager{},
		scriptManagers:     map[int]*scripts.Manager{},
	}
}

func TestCreateUserValidation(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
		role     string
		user     models.UserConfig
		wantErr  string
	}{
		{"email vacío", "", "secreto", "user", models.UserConfig{Enabled: true, Command: "x"}, "email is required"},
		{"email sin @", "ana", "secreto", "user", models.UserConfig{Enabled: true, Command: "x"}, "not a valid email"},
		{"email con espacio", "a b@x.com", "secreto", "user", models.UserConfig{Enabled: true, Command: "x"}, "not a valid email"},
		{"password vacío", "ana@x.com", "  ", "user", models.UserConfig{Enabled: true, Command: "x"}, "password is required"},
		{"user disabled", "ana@x.com", "secreto", "user", models.UserConfig{Enabled: false, Command: "x"}, "user creation disabled"},
		{"sin command", "ana@x.com", "secreto", "user", models.UserConfig{Enabled: true, Command: " "}, "no create-user command configured"},
	}
	for _, tt := range tests {
		a := newUserTestApp(t)
		// Reemplaza la UserConfig del proyecto 0.
		proj := a.cfg.Projects()[0]
		proj.User = tt.user
		_ = a.cfg.UpdateProject(0, proj)

		errs := a.CreateUser(0, tt.email, "Nombre", tt.password, tt.role)
		if len(errs) == 0 {
			t.Errorf("%s: esperaba error, got nil", tt.name)
			continue
		}
		if !containsSubstr(errs, tt.wantErr) {
			t.Errorf("%s: error = %v, want contiene %q", tt.name, errs, tt.wantErr)
		}
		if len(errs) > 1 {
			t.Errorf("%s: esperaba 1 error, got %v", tt.name, errs)
		}
	}
}

func TestCreateUserIndexOutOfRange(t *testing.T) {
	a := newUserTestApp(t)
	errs := a.CreateUser(5, "ana@x.com", "", "secreto", "user")
	if len(errs) == 0 || !strings.Contains(errs[0], "out of range") {
		t.Errorf("esperaba out of range, got %v", errs)
	}
}

// TestCreateUserRunsCommand: happy path. Pre-sembra el script manager (los
// callbacks de ensureManagers emiten a ctx Wails que en tests es nil y
// log.Fatalf). El manager inyectado captura la salida sin tocar wails.
func TestCreateUserRunsCommand(t *testing.T) {
	a := newUserTestApp(t)
	proj := a.cfg.Projects()[0]
	proj.User = models.UserConfig{Enabled: true, Command: testutil.EchoEnvCmdStr("DM_USER_EMAIL")}
	_ = a.cfg.UpdateProject(0, proj)

	var mu sync.Mutex
	var logs []string
	done := make(chan struct{})
	scm := scripts.NewManager(proj, scripts.Callbacks{
		OnLog: func(msg string, isErr bool) { mu.Lock(); logs = append(logs, msg); mu.Unlock() },
		OnScriptFinished: func(name string, code int) { close(done) },
	})
	a.scriptManagers = map[int]*scripts.Manager{0: scm}

	errs := a.CreateUser(0, "ana@example.com", "Ana", "p4ss w0rd!&|", "admin")
	if len(errs) != 0 {
		t.Fatalf("CreateUser errores: %v", errs)
	}

	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("el comando de creación no terminó en 8s")
	}

	mu.Lock()
	defer mu.Unlock()
	joined := strings.Join(logs, "\n")
	// La password con caracteres de shell no debe romper la ejecución (env var).
	if !strings.Contains(joined, "[Create User] DM_USER_EMAIL=ana@example.com") {
		t.Errorf("env DM_USER_EMAIL no aplicada. logs: %s", joined)
	}
	// El comando logueado NO debe contener la password (viaja solo en env).
	for _, l := range logs {
		if strings.Contains(l, "p4ss w0rd!&|") {
			t.Errorf("password filtrada a logs: %q", l)
		}
	}
}

func containsSubstr(errs []string, sub string) bool {
	for _, e := range errs {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// ---- GetProjectFeatures (tab visibility) ----

func TestGetProjectFeatures(t *testing.T) {
	tests := []struct {
		name        string
		prepare     func(path string) string // devuelve el path del proyecto
		wantPM      bool
		wantEvidenc bool
	}{
		{
			name:    "proyecto vacío sin features",
			prepare: func(path string) string { return path },
		},
		{
			name: "con package.json",
			prepare: func(path string) string {
				writeFile(t, filepath.Join(path, "package.json"), `{"name":"x","dependencies":{"y":"1.0.0"}}`)
				return path
			},
			wantPM: true,
		},
		{
			name: "con go.mod",
			prepare: func(path string) string {
				writeFile(t, filepath.Join(path, "go.mod"), "module x\n")
				return path
			},
			wantPM: true,
		},
		{
			name: "con evidencias en test-results",
			prepare: func(path string) string {
				writeFile(t, filepath.Join(path, "test-results", "a", "shot.png"), "x")
				return path
			},
			wantEvidenc: true,
		},
		{
			name: "test-results vacío no cuenta como evidencia",
			prepare: func(path string) string {
				_ = os.MkdirAll(filepath.Join(path, "test-results"), 0o755)
				return path
			},
		},
		{
			name: "package.json + evidencias juntos",
			prepare: func(path string) string {
				writeFile(t, filepath.Join(path, "package.json"), `{}`)
				writeFile(t, filepath.Join(path, "test-results", "t.zip"), "x")
				return path
			},
			wantPM:      true,
			wantEvidenc: true,
		},
	}
	for _, tt := range tests {
		projPath := tt.prepare(t.TempDir())
		a := newUserTestApp(t)
		proj := a.cfg.Projects()[0]
		proj.Path = projPath
		if err := a.cfg.UpdateProject(0, proj); err != nil {
			t.Fatalf("%s: update: %v", tt.name, err)
		}
		got := a.GetProjectFeatures(0)
		if got.HasPackageManager != tt.wantPM {
			t.Errorf("%s: HasPackageManager = %v, want %v", tt.name, got.HasPackageManager, tt.wantPM)
		}
		if got.HasEvidenceFiles != tt.wantEvidenc {
			t.Errorf("%s: HasEvidenceFiles = %v, want %v", tt.name, got.HasEvidenceFiles, tt.wantEvidenc)
		}
	}
}

func TestGetProjectFeaturesIndexOutOfRange(t *testing.T) {
	a := newUserTestApp(t)
	got := a.GetProjectFeatures(9)
	if got.HasPackageManager || got.HasEvidenceFiles {
		t.Errorf("out of range debe dar features vacías, got %+v", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}