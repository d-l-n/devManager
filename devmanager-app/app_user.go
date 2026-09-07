package main

import (
	"fmt"
	"strings"

	"github.com/d-l-n/devmanager/internal/utils/deps"
	"github.com/d-l-n/devmanager/internal/utils/detection"
	"github.com/d-l-n/devmanager/internal/utils/evidence"
)

// ---- Create User bindings (feature) ----
// Crea un usuario en el proyecto gestionado ejecutando el comando configurado
// (user.command) con los datos via env vars DM_USER_*. Las env vars evitan la
// interpolación de shell (quoting frágil en cmd.exe con passwords que contienen
// & | ^ espacios, etc.). El proyecto decide su propio mecanismo: Firebase Admin
// SDK, REST API, seed de DB, lo que sea.
//
// La ejecución comparte el scripts.Manager del proyecto: salida en panel de
// Logs via eventos script:*, un solo subproceso activo, Stop por usuario.

const (
	envUserEmail    = "DM_USER_EMAIL"
	envUserName     = "DM_USER_NAME"
	envUserPassword = "DM_USER_PASSWORD"
	envUserRole     = "DM_USER_ROLE"

	defaultUserRole = "user"
)

// isProbablyEmail valida ligero: no vacío, contiene @, sin espacios/control.
func isProbablyEmail(s string) bool {
	return s != "" && strings.Contains(s, "@") && !strings.ContainsAny(s, " \t\r\n")
}

// CreateUser valida los datos y lanza user.command con DM_USER_* en el entorno.
// Devuelve []string con errores (paridad AddProject/UpdateProject); nil = OK.
// No devuelve el password: el frontend lo genera y lo muestra al usuario
// antes de enviarlo (la contraseña viaja solo en env, nunca en logs).
func (a *App) CreateUser(index int, email, name, password, role string) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("project index %d out of range", index)}
	}
	p := projects[index]

	email = strings.TrimSpace(email)
	if email == "" {
		return []string{"email is required"}
	}
	if !isProbablyEmail(email) {
		return []string{fmt.Sprintf("%q is not a valid email", email)}
	}
	if strings.TrimSpace(password) == "" {
		return []string{"password is required (use the Generate button or type one)"}
	}
	if !p.User.Enabled {
		return []string{"user creation disabled for this project (edit project → User)"}
	}
	if strings.TrimSpace(p.User.Command) == "" {
		return []string{"no create-user command configured (edit project → User)"}
	}

	env := map[string]string{
		envUserEmail:    email,
		envUserName:     strings.TrimSpace(name),
		envUserPassword: password,
		envUserRole:     strings.TrimSpace(role),
	}
	if env[envUserRole] == "" {
		env[envUserRole] = defaultUserRole
	}

	if _, _, scm := a.ensureManagers(index); scm != nil {
		scm.RunScriptWithEnv("Create User", p.User.Command, env)
	}
	return nil
}

// ---- Tab visibility (per-project feature flags) ----
// La UI oculta tabs que no aplican al proyecto seleccionado: Deps sin gestor de
// paquetes, Evidence sin artefactos de test-results. Playwright y Git se derivan
// del propio Project (playwright.enabled / repo), ya disponibles en frontend.

// ProjectFeatures describe qué señales de proyecto existen para decidir si la
// UI muestra o no los tabs Deps y Evidence.
type ProjectFeatures struct {
	HasPackageManager bool `json:"hasPackageManager"` // go.mod o package.json (no lockfile solo)
	HasEvidenceFiles  bool `json:"hasEvidenceFiles"`  // test-results con al menos un artefacto
}

// GetProjectFeatures devuelve señales de features por proyecto (para ocultar
// tabs del detail view que no aplican). Solo lee el filesystem: barato por selección.
func (a *App) GetProjectFeatures(index int) ProjectFeatures {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return ProjectFeatures{}
	}
	path := projects[index].Path

	f := ProjectFeatures{}
	if deps.DetectManager(path) != "" {
		f.HasPackageManager = true
	}
	if evidence.HasEvidence(path) {
		f.HasEvidenceFiles = true
	}
	return f
}

// DetectUserCommand autodetecta el comando de creación de usuario para una
// carpeta (heurística en internal/utils/detection). Devuelve "" si no hay
// señal. El dialog de proyecto la usa para prellenar user.command.
func (a *App) DetectUserCommand(path string) string {
	return detection.DetectUserCommand(path)
}