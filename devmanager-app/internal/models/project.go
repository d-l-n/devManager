// Package models porta app/models/project.py.
package models

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/d-l-n/devmanager/internal/env"
)

// ServerState replica ServerState de project.py como strings serializables
// al frontend vía EventsEmit.
type ServerState string

const (
	StateStopped  ServerState = "stopped"
	StateStarting ServerState = "starting"
	StateRunning  ServerState = "running"
	StateStopping ServerState = "stopping"
	StateError    ServerState = "error"
)

type ServerConfig struct {
	Enabled        bool   `json:"enabled"`
	Command        string `json:"command"`
	Port           int    `json:"port"`
	URL            string `json:"url"`
	StartupTimeout int    `json:"startup_timeout"`
}

type PlaywrightConfig struct {
	Enabled       bool   `json:"enabled"`
	Command       string `json:"command"`
	UICommand     string `json:"ui_command"`
	DebugCommand  string `json:"debug_command"`
	ReportCommand string `json:"report_command"`
}

// UserConfig define el comando de creación de usuarios del proyecto gestionado.
// Se ejecuta con los datos del usuario en env vars DM_USER_* (sin interpolación
// de shell); el propio proyecto decide su stack (Firebase, REST, seed DB, ...).
type UserConfig struct {
	Enabled bool   `json:"enabled"`
	Command string `json:"command"`
}

// KnownTabs son los ids de tabs del detail view (paridad con index.html).
// Logs no se puede ocultar: es el fallback del tab activo.
var KnownTabs = []string{"logs", "scripts", "git", "deps", "playwright", "evidence", "obscura", "backlog", "workflows"}

// TabsConfig personaliza por proyecto el detail view: tabs ocultos y orden
// de aparición. Vacío = comportamiento default (todos visibles, orden del DOM).
type TabsConfig struct {
	Hidden []string `json:"hidden"`
	Order  []string `json:"order"`
}

type BacklogItem struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      string `json:"status"`   // todo, in-progress, done
	Priority    string `json:"priority"` // low, medium, high
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// EnvConfig agrupa la config conmutable por entorno (Fase 1 #67).
// Fase 2: Vars son variables dotenv del entorno; EnvFile es el archivo
// dotenv relativo al proyecto (solo el nombre, nunca valores, va a logs).
// Fase 3: Secrets lista keys de Vars cuyo valor se enmascara al leer.
type EnvConfig struct {
	Server     ServerConfig      `json:"server"`
	Playwright PlaywrightConfig  `json:"playwright"`
	User       UserConfig        `json:"user"`
	Vars       map[string]string `json:"vars"`
	EnvFile    string            `json:"env_file"`
	Secrets    []string          `json:"secrets"`
}

// MaskedValue reemplaza valores secretos al leer (nunca viaja el real
// salvo reveal explícito).
const MaskedValue = "***"

// IsSecret reporta si key está en la lista de secretos.
func IsSecret(secrets []string, key string) bool {
	for _, s := range secrets {
		if s == key {
			return true
		}
	}
	return false
}

// MaskedVars devuelve copia de vars con secretos enmascarados.
// Nunca loguear su salida más allá de conteo/keys.
func MaskedVars(vars map[string]string, secrets []string) map[string]string {
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		if IsSecret(secrets, k) {
			out[k] = MaskedValue
		} else {
			out[k] = v
		}
	}
	return out
}

// DefaultEnvFile devuelve el archivo dotenv por defecto del entorno:
// dev=.env, staging=.env.staging, prod=.env.prod, custom=.env.<name>.
func DefaultEnvFile(name string) string {
	switch name {
	case "dev":
		return ".env"
	case "staging":
		return ".env.staging"
	case "prod":
		return ".env.prod"
	default:
		return ".env." + name
	}
}

type Project struct {
	Name       string              `json:"name"`
	Path       string              `json:"path"`
	Server     ServerConfig        `json:"server"`
	Playwright PlaywrightConfig    `json:"playwright"`
	User       UserConfig          `json:"user"`
	Tabs       TabsConfig          `json:"tabs"`
	Pinned     bool                `json:"pinned"`
	Backlog    []BacklogItem       `json:"backlog"`
	ActiveEnv  string              `json:"active_env"`
	Envs       map[string]EnvConfig `json:"envs"`
	// Workflows (Issue #65): automatizaciones por proyecto. Ausente → vacío
	// (legacy configs sin estos campos siguen cargando).
	Workflows []Workflow      `json:"workflows"`
	Webhooks  []WebhookConfig `json:"webhooks"`
	CICD      CIConfig        `json:"cicd"`
}

// ---- Workflows / Webhooks / CI-CD (Issue #65, MVP) ----

// Triggers de workflow.
const (
	WorkflowTriggerManual   = "manual"
	WorkflowTriggerSchedule = "schedule"
	WorkflowTriggerEvent    = "event"
)

// Eventos que disparan workflows trigger=event.
const (
	WorkflowEventServerStarted   = "server_started"
	WorkflowEventServerStopped   = "server_stopped"
	WorkflowEventTestsFinished   = "tests_finished"
	WorkflowEventWebhookReceived = "webhook_received"
)

// Kinds de step.
const (
	WorkflowStepCommand   = "command"
	WorkflowStepNotify    = "notify"
	WorkflowStepWebhook   = "webhook"
	WorkflowStepCITrigger = "ci_trigger"
)

// WorkflowStep es un paso secuencial del workflow.
type WorkflowStep struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Command    string            `json:"command,omitempty"`
	URL        string            `json:"url,omitempty"`
	Method     string            `json:"method,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	CITarget   string            `json:"ci_target,omitempty"`
	Retry      int               `json:"retry,omitempty"`
	TimeoutSec int               `json:"timeout_sec,omitempty"`
}

// Workflow es una automatización por proyecto.
type Workflow struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Enabled  bool           `json:"enabled"`
	Trigger  string         `json:"trigger"`
	Schedule string         `json:"schedule,omitempty"`
	Event    string         `json:"event,omitempty"`
	Steps    []WorkflowStep `json:"steps"`
}

// WebhookConfig es un webhook saliente por proyecto: ante cada evento
// listado se POSTea JSON {event, project, at, payload} a URL.
type WebhookConfig struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
}

// CIConfig agrupa la config CI/CD por proyecto. Los tokens NUNCA van en
// claro: TokenRef es `env:NAME` (variable de entorno) o vacío.
type CIConfig struct {
	GitHub  GitHubCIConfig  `json:"github"`
	GitLab  GitLabCIConfig  `json:"gitlab"`
	Jenkins JenkinsCIConfig `json:"jenkins"`
}

type GitHubCIConfig struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	WorkflowFile string `json:"workflow_file"`
	Ref          string `json:"ref"`
	TokenRef     string `json:"token_ref"`
	APIBase      string `json:"api_base,omitempty"`
}

type GitLabCIConfig struct {
	BaseURL   string `json:"base_url"`
	ProjectID string `json:"project_id"`
	Ref       string `json:"ref"`
	TokenRef  string `json:"token_ref"`
}

type JenkinsCIConfig struct {
	BaseURL  string `json:"base_url"`
	Job      string `json:"job"`
	TokenRef string `json:"token_ref"`
}

// envNameRe valida nombres de entorno: ^[a-z0-9_-]{1,32}$.
var envNameRe = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)

// IsValidEnvName reporta si un nombre de entorno es válido.
func IsValidEnvName(name string) bool {
	return envNameRe.MatchString(name)
}

// EffectiveEnv devuelve la config del entorno activo (fallback: top-level).
func (p Project) EffectiveEnv() EnvConfig {
	if e, ok := p.Envs[p.ActiveEnv]; ok {
		return e
	}
	return EnvConfig{Server: p.Server, Playwright: p.Playwright, User: p.User, Vars: map[string]string{}}
}

// EffectiveServer devuelve el ServerConfig del entorno activo.
func (p Project) EffectiveServer() ServerConfig {
	return p.EffectiveEnv().Server
}

// EnsureEnvsFromTopLevel expone ensureEnvs al package config: sintetiza
// `dev` desde el top-level cuando Envs es nil (legacy). No toca envs
// explícitos (incluido mapa vacío, que Validate rechaza).
func (p *Project) EnsureEnvsFromTopLevel() {
	if p.Envs != nil {
		return
	}
	p.ensureEnvs()
}

// ensureEnvs sintetiza `dev` desde el top-level cuando no hay envs.
// Normaliza envs existentes: Vars no-nil y EnvFile con default.
func (p *Project) ensureEnvs() {
	if len(p.Envs) > 0 {
		if p.ActiveEnv == "" {
			if _, ok := p.Envs["dev"]; ok {
				p.ActiveEnv = "dev"
			} else {
				for k := range p.Envs {
					p.ActiveEnv = k
					break
				}
			}
		}
		for name, e := range p.Envs {
			if e.Vars == nil {
				e.Vars = map[string]string{}
			}
			if e.EnvFile == "" {
				e.EnvFile = DefaultEnvFile(name)
			}
			if e.Secrets == nil {
				e.Secrets = []string{}
			}
			p.Envs[name] = e
		}
		return
	}
	p.Envs = map[string]EnvConfig{
		"dev": {Server: p.Server, Playwright: p.Playwright, User: p.User,
			Vars: map[string]string{}, EnvFile: DefaultEnvFile("dev"), Secrets: []string{}},
	}
	p.ActiveEnv = "dev"
}

// Validate replica Project.validate().
func (p Project) Validate() []string {
	var errs []string
	if strings.TrimSpace(p.Name) == "" {
		errs = append(errs, "Project name cannot be empty")
	}
	if strings.TrimSpace(p.Path) == "" {
		errs = append(errs, "Project path cannot be empty")
	}
	errs = append(errs, p.validateTabs()...)
	errs = append(errs, p.validateEnvs()...)
	errs = append(errs, p.validateWorkflows()...)
	errs = append(errs, p.validateWebhooks()...)
	errs = append(errs, p.validateCI()...)
	return errs
}

// validateEnvs valida nombres de entorno y que active_env exista.
// Cubre "prohibir borrar active/last": tras borrar el active, active_env
// queda huérfano; tras borrar el último, envs queda vacío.
// Envs nil = legacy en memoria (se sintetiza al guardar): se tolera para
// no romper structs construidos sin envs; mapa no-nil vacío sí es error.
func (p Project) validateEnvs() []string {
	var errs []string
	if p.Envs == nil {
		return errs
	}
	if len(p.Envs) == 0 {
		errs = append(errs, "project must have at least one environment")
		return errs
	}
	for name, e := range p.Envs {
		if !IsValidEnvName(name) {
			errs = append(errs, fmt.Sprintf(`invalid environment name %q (use [a-z0-9_-]{1,32})`, name))
		}
		for _, s := range e.Secrets {
			if err := env.ValidateKey(s); err != nil {
				errs = append(errs, fmt.Sprintf(`invalid secret key %q in environment %q`, s, name))
			}
		}
	}
	if p.ActiveEnv == "" {
		errs = append(errs, "active_env cannot be empty")
		return errs
	}
	if !IsValidEnvName(p.ActiveEnv) {
		errs = append(errs, fmt.Sprintf(`invalid environment name %q (use [a-z0-9_-]{1,32})`, p.ActiveEnv))
		return errs
	}
	if _, ok := p.Envs[p.ActiveEnv]; !ok {
		errs = append(errs, fmt.Sprintf(`active environment %q does not exist`, p.ActiveEnv))
	}
	return errs
}

// isHTTPURL reporta si s es http(s)://... (para webhooks y steps).
func isHTTPURL(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// validateWorkflows valida workflows: nombres no vacíos, trigger válido,
// steps no vacíos, URLs http(s) en steps webhook.
func (p Project) validateWorkflows() []string {
	var errs []string
	seen := map[string]bool{}
	for i, w := range p.Workflows {
		where := fmt.Sprintf("workflows[%d]", i)
		if strings.TrimSpace(w.Name) == "" {
			errs = append(errs, where+": name cannot be empty")
		}
		if w.ID != "" {
			if seen[w.ID] {
				errs = append(errs, fmt.Sprintf(where+": duplicate id %q", w.ID))
			}
			seen[w.ID] = true
		}
		switch w.Trigger {
		case WorkflowTriggerManual, WorkflowTriggerSchedule, WorkflowTriggerEvent:
		case "":
			errs = append(errs, where+": trigger cannot be empty")
		default:
			errs = append(errs, fmt.Sprintf(where+": invalid trigger %q", w.Trigger))
		}
		if w.Trigger == WorkflowTriggerSchedule && strings.TrimSpace(w.Schedule) == "" {
			errs = append(errs, where+": schedule cannot be empty for schedule trigger")
		}
		if w.Trigger == WorkflowTriggerEvent {
			switch w.Event {
			case WorkflowEventServerStarted, WorkflowEventServerStopped,
				WorkflowEventTestsFinished, WorkflowEventWebhookReceived:
			case "":
				errs = append(errs, where+": event cannot be empty for event trigger")
			default:
				errs = append(errs, fmt.Sprintf(where+": invalid event %q", w.Event))
			}
		}
		if len(w.Steps) == 0 {
			errs = append(errs, where+": steps cannot be empty")
		}
		for j, s := range w.Steps {
			swhere := fmt.Sprintf("%s.steps[%d]", where, j)
			if strings.TrimSpace(s.Name) == "" {
				errs = append(errs, swhere+": name cannot be empty")
			}
			switch s.Kind {
			case WorkflowStepCommand:
				if strings.TrimSpace(s.Command) == "" {
					errs = append(errs, swhere+": command cannot be empty for command step")
				}
			case WorkflowStepNotify:
				if strings.TrimSpace(s.Command) == "" && strings.TrimSpace(s.Body) == "" {
					errs = append(errs, swhere+": notify step needs command or body as message")
				}
			case WorkflowStepWebhook:
				if !isHTTPURL(s.URL) {
					errs = append(errs, swhere+": url must be http(s) for webhook step")
				}
			case WorkflowStepCITrigger:
				if strings.TrimSpace(s.CITarget) == "" {
					errs = append(errs, swhere+": ci_target cannot be empty for ci_trigger step")
				}
			case "":
				errs = append(errs, swhere+": kind cannot be empty")
			default:
				errs = append(errs, fmt.Sprintf(swhere+": invalid kind %q", s.Kind))
			}
			if s.Retry < 0 || s.Retry > 3 {
				errs = append(errs, swhere+": retry must be 0-3")
			}
			if s.TimeoutSec < 0 || s.TimeoutSec > 3600 {
				errs = append(errs, swhere+": timeout_sec must be 0-3600")
			}
		}
	}
	return errs
}

// validateWebhooks valida webhooks salientes: nombre no vacío, URL http(s),
// al menos un evento conocido.
func (p Project) validateWebhooks() []string {
	var errs []string
	for i, wh := range p.Webhooks {
		where := fmt.Sprintf("webhooks[%d]", i)
		if strings.TrimSpace(wh.Name) == "" {
			errs = append(errs, where+": name cannot be empty")
		}
		if !isHTTPURL(wh.URL) {
			errs = append(errs, where+": url must be http(s)")
		}
		if len(wh.Events) == 0 {
			errs = append(errs, where+": events cannot be empty")
		}
		for _, e := range wh.Events {
			switch e {
			case WorkflowEventServerStarted, WorkflowEventServerStopped,
				WorkflowEventTestsFinished, WorkflowEventWebhookReceived:
			default:
				errs = append(errs, fmt.Sprintf(where+": invalid event %q", e))
			}
		}
	}
	return errs
}

// validateCI valida URLs http(s) cuando el target está configurado y que los
// token refs usen formato env:NAME o vacío (nunca token en claro).
func (p Project) validateCI() []string {
	var errs []string
	for _, ref := range []string{p.CICD.GitHub.TokenRef, p.CICD.GitLab.TokenRef, p.CICD.Jenkins.TokenRef} {
		if ref != "" && !strings.HasPrefix(ref, "env:") {
			errs = append(errs, fmt.Sprintf("cicd: token_ref %q must use env:NAME format", ref))
		}
	}
	gh := p.CICD.GitHub
	if gh.Owner != "" || gh.Repo != "" || gh.WorkflowFile != "" {
		if strings.TrimSpace(gh.Owner) == "" || strings.TrimSpace(gh.Repo) == "" {
			errs = append(errs, "cicd.github: owner and repo are required together")
		}
		if gh.APIBase != "" && !isHTTPURL(gh.APIBase) {
			errs = append(errs, "cicd.github: api_base must be http(s)")
		}
	}
	gl := p.CICD.GitLab
	if gl.BaseURL != "" || gl.ProjectID != "" {
		if !isHTTPURL(gl.BaseURL) {
			errs = append(errs, "cicd.gitlab: base_url must be http(s)")
		}
		if strings.TrimSpace(gl.ProjectID) == "" {
			errs = append(errs, "cicd.gitlab: project_id is required with base_url")
		}
	}
	jk := p.CICD.Jenkins
	if jk.BaseURL != "" || jk.Job != "" {
		if !isHTTPURL(jk.BaseURL) {
			errs = append(errs, "cicd.jenkins: base_url must be http(s)")
		}
		if strings.TrimSpace(jk.Job) == "" {
			errs = append(errs, "cicd.jenkins: job is required with base_url")
		}
	}
	return errs
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// validateTabs valida ids conocidos, sin duplicados y logs no ocultable.
func (p Project) validateTabs() []string {
	var errs []string
	seen := map[string]bool{}
	for _, id := range p.Tabs.Hidden {
		if id == "logs" {
			errs = append(errs, `tab "logs" cannot be hidden`)
		}
		if !containsString(KnownTabs, id) {
			errs = append(errs, fmt.Sprintf(`unknown tab %q in hidden`, id))
		}
		if seen[id] {
			errs = append(errs, fmt.Sprintf(`duplicate tab %q in hidden`, id))
		}
		seen[id] = true
	}
	seen = map[string]bool{}
	for _, id := range p.Tabs.Order {
		if !containsString(KnownTabs, id) {
			errs = append(errs, fmt.Sprintf(`unknown tab %q in order`, id))
		}
		if seen[id] {
			errs = append(errs, fmt.Sprintf(`duplicate tab %q in order`, id))
		}
		seen[id] = true
	}
	return errs
}

// Tipos sombra con punteros: distinguen clave ausente de valor cero,
// replicando dict.get(key, default) de Python.
type serverConfigJSON struct {
	Enabled        *bool   `json:"enabled"`
	Command        *string `json:"command"`
	Port           *int    `json:"port"`
	URL            *string `json:"url"`
	StartupTimeout *int    `json:"startup_timeout"`
}

type playwrightConfigJSON struct {
	Enabled       *bool   `json:"enabled"`
	Command       *string `json:"command"`
	UICommand     *string `json:"ui_command"`
	DebugCommand  *string `json:"debug_command"`
	ReportCommand *string `json:"report_command"`
}

type backlogItemJSON struct {
	ID          *string `json:"id"`
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	Priority    *string `json:"priority"`
	CreatedAt   *string `json:"created_at"`
	UpdatedAt   *string `json:"updated_at"`
}

type userConfigJSON struct {
	Enabled *bool   `json:"enabled"`
	Command *string `json:"command"`
}

type tabsConfigJSON struct {
	Hidden *[]string `json:"hidden"`
	Order  *[]string `json:"order"`
}

type envConfigJSON struct {
	Server     *serverConfigJSON     `json:"server"`
	Playwright *playwrightConfigJSON `json:"playwright"`
	User       *userConfigJSON       `json:"user"`
	Vars       *map[string]string    `json:"vars"`
	EnvFile    *string               `json:"env_file"`
	Secrets    *[]string             `json:"secrets"`
}

type projectJSON struct {
	Name       *string               `json:"name"`
	Path       *string               `json:"path"`
	Server     *serverConfigJSON     `json:"server"`
	Playwright *playwrightConfigJSON `json:"playwright"`
	User       *userConfigJSON       `json:"user"`
	Tabs       *tabsConfigJSON       `json:"tabs"`
	Pinned     *bool                 `json:"pinned"`
	Backlog    *[]backlogItemJSON    `json:"backlog"`
	ActiveEnv  *string               `json:"active_env"`
	Envs       *map[string]envConfigJSON `json:"envs"`
	Workflows  *[]workflowJSON       `json:"workflows"`
	Webhooks   *[]webhookJSON        `json:"webhooks"`
	CICD       *ciConfigJSON         `json:"cicd"`
}

// Tipos sombra para workflows/webhooks/cicd: ausente → vacío (legacy).
type workflowStepJSON struct {
	ID         *string            `json:"id"`
	Name       *string            `json:"name"`
	Kind       *string            `json:"kind"`
	Command    *string            `json:"command"`
	URL        *string            `json:"url"`
	Method     *string            `json:"method"`
	Headers    *map[string]string `json:"headers"`
	Body       *string            `json:"body"`
	CITarget   *string            `json:"ci_target"`
	Retry      *int               `json:"retry"`
	TimeoutSec *int               `json:"timeout_sec"`
}

type workflowJSON struct {
	ID       *string          `json:"id"`
	Name     *string          `json:"name"`
	Enabled  *bool            `json:"enabled"`
	Trigger  *string          `json:"trigger"`
	Schedule *string          `json:"schedule"`
	Event    *string          `json:"event"`
	Steps    *[]workflowStepJSON `json:"steps"`
}

type webhookJSON struct {
	ID      *string   `json:"id"`
	Name    *string   `json:"name"`
	URL     *string   `json:"url"`
	Events  *[]string `json:"events"`
	Enabled *bool     `json:"enabled"`
}

type ciConfigJSON struct {
	GitHub  *gitHubCIJSON  `json:"github"`
	GitLab  *gitLabCIJSON  `json:"gitlab"`
	Jenkins *jenkinsCIJSON `json:"jenkins"`
}

type gitHubCIJSON struct {
	Owner        *string `json:"owner"`
	Repo         *string `json:"repo"`
	WorkflowFile *string `json:"workflow_file"`
	Ref          *string `json:"ref"`
	TokenRef     *string `json:"token_ref"`
	APIBase      *string `json:"api_base"`
}

type gitLabCIJSON struct {
	BaseURL   *string `json:"base_url"`
	ProjectID *string `json:"project_id"`
	Ref       *string `json:"ref"`
	TokenRef  *string `json:"token_ref"`
}

type jenkinsCIJSON struct {
	BaseURL  *string `json:"base_url"`
	Job      *string `json:"job"`
	TokenRef *string `json:"token_ref"`
}

func strOr(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}

func applyWorkflowStep(j workflowStepJSON) WorkflowStep {
	s := WorkflowStep{Method: "POST"}
	if j.ID != nil {
		s.ID = *j.ID
	}
	if j.Name != nil {
		s.Name = *j.Name
	}
	if j.Kind != nil {
		s.Kind = *j.Kind
	} else {
		s.Kind = ""
	}
	if j.Command != nil {
		s.Command = *j.Command
	}
	if j.URL != nil {
		s.URL = *j.URL
	}
	if j.Method != nil && *j.Method != "" {
		s.Method = *j.Method
	}
	if j.Headers != nil {
		s.Headers = make(map[string]string, len(*j.Headers))
		for k, v := range *j.Headers {
			s.Headers[k] = v
		}
	}
	if j.Body != nil {
		s.Body = *j.Body
	}
	if j.CITarget != nil {
		s.CITarget = *j.CITarget
	}
	if j.Retry != nil {
		s.Retry = *j.Retry
	}
	if j.TimeoutSec != nil {
		s.TimeoutSec = *j.TimeoutSec
	}
	return s
}

func applyWorkflow(j workflowJSON) Workflow {
	w := Workflow{Enabled: true}
	if j.ID != nil {
		w.ID = *j.ID
	}
	if j.Name != nil {
		w.Name = *j.Name
	}
	if j.Enabled != nil {
		w.Enabled = *j.Enabled
	}
	if j.Trigger != nil {
		w.Trigger = *j.Trigger
	}
	if j.Schedule != nil {
		w.Schedule = *j.Schedule
	}
	if j.Event != nil {
		w.Event = *j.Event
	}
	w.Steps = []WorkflowStep{}
	if j.Steps != nil {
		w.Steps = make([]WorkflowStep, len(*j.Steps))
		for i, s := range *j.Steps {
			w.Steps[i] = applyWorkflowStep(s)
		}
	}
	return w
}

func applyWebhook(j webhookJSON) WebhookConfig {
	w := WebhookConfig{Enabled: true}
	if j.ID != nil {
		w.ID = *j.ID
	}
	if j.Name != nil {
		w.Name = *j.Name
	}
	if j.URL != nil {
		w.URL = *j.URL
	}
	if j.Events != nil {
		w.Events = append([]string{}, *j.Events...)
	} else {
		w.Events = []string{}
	}
	if j.Enabled != nil {
		w.Enabled = *j.Enabled
	}
	return w
}

func applyCIConfig(j *ciConfigJSON) CIConfig {
	var c CIConfig
	if j == nil {
		return c
	}
	if j.GitHub != nil {
		c.GitHub = GitHubCIConfig{
			Owner: strOr(j.GitHub.Owner, ""), Repo: strOr(j.GitHub.Repo, ""),
			WorkflowFile: strOr(j.GitHub.WorkflowFile, ""), Ref: strOr(j.GitHub.Ref, ""),
			TokenRef: strOr(j.GitHub.TokenRef, ""), APIBase: strOr(j.GitHub.APIBase, ""),
		}
	}
	if j.GitLab != nil {
		c.GitLab = GitLabCIConfig{
			BaseURL: strOr(j.GitLab.BaseURL, ""), ProjectID: strOr(j.GitLab.ProjectID, ""),
			Ref: strOr(j.GitLab.Ref, ""), TokenRef: strOr(j.GitLab.TokenRef, ""),
		}
	}
	if j.Jenkins != nil {
		c.Jenkins = JenkinsCIConfig{
			BaseURL: strOr(j.Jenkins.BaseURL, ""), Job: strOr(j.Jenkins.Job, ""),
			TokenRef: strOr(j.Jenkins.TokenRef, ""),
		}
	}
	return c
}

func applyEnv(j *envConfigJSON, name string) EnvConfig {
	e := EnvConfig{
		Server:     applyServer(nil),
		Playwright: applyPlaywright(nil),
		User:       applyUser(nil),
		Vars:       map[string]string{},
		EnvFile:    DefaultEnvFile(name),
		Secrets:    []string{},
	}
	if j == nil {
		return e
	}
	e.Server = applyServer(j.Server)
	e.Playwright = applyPlaywright(j.Playwright)
	e.User = applyUser(j.User)
	if j.Vars != nil {
		cp := make(map[string]string, len(*j.Vars))
		for k, v := range *j.Vars {
			cp[k] = v
		}
		e.Vars = cp
	}
	if j.EnvFile != nil && *j.EnvFile != "" {
		e.EnvFile = *j.EnvFile
	}
	if j.Secrets != nil {
		e.Secrets = append([]string{}, *j.Secrets...)
	}
	return e
}

func applyServer(j *serverConfigJSON) ServerConfig {
	c := ServerConfig{
		Enabled:        true,
		Command:        "npm run dev",
		Port:           5173,
		URL:            "http://localhost:5173",
		StartupTimeout: 15000,
	}
	if j == nil {
		return c
	}
	if j.Enabled != nil {
		c.Enabled = *j.Enabled
	}
	if j.Command != nil {
		c.Command = *j.Command
	}
	if j.Port != nil {
		c.Port = *j.Port
	}
	if j.URL != nil {
		c.URL = *j.URL
	}
	if j.StartupTimeout != nil {
		c.StartupTimeout = *j.StartupTimeout
	}
	return c
}

func applyPlaywright(j *playwrightConfigJSON) PlaywrightConfig {
	c := PlaywrightConfig{
		Enabled:       true,
		Command:       "npx playwright test",
		UICommand:     "npx playwright test --ui",
		DebugCommand:  "npx playwright test --debug",
		ReportCommand: "npx playwright show-report",
	}
	if j == nil {
		return c
	}
	if j.Enabled != nil {
		c.Enabled = *j.Enabled
	}
	if j.Command != nil {
		c.Command = *j.Command
	}
	if j.UICommand != nil {
		c.UICommand = *j.UICommand
	}
	if j.DebugCommand != nil {
		c.DebugCommand = *j.DebugCommand
	}
	if j.ReportCommand != nil {
		c.ReportCommand = *j.ReportCommand
	}
	return c
}

func applyUser(j *userConfigJSON) UserConfig {
	c := UserConfig{
		Enabled: true,
		Command: "",
	}
	if j == nil {
		return c
	}
	if j.Enabled != nil {
		c.Enabled = *j.Enabled
	}
	if j.Command != nil {
		c.Command = *j.Command
	}
	return c
}

func applyBacklogItem(j backlogItemJSON) BacklogItem {
	now := time.Now().Format(time.RFC3339Nano)
	c := BacklogItem{
		ID:          "",
		Title:       "",
		Description: "",
		Status:      "todo",
		Priority:    "medium",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if j.ID != nil {
		c.ID = *j.ID
	}
	if j.Title != nil {
		c.Title = *j.Title
	}
	if j.Description != nil {
		c.Description = *j.Description
	}
	if j.Status != nil {
		c.Status = *j.Status
	}
	if j.Priority != nil {
		c.Priority = *j.Priority
	}
	if j.CreatedAt != nil {
		c.CreatedAt = *j.CreatedAt
	}
	if j.UpdatedAt != nil {
		c.UpdatedAt = *j.UpdatedAt
	}
	return c
}

func applyTabsConfig(j *tabsConfigJSON) TabsConfig {
	if j == nil {
		return TabsConfig{}
	}
	t := TabsConfig{}
	if j.Hidden != nil {
		t.Hidden = append([]string{}, *j.Hidden...)
	}
	if j.Order != nil {
		t.Order = append([]string{}, *j.Order...)
	}
	return t
}

// ParseProject replica Project.from_dict(): claves ausentes → defaults.
func ParseProject(data []byte) (Project, error) {
	var j projectJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return Project{}, fmt.Errorf("invalid project json: %w", err)
	}
	p := Project{
		Server:     applyServer(j.Server),
		Playwright: applyPlaywright(j.Playwright),
		User:       applyUser(j.User),
		Tabs:       applyTabsConfig(j.Tabs),
		Backlog:    []BacklogItem{},
		Workflows:  []Workflow{},
		Webhooks:   []WebhookConfig{},
	}
	if j.Name != nil {
		p.Name = *j.Name
	}
	if j.Path != nil {
		p.Path = *j.Path
	}
	if j.Pinned != nil {
		p.Pinned = *j.Pinned
	}
	if j.Backlog != nil {
		p.Backlog = make([]BacklogItem, len(*j.Backlog))
		for i, item := range *j.Backlog {
			p.Backlog[i] = applyBacklogItem(item)
		}
	}
	if j.ActiveEnv != nil {
		p.ActiveEnv = *j.ActiveEnv
	}
	if j.Envs != nil {
		p.Envs = make(map[string]EnvConfig, len(*j.Envs))
		for k, v := range *j.Envs {
			vv := v
			p.Envs[k] = applyEnv(&vv, k)
		}
	}
	if j.Workflows != nil {
		p.Workflows = make([]Workflow, len(*j.Workflows))
		for i, w := range *j.Workflows {
			p.Workflows[i] = applyWorkflow(w)
		}
	}
	if j.Webhooks != nil {
		p.Webhooks = make([]WebhookConfig, len(*j.Webhooks))
		for i, w := range *j.Webhooks {
			p.Webhooks[i] = applyWebhook(w)
		}
	}
	p.CICD = applyCIConfig(j.CICD)
	p.ensureEnvs()
	return p, nil
}
