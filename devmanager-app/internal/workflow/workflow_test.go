package workflow

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

func goodWorkflow() models.Workflow {
	return models.Workflow{
		ID: "w1", Name: "demo", Enabled: true, Trigger: models.WorkflowTriggerManual,
		Steps: []models.WorkflowStep{
			{ID: "s1", Name: "hi", Kind: models.WorkflowStepCommand, Command: "echo hi"},
		},
	}
}

func TestValidateWorkflowOK(t *testing.T) {
	if errs := ValidateWorkflow(goodWorkflow()); len(errs) != 0 {
		t.Fatalf("esperaba sin errores, got %v", errs)
	}
}

func TestValidateWorkflowBad(t *testing.T) {
	w := goodWorkflow()
	w.Name = "  "
	w.Trigger = "bogus"
	w.Steps = nil
	errs := ValidateWorkflow(w)
	if len(errs) < 3 {
		t.Fatalf("esperaba >=3 errores, got %v", errs)
	}
	joined := strings.Join(errs, ";")
	for _, want := range []string{"name cannot be empty", "invalid trigger", "steps cannot be empty"} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %q en %v", want, errs)
		}
	}
}

func TestValidSchedule(t *testing.T) {
	ok := []string{"@every 5m", "@every 30s", "@every 1h", "@hourly", "@daily",
		"0 * * * *", "*/15 9-17 * * 1-5", "30 8 1 * *"}
	for _, s := range ok {
		if !ValidSchedule(s) {
			t.Errorf("esperaba válido %q", s)
		}
	}
	bad := []string{"", "every 5m", "@minutely", "@every banana", "* * * *", "99 * * * *", "*/x * * * *"}
	for _, s := range bad {
		if ValidSchedule(s) {
			t.Errorf("esperaba inválido %q", s)
		}
	}
}

func TestMatchesScheduleEvery(t *testing.T) {
	w := models.Workflow{Enabled: true, Trigger: models.WorkflowTriggerSchedule, Schedule: "@every 5m"}
	now := time.Now()
	if !MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("nunca corrido → debe vencer")
	}
	if !MatchesSchedule(w, now, now.Add(-6*time.Minute)) {
		t.Fatal("6m > 5m → debe vencer")
	}
	if MatchesSchedule(w, now, now.Add(-1*time.Minute)) {
		t.Fatal("1m < 5m → no debe vencer")
	}
	w.Enabled = false
	if MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("disabled → no vence")
	}
}

func TestMatchesScheduleCron(t *testing.T) {
	// Lunes 2026-09-07 09:30 (verificar weekday real por construcción).
	now := time.Date(2026, 9, 7, 9, 30, 0, 0, time.UTC)
	wd := int(now.Weekday())
	w := models.Workflow{Enabled: true, Trigger: models.WorkflowTriggerSchedule,
		Schedule: "30 9 * * *"}
	if !MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("cron exacto debe matchear")
	}
	w.Schedule = "31 9 * * *"
	if MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("minuto distinto no matchea")
	}
	w.Schedule = "*/15 9 * * *"
	if !MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("*/15 con min 30 debe matchear")
	}
	w.Schedule = "* * * * " + weekdayStr(wd)
	if !MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("dow exacto debe matchear")
	}
	// Dedupe: run hace 10s → no revencer aunque matchee.
	w.Schedule = "30 9 * * *"
	if MatchesSchedule(w, now, now.Add(-10*time.Second)) {
		t.Fatal("dedupe 50s: no debe revencer")
	}
	// @hourly / @daily.
	top := time.Date(2026, 9, 7, 14, 0, 0, 0, time.UTC)
	w.Schedule = "@hourly"
	if !MatchesSchedule(w, top, time.Time{}) {
		t.Fatal("@hourly en :00 debe vencer")
	}
	if MatchesSchedule(w, now, time.Time{}) {
		t.Fatal("@hourly en :30 no vence")
	}
	w.Schedule = "@daily"
	mid := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	if !MatchesSchedule(w, mid, time.Time{}) {
		t.Fatal("@daily en 00:00 debe vencer")
	}
}

func weekdayStr(wd int) string { return string(rune('0' + wd)) }

func TestExecutorRetrySuccess(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	e := &Executor{Dir: t.TempDir()}
	w := models.Workflow{ID: "w", Name: "r", Enabled: true, Trigger: "manual",
		Steps: []models.WorkflowStep{
			{ID: "s", Name: "hook", Kind: "webhook", URL: srv.URL, Retry: 3},
		}}
	run := e.Execute(w, "proj", "run-1", "")
	if run.Status != "success" {
		t.Fatalf("esperaba success, got %s logs=%v", run.Status, run.Logs)
	}
	if run.StepResults[0].Attempts != 3 {
		t.Fatalf("esperaba 3 attempts, got %d", run.StepResults[0].Attempts)
	}
}

func TestExecutorRetryExhausted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	e := &Executor{Dir: t.TempDir()}
	w := models.Workflow{ID: "w", Name: "r", Enabled: true, Trigger: "manual",
		Steps: []models.WorkflowStep{
			{ID: "s", Name: "hook", Kind: "webhook", URL: srv.URL, Retry: 1},
			{ID: "s2", Name: "next", Kind: "notify", Body: "hi"},
		}}
	run := e.Execute(w, "proj", "run-2", "")
	if run.Status != "failed" {
		t.Fatalf("esperaba failed, got %s", run.Status)
	}
	if run.StepResults[0].Attempts != 2 {
		t.Fatalf("esperaba 2 attempts, got %d", run.StepResults[0].Attempts)
	}
	if run.StepResults[1].Status != "skipped" {
		t.Fatalf("segundo step debe skipearse, got %s", run.StepResults[1].Status)
	}
}

func TestExecutorCommandAndNotify(t *testing.T) {
	var notified []string
	e := &Executor{Dir: t.TempDir(), OnNotify: func(title, msg string) {
		notified = append(notified, title+":"+msg)
	}}
	w := models.Workflow{ID: "w", Name: "r", Enabled: true, Trigger: "manual",
		Steps: []models.WorkflowStep{
			{ID: "s1", Name: "echo", Kind: "command", Command: "echo hello"},
			{ID: "s2", Name: "note", Kind: "notify", Body: "done"},
		}}
	run := e.Execute(w, "proj", "run-3", "")
	if run.Status != "success" {
		t.Fatalf("esperaba success, got %s (%v)", run.Status, run.Logs)
	}
	if !strings.Contains(run.StepResults[0].Output, "hello") {
		t.Fatalf("output debe contener hello: %q", run.StepResults[0].Output)
	}
	if len(notified) != 1 {
		t.Fatalf("esperaba 1 notify, got %v", notified)
	}
}

func TestExecutorCommandTimeout(t *testing.T) {
	e := &Executor{Dir: t.TempDir()}
	// Comando lento multiplataforma: sleep en unix, ping -n en Windows.
	slow := "sleep 30"
	if runtime.GOOS == "windows" {
		slow = "ping -n 5 127.0.0.1"
	}
	w := models.Workflow{ID: "w", Name: "r", Enabled: true, Trigger: "manual",
		Steps: []models.WorkflowStep{
			{ID: "s1", Name: "slow", Kind: "command", Command: slow, TimeoutSec: 1},
		}}
	run := e.Execute(w, "proj", "run-4", "")
	if run.Status != "failed" {
		t.Fatalf("esperaba failed por timeout, got %s", run.Status)
	}
	if !strings.Contains(run.StepResults[0].Error, "timeout") {
		t.Fatalf("error debe mencionar timeout: %q", run.StepResults[0].Error)
	}
}

func TestTriggerCIBuildGitHub(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		w.WriteHeader(204)
	}))
	defer srv.Close()
	t.Setenv("WF_TEST_TOKEN", "secret123")
	cfg := models.CIConfig{}
	cfg.GitHub.Owner = "o"
	cfg.GitHub.Repo = "r"
	cfg.GitHub.APIBase = srv.URL
	cfg.GitHub.TokenRef = "env:WF_TEST_TOKEN"
	msg, err := TriggerCIBuild("github", "proj", cfg, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "204") || gotPath != "/repos/o/r/dispatches" {
		t.Fatalf("inesperado: %q path=%q", msg, gotPath)
	}
	if gotAuth != "Bearer secret123" {
		t.Fatalf("auth inesperado: %q", gotAuth)
	}
}

func TestStoreCapAndPersist(t *testing.T) {
	path := t.TempDir() + "/runs.json"
	s := NewStore(path)
	s.Load()
	for i := 0; i < 105; i++ {
		s.Add(Run{ID: string(rune(i)), WorkflowID: "w", Status: "success"})
	}
	if got := len(s.Get("w")); got != MaxRunsPerWorkflow {
		t.Fatalf("esperaba cap %d, got %d", MaxRunsPerWorkflow, got)
	}
	s2 := NewStore(path)
	s2.Load()
	if got := len(s2.Get("w")); got != MaxRunsPerWorkflow {
		t.Fatalf("persist esperaba %d, got %d", MaxRunsPerWorkflow, got)
	}
	if got := len(s2.All()); got != MaxRunsPerWorkflow {
		t.Fatalf("all esperaba %d, got %d", MaxRunsPerWorkflow, got)
	}
}
