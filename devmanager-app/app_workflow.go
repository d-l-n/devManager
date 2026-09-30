package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/google/uuid"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/workflow"
)

// ---- Workflows / Webhooks / CI-CD bindings (Issue #65, MVP) ----
//
// Webhooks salientes: List/Save/DeleteWebhook por proyecto (los steps de
// kind webhook llevan su propia URL y no necesitan registro).
// Eventos push: workflow:started, workflow:finished, workflow:listener_error.

// CIStatusEntry es el último estado conocido de un target CI.
type CIStatusEntry struct {
	Target  string `json:"target"`
	Status  string `json:"status"` // unknown | success | failed | running
	Message string `json:"message"`
	At      string `json:"at"`
}

// CIBuildResult es la respuesta de TriggerCIBuild (ok + resumen para toast).
type CIBuildResult struct {
	OK      bool   `json:"ok"`
	Target  string `json:"target"`
	Message string `json:"message"`
}

// initWorkflows prepara el store persistido
// (%APPDATA%/devManager/workflows_runs.json, best-effort).
func (a *App) initWorkflows() {
	runsPath := ""
	if a.settingsPath != "" {
		runsPath = filepath.Join(filepath.Dir(a.settingsPath), "workflows_runs.json")
	}
	a.wfStore = workflow.NewStore(runsPath)
	a.wfStore.Load()
	a.wfMu.Lock()
	if a.wfRunning == nil {
		a.wfRunning = map[string]bool{}
	}
	if a.wfLastRun == nil {
		a.wfLastRun = map[string]time.Time{}
	}
	if a.ciStatus == nil {
		a.ciStatus = map[string]CIStatusEntry{}
	}
	a.wfMu.Unlock()
}

// wfStoreOrInit devuelve el store, creándolo lazy si startup() no corrió
// (tests / bindings tempranos). Nunca nil.
func (a *App) wfStoreOrInit() *workflow.Store {
	if a.wfStore != nil {
		return a.wfStore
	}
	a.initWorkflows()
	return a.wfStore
}

// ListWorkflows devuelve los workflows del proyecto (vacío si índice malo).
func (a *App) ListWorkflows(index int) []models.Workflow {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []models.Workflow{}
	}
	out := append([]models.Workflow{}, projects[index].Workflows...)
	if out == nil {
		return []models.Workflow{}
	}
	return out
}

// SaveWorkflow crea (id vacío → uuid) o actualiza por id, valida y persiste.
func (a *App) SaveWorkflow(index int, w models.Workflow) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	if strings.TrimSpace(w.ID) == "" {
		w.ID = uuid.NewString()
	}
	for i := range w.Steps {
		if strings.TrimSpace(w.Steps[i].ID) == "" {
			w.Steps[i].ID = uuid.NewString()
		}
		if strings.TrimSpace(w.Steps[i].Method) == "" && w.Steps[i].Kind == models.WorkflowStepWebhook {
			w.Steps[i].Method = "POST"
		}
	}
	if errs := workflow.ValidateWorkflow(w); len(errs) > 0 {
		return errs
	}
	p := projects[index]
	found := false
	for i := range p.Workflows {
		if p.Workflows[i].ID == w.ID {
			p.Workflows[i] = w
			found = true
			break
		}
	}
	if !found {
		p.Workflows = append(p.Workflows, w)
	}
	if errs := p.Validate(); len(errs) > 0 {
		return errs
	}
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DeleteWorkflow borra el workflow y su historial de runs.
func (a *App) DeleteWorkflow(index int, id string) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	p := projects[index]
	kept := p.Workflows[:0:0]
	found := false
	for _, w := range p.Workflows {
		if w.ID == id {
			found = true
			continue
		}
		kept = append(kept, w)
	}
	if !found {
		return []string{fmt.Sprintf("workflow %q not found", id)}
	}
	p.Workflows = kept
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	a.wfStoreOrInit().Clear(id)
	return nil
}

// wfRunKey identifica una ejecución en curso (dedupe scheduler/manual).
func wfRunKey(index int, workflowID string) string {
	return fmt.Sprintf("%d:%s", index, workflowID)
}

// RunWorkflow lanza el workflow en background (emite workflow:started y
// workflow:finished). Si ya hay un run en curso del mismo workflow, no
// duplica: devuelve error.
func (a *App) RunWorkflow(index int, id string) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	var found *models.Workflow
	for i := range projects[index].Workflows {
		if projects[index].Workflows[i].ID == id {
			w := projects[index].Workflows[i]
			found = &w
			break
		}
	}
	if found == nil {
		return []string{fmt.Sprintf("workflow %q not found", id)}
	}
	if err := a.runWorkflowAsync(index, *found, "manual", nil); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// runWorkflowAsync ejecuta en goroutine con dedupe por workflow.
func (a *App) runWorkflowAsync(index int, w models.Workflow, triggerEvent string, payload map[string]interface{}) error {
	key := wfRunKey(index, w.ID)
	a.wfMu.Lock()
	if a.wfRunning == nil {
		a.wfRunning = map[string]bool{}
	}
	if a.wfRunning[key] {
		a.wfMu.Unlock()
		return fmt.Errorf("workflow %q is already running", w.Name)
	}
	a.wfRunning[key] = true
	a.wfMu.Unlock()

	runID := uuid.NewString()
	if a.ctx != nil {
		wails.EventsEmit(a.ctx, "workflow:started", map[string]interface{}{
			"index": index, "workflowId": w.ID, "runId": runID,
		})
	}
	go func() {
		defer func() {
			a.wfMu.Lock()
			delete(a.wfRunning, key)
			a.wfLastRun[key] = time.Now()
			a.wfMu.Unlock()
		}()
		projects := a.cfg.Projects()
		projectName, projectPath, ciCfg := w.Name, "", models.CIConfig{}
		if index >= 0 && index < len(projects) {
			projectName = projects[index].Name
			projectPath = projects[index].Path
			ciCfg = projects[index].CICD
		}
		ex := &workflow.Executor{
			Dir:      projectPath,
			CIConfig: ciCfg,
			OnNotify: func(title, msg string) { a.emitNotify(title, msg, "info") },
		}
		run := ex.Execute(w, projectName, runID, triggerEvent)
		a.wfStoreOrInit().Add(run)
		if a.ctx != nil {
			wails.EventsEmit(a.ctx, "workflow:finished", map[string]interface{}{
				"index": index, "workflowId": w.ID, "runId": runID,
				"status": run.Status, "project": projectName, "name": w.Name,
			})
		}
		_ = payload
	}()
	return nil
}

// ListWorkflowRuns devuelve todos los runs (newest primero).
func (a *App) ListWorkflowRuns() []workflow.Run {
	return a.wfStoreOrInit().All()
}

// GetWorkflowRuns devuelve los runs de un workflow (newest primero).
func (a *App) GetWorkflowRuns(workflowID string) []workflow.Run {
	return a.wfStoreOrInit().Get(workflowID)
}

// TriggerEvent dispara manualmente un evento en un proyecto: corre los
// workflows event-triggered que lo escuchan + los webhooks salientes.
func (a *App) TriggerEvent(index int, event string) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	switch event {
	case models.WorkflowEventServerStarted, models.WorkflowEventServerStopped,
		models.WorkflowEventTestsFinished, models.WorkflowEventWebhookReceived:
	default:
		return []string{fmt.Sprintf("unknown event %q", event)}
	}
	a.FireWorkflowEvent(index, event, nil)
	return nil
}

// FireWorkflowEvent es el fan-out interno de eventos: workflows con
// trigger=event que escuchan eventName + webhooks salientes suscritos.
// No bloquea: cada run/POST va en su goroutine.
func (a *App) FireWorkflowEvent(index int, eventName string, payload map[string]interface{}) {
	if a.cfg == nil {
		return
	}
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return
	}
	p := projects[index]
	if payload == nil {
		payload = map[string]interface{}{}
	}
	for _, w := range p.Workflows {
		if !w.Enabled || w.Trigger != models.WorkflowTriggerEvent || w.Event != eventName {
			continue
		}
		_ = a.runWorkflowAsync(index, w, eventName, payload)
	}
	for _, wh := range p.Webhooks {
		if !wh.Enabled {
			continue
		}
		subscribed := false
		for _, e := range wh.Events {
			if e == eventName {
				subscribed = true
				break
			}
		}
		if !subscribed {
			continue
		}
		go func(url, project string) {
			_ = workflow.PostOutgoing(nil, url, eventName, project, payload)
		}(wh.URL, p.Name)
	}
}

// ---- Scheduler (ticker 60s, patrón backup/dashboard) ----

func (a *App) startWorkflowScheduler() {
	a.wfMu.Lock()
	if a.wfSchedStop != nil {
		a.wfMu.Unlock()
		return
	}
	a.wfSchedStop = make(chan struct{})
	stop := a.wfSchedStop
	a.wfMu.Unlock()
	go func() {
		a.checkWorkflowSchedules()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				a.checkWorkflowSchedules()
			}
		}
	}()
}

func (a *App) stopWorkflowScheduler() {
	a.wfMu.Lock()
	if a.wfSchedStop != nil {
		close(a.wfSchedStop)
		a.wfSchedStop = nil
	}
	a.wfMu.Unlock()
}

// checkWorkflowSchedules corre los workflows schedule-enabled vencidos.
// Dedupe: runWorkflowAsync rechaza si ya hay un run en curso.
func (a *App) checkWorkflowSchedules() {
	if a.cfg == nil || a.ctx == nil {
		return
	}
	now := time.Now()
	projects := a.cfg.Projects()
	for i := range projects {
		for _, w := range projects[i].Workflows {
			if !w.Enabled || w.Trigger != models.WorkflowTriggerSchedule {
				continue
			}
			last := a.workflowLastRun(i, w.ID)
			if workflow.MatchesSchedule(w, now, last) {
				_ = a.runWorkflowAsync(i, w, "schedule", nil)
			}
		}
	}
}

// workflowLastRun prefiere el mapa en memoria y cae al store persistido.
func (a *App) workflowLastRun(index int, workflowID string) time.Time {
	key := wfRunKey(index, workflowID)
	a.wfMu.Lock()
	t, ok := a.wfLastRun[key]
	a.wfMu.Unlock()
	if ok {
		return t
	}
	if s := a.wfStoreOrInit().LastFinishedAt(workflowID); s != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

// ---- Listener entrante (POST /hook/{workflowId} en localhost) ----

func (a *App) startWorkflowListener() {
	s := a.GetSettings()
	if !s.WorkflowWebhookEnabled {
		return
	}
	port := s.WorkflowWebhookPort
	if port < 1 || port > 65535 {
		port = 9876
	}
	onHook := func(workflowID string, payload map[string]interface{}) {
		a.onIncomingWebhook(workflowID, payload)
	}
	ln := workflow.NewListener("127.0.0.1", port, onHook)
	if err := ln.Start(); err != nil {
		// Nunca bloquear el boot por puerto ocupado: log + evento.
		a.emitNotify("Workflows", "Incoming webhook listener failed: "+err.Error(), "warning")
		if a.ctx != nil {
			wails.EventsEmit(a.ctx, "workflow:listener_error", map[string]string{
				"message": err.Error(), "addr": ln.Addr(),
			})
		}
		return
	}
	a.wfMu.Lock()
	a.wfListener = ln
	a.wfMu.Unlock()
}

// restartWorkflowListener re-bindea tras cambiar settings (puerto/enabled).
func (a *App) restartWorkflowListener() {
	a.stopWorkflowListener()
	a.startWorkflowListener()
}

func (a *App) stopWorkflowListener() {
	a.wfMu.Lock()
	ln := a.wfListener
	a.wfListener = nil
	a.wfMu.Unlock()
	if ln != nil {
		ln.Stop()
	}
}

// onIncomingWebhook busca el workflow por ID en todos los proyectos y lo
// dispara si es trigger=event; además fanea el evento webhook_received a los
// webhooks salientes del proyecto dueño.
func (a *App) onIncomingWebhook(workflowID string, payload map[string]interface{}) {
	if a.cfg == nil {
		return
	}
	projects := a.cfg.Projects()
	for i := range projects {
		for _, w := range projects[i].Workflows {
			if w.ID != workflowID {
				continue
			}
			if !w.Enabled || w.Trigger != models.WorkflowTriggerEvent {
				return
			}
			_ = a.runWorkflowAsync(i, w, models.WorkflowEventWebhookReceived, payload)
			a.FireWorkflowEvent(i, models.WorkflowEventWebhookReceived, payload)
			return
		}
	}
}

// GetWorkflowListenerAddr devuelve host:port efectivo ("" si apagado).
func (a *App) GetWorkflowListenerAddr() string {
	a.wfMu.Lock()
	defer a.wfMu.Unlock()
	if a.wfListener == nil {
		return ""
	}
	return a.wfListener.Addr()
}

// ---- Webhooks salientes (bindings por proyecto) ----

func (a *App) ListWebhooks(index int) []models.WebhookConfig {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []models.WebhookConfig{}
	}
	out := append([]models.WebhookConfig{}, projects[index].Webhooks...)
	if out == nil {
		return []models.WebhookConfig{}
	}
	return out
}

// SaveWebhook crea (id vacío → uuid) o actualiza por id, valida y persiste.
func (a *App) SaveWebhook(index int, wh models.WebhookConfig) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	if strings.TrimSpace(wh.ID) == "" {
		wh.ID = uuid.NewString()
	}
	p := projects[index]
	found := false
	for i := range p.Webhooks {
		if p.Webhooks[i].ID == wh.ID {
			p.Webhooks[i] = wh
			found = true
			break
		}
	}
	if !found {
		p.Webhooks = append(p.Webhooks, wh)
	}
	if errs := p.Validate(); len(errs) > 0 {
		return errs
	}
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// DeleteWebhook borra el webhook por id.
func (a *App) DeleteWebhook(index int, id string) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	p := projects[index]
	kept := p.Webhooks[:0:0]
	found := false
	for _, wh := range p.Webhooks {
		if wh.ID == id {
			found = true
			continue
		}
		kept = append(kept, wh)
	}
	if !found {
		return []string{fmt.Sprintf("webhook %q not found", id)}
	}
	p.Webhooks = kept
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// ---- CI/CD (bindings por proyecto) ----

// maskedCIConfig devuelve la config con token refs enmascarados (***).
func maskedCIConfig(c models.CIConfig) models.CIConfig {
	if c.GitHub.TokenRef != "" {
		c.GitHub.TokenRef = models.MaskedValue
	}
	if c.GitLab.TokenRef != "" {
		c.GitLab.TokenRef = models.MaskedValue
	}
	if c.Jenkins.TokenRef != "" {
		c.Jenkins.TokenRef = models.MaskedValue
	}
	return c
}

// GetCIConfig devuelve la config CI con tokens enmascarados.
func (a *App) GetCIConfig(index int) models.CIConfig {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return models.CIConfig{}
	}
	return maskedCIConfig(projects[index].CICD)
}

// SaveCIConfig persiste la config CI; "***" preserva el valor guardado
// (paridad SetEnvVars con secrets).
func (a *App) SaveCIConfig(index int, cfg models.CIConfig) []string {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return []string{fmt.Sprintf("index %d out of range", index)}
	}
	p := projects[index]
	if cfg.GitHub.TokenRef == models.MaskedValue {
		cfg.GitHub.TokenRef = p.CICD.GitHub.TokenRef
	}
	if cfg.GitLab.TokenRef == models.MaskedValue {
		cfg.GitLab.TokenRef = p.CICD.GitLab.TokenRef
	}
	if cfg.Jenkins.TokenRef == models.MaskedValue {
		cfg.Jenkins.TokenRef = p.CICD.Jenkins.TokenRef
	}
	p.CICD = cfg
	if errs := p.Validate(); len(errs) > 0 {
		return errs
	}
	if err := a.cfg.UpdateProject(index, p); err != nil {
		return []string{err.Error()}
	}
	return nil
}

// TriggerCIBuild dispara un build real (POST, timeout 15s) y registra el
// último estado conocido (para GetCIStatus). Nunca loguea tokens.
func (a *App) TriggerCIBuild(index int, target string) CIBuildResult {
	projects := a.cfg.Projects()
	if index < 0 || index >= len(projects) {
		return CIBuildResult{OK: false, Target: target, Message: fmt.Sprintf("index %d out of range", index)}
	}
	p := projects[index]
	msg, err := workflow.TriggerCIBuild(target, p.Name, p.CICD, &http.Client{Timeout: 15 * time.Second})
	entry := CIStatusEntry{Target: target, At: time.Now().Format(time.RFC3339Nano)}
	if err != nil {
		entry.Status = "failed"
		entry.Message = err.Error()
	} else {
		entry.Status = "success"
		entry.Message = msg
	}
	a.wfMu.Lock()
	if a.ciStatus == nil {
		a.ciStatus = map[string]CIStatusEntry{}
	}
	a.ciStatus[fmt.Sprintf("%d:%s", index, strings.ToLower(strings.TrimSpace(target)))] = entry
	a.wfMu.Unlock()
	if err != nil {
		return CIBuildResult{OK: false, Target: target, Message: err.Error()}
	}
	return CIBuildResult{OK: true, Target: target, Message: msg}
}

// GetCIStatus devuelve el último estado conocido del target (unknown si
// nunca se disparó).
func (a *App) GetCIStatus(index int, target string) CIStatusEntry {
	a.wfMu.Lock()
	defer a.wfMu.Unlock()
	if e, ok := a.ciStatus[fmt.Sprintf("%d:%s", index, strings.ToLower(strings.TrimSpace(target)))]; ok {
		return e
	}
	return CIStatusEntry{Target: target, Status: "unknown", Message: "no builds triggered yet"}
}
