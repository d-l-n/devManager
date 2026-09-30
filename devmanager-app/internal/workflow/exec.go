package workflow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

// Run registra una ejecución de workflow.
type Run struct {
	ID           string       `json:"id"`
	WorkflowID   string       `json:"workflowId"`
	ProjectName  string       `json:"project"`
	Status       string       `json:"status"` // running | success | failed
	StartedAt    string       `json:"startedAt"`
	FinishedAt   string       `json:"finishedAt"`
	Logs         []string     `json:"logs"`
	StepResults  []StepResult `json:"stepResults"`
	TriggerEvent string       `json:"triggerEvent,omitempty"`
}

// StepResult registra el resultado de un step.
type StepResult struct {
	StepID   string `json:"stepId"`
	StepName string `json:"stepName"`
	Kind     string `json:"kind"`
	Status   string `json:"status"` // success | failed | skipped
	Output   string `json:"output"`
	Attempts int    `json:"attempts"`
	Error    string `json:"error,omitempty"`
}

// Executor ejecuta workflows secuencialmente. Dir es el project.Path
// (working dir de los steps command). Client nil → default con 15s.
type Executor struct {
	Dir        string
	Client     *http.Client
	CIConfig   models.CIConfig
	CITrigger  func(target string, cfg models.CIConfig) (string, error)
	OnNotify   func(title, message string)
	RetryDelay time.Duration
}

// defaultTimeout es el timeout por step cuando timeout_sec es 0.
const defaultStepTimeout = 120 * time.Second

func (e *Executor) client() *http.Client {
	if e.Client != nil {
		return e.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (e *Executor) retryDelay() time.Duration {
	if e.RetryDelay > 0 {
		return e.RetryDelay
	}
	return 100 * time.Millisecond
}

// Execute corre el workflow paso a paso con retry + timeout por step.
// Nunca falla en sí: el fracaso vive en Run.Status/StepResults.
func (e *Executor) Execute(w models.Workflow, projectName, runID, triggerEvent string) Run {
	started := time.Now()
	run := Run{
		ID: runID, WorkflowID: w.ID, ProjectName: projectName,
		Status: "running", StartedAt: started.Format(time.RFC3339Nano),
		Logs: []string{}, StepResults: []StepResult{}, TriggerEvent: triggerEvent,
	}
	logf := func(format string, args ...interface{}) {
		run.Logs = append(run.Logs, fmt.Sprintf(format, args...))
	}
	failed := false
	for _, s := range w.Steps {
		if failed {
			run.StepResults = append(run.StepResults, StepResult{
				StepID: s.ID, StepName: s.Name, Kind: s.Kind, Status: "skipped",
			})
			logf("step %q skipped (previous failure)", s.Name)
			continue
		}
		res := e.runStep(s, w, logf)
		run.StepResults = append(run.StepResults, res)
		if res.Status != "success" {
			failed = true
		}
	}
	if failed {
		run.Status = "failed"
	} else {
		run.Status = "success"
	}
	run.FinishedAt = time.Now().Format(time.RFC3339Nano)
	return run
}

func (e *Executor) runStep(s models.WorkflowStep, w models.Workflow, logf func(string, ...interface{})) StepResult {
	res := StepResult{StepID: s.ID, StepName: s.Name, Kind: s.Kind}
	attempts := s.Retry + 1
	if attempts < 1 {
		attempts = 1
	}
	timeout := time.Duration(s.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = defaultStepTimeout
	}
	var lastErr string
	var lastOut string
	for a := 1; a <= attempts; a++ {
		res.Attempts = a
		out, err := e.runStepOnce(s, w, timeout)
		lastOut = out
		if err == nil {
			res.Status = "success"
			res.Output = truncate(out, 4000)
			logf("step %q ok (attempt %d)", s.Name, a)
			return res
		}
		lastErr = err.Error()
		logf("step %q attempt %d failed: %s", s.Name, a, lastErr)
		if a < attempts {
			time.Sleep(e.retryDelay() * time.Duration(a))
		}
	}
	res.Status = "failed"
	res.Output = truncate(lastOut, 4000)
	res.Error = lastErr
	return res
}

func (e *Executor) runStepOnce(s models.WorkflowStep, w models.Workflow, timeout time.Duration) (string, error) {
	switch s.Kind {
	case models.WorkflowStepNotify:
		msg := s.Body
		if msg == "" {
			msg = s.Command
		}
		if e.OnNotify != nil {
			e.OnNotify("Workflow "+w.Name, msg)
		}
		return msg, nil
	case models.WorkflowStepWebhook:
		return e.runWebhookStep(s, w, timeout)
	case models.WorkflowStepCITrigger:
		trig := e.CITrigger
		if trig == nil {
			trig = func(target string, cfg models.CIConfig) (string, error) {
				return TriggerCIBuild(target, w.Name, cfg, e.client())
			}
		}
		return trig(s.CITarget, e.CIConfig)
	default: // command (y kinds desconocidos → shell, validación los frena antes)
		return e.runCommand(s.Command, timeout)
	}
}

func (e *Executor) runCommand(command string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", fmt.Errorf("empty command")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := shellCommand(ctx, command, e.Dir)
	// Salida combinada: el log del run la conserva (cap 4000).
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return string(out), fmt.Errorf("timeout after %s", timeout)
	}
	if err != nil {
		return string(out), fmt.Errorf("exit: %v", err)
	}
	return string(out), nil
}

func (e *Executor) runWebhookStep(s models.WorkflowStep, w models.Workflow, timeout time.Duration) (string, error) {
	method := strings.ToUpper(strings.TrimSpace(s.Method))
	if method == "" {
		method = "POST"
	}
	body := s.Body
	if body == "" {
		payload := map[string]interface{}{
			"event": "workflow_step", "workflow": w.Name, "step": s.Name,
			"at": time.Now().Format(time.RFC3339Nano),
		}
		raw, _ := json.Marshal(payload)
		body = string(raw)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, s.URL, bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range s.Headers {
		if strings.EqualFold(k, "authorization") && strings.TrimSpace(v) == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	resp, err := e.client().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4000))
	if resp.StatusCode >= 400 {
		return string(raw), fmt.Errorf("webhook returned %d", resp.StatusCode)
	}
	return string(raw), nil
}

// OutgoingPayload es el JSON de los webhooks salientes.
type OutgoingPayload struct {
	Event   string      `json:"event"`
	Project string      `json:"project"`
	At      string      `json:"at"`
	Payload interface{} `json:"payload,omitempty"`
}

// PostOutgoing POSTea {event, project, at, payload} a url (timeout 15s).
func PostOutgoing(client *http.Client, url, event, project string, payload interface{}) error {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	raw, _ := json.Marshal(OutgoingPayload{
		Event: event, Project: project,
		At:      time.Now().Format(time.RFC3339Nano),
		Payload: payload,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook %s returned %d", url, resp.StatusCode)
	}
	return nil
}

// ResolveTokenRef resuelve `env:NAME` a la variable de entorno ("" si falta).
// Cualquier otro formato se trata como ausente (nunca token en claro).
func ResolveTokenRef(ref string) string {
	if !strings.HasPrefix(ref, "env:") {
		return ""
	}
	return os.Getenv(strings.TrimPrefix(ref, "env:"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…[truncated]"
}
