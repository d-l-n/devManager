package models

import (
	"strings"
	"testing"
)

// Legacy sin workflows/webhooks/cicd → vacíos, sin errores.
func TestParseProjectWorkflowLegacyDefaults(t *testing.T) {
	p, err := ParseProject([]byte(`{"name":"x","path":"y"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Workflows) != 0 || len(p.Webhooks) != 0 {
		t.Fatalf("legacy debe dar vacíos: %+v", p)
	}
	if (p.CICD != CIConfig{}) {
		t.Fatalf("cicd legacy debe ser cero: %+v", p.CICD)
	}
	if errs := p.Validate(); len(errs) != 0 {
		t.Fatalf("legacy válido, got %v", errs)
	}
}

func TestParseProjectWorkflowsRoundTrip(t *testing.T) {
	raw := `{"name":"x","path":"y","workflows":[
		{"id":"w1","name":"nightly","enabled":true,"trigger":"schedule",
		 "schedule":"@every 1h","steps":[
			{"id":"s1","name":"tests","kind":"command","command":"npm test","retry":1,"timeout_sec":300},
			{"id":"s2","name":"ping","kind":"webhook","url":"https://example.com/hook"}
		]}],
		"webhooks":[{"id":"h1","name":"ops","url":"https://example.com/ev","events":["server_started"],"enabled":true}],
		"cicd":{"github":{"owner":"o","repo":"r","token_ref":"env:GH_TOKEN"}}}`
	p, err := ParseProject([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Workflows) != 1 || len(p.Workflows[0].Steps) != 2 {
		t.Fatalf("workflows mal parseados: %+v", p.Workflows)
	}
	if p.Workflows[0].Steps[0].Retry != 1 || p.Workflows[0].Steps[0].TimeoutSec != 300 {
		t.Fatalf("step opts: %+v", p.Workflows[0].Steps[0])
	}
	if p.Workflows[0].Steps[1].Method != "POST" {
		t.Fatalf("method default POST: %+v", p.Workflows[0].Steps[1])
	}
	if len(p.Webhooks) != 1 || p.CICD.GitHub.Owner != "o" {
		t.Fatalf("webhooks/cicd: %+v %+v", p.Webhooks, p.CICD)
	}
	if errs := p.Validate(); len(errs) != 0 {
		t.Fatalf("válido, got %v", errs)
	}
}

func TestValidateWorkflowsBad(t *testing.T) {
	p := Project{Name: "x", Path: "y"}
	p.EnsureEnvsFromTopLevel()
	p.Workflows = []Workflow{
		{ID: "a", Name: "", Trigger: "bogus", Steps: nil},
		{ID: "a", Name: "ok", Trigger: "schedule", Schedule: "", Steps: []WorkflowStep{
			{ID: "s", Name: "", Kind: "webhook", URL: "ftp://x", Retry: 9},
		}},
		{ID: "b", Name: "ev", Trigger: "event", Event: "bogus", Steps: []WorkflowStep{
			{ID: "s", Name: "n", Kind: "notify"},
		}},
	}
	errs := p.Validate()
	joined := strings.Join(errs, ";")
	for _, want := range []string{
		"name cannot be empty", "invalid trigger", "steps cannot be empty",
		"duplicate id", "schedule cannot be empty", "url must be http(s)",
		"retry must be 0-3", "invalid event", "notify step needs",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %q en %v", want, errs)
		}
	}
}

func TestValidateWebhooksAndCI(t *testing.T) {
	p := Project{Name: "x", Path: "y"}
	p.EnsureEnvsFromTopLevel()
	p.Webhooks = []WebhookConfig{{ID: "h", Name: "", URL: "notaurl", Events: []string{"bogus"}}}
	p.CICD.GitHub.TokenRef = "plaintext-token"
	p.CICD.GitLab.BaseURL = "https://gl.example.com"
	errs := p.Validate()
	joined := strings.Join(errs, ";")
	for _, want := range []string{
		"webhooks[0]: name cannot be empty", "url must be http(s)",
		"invalid event", "must use env:NAME",
		"gitlab: project_id is required",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("falta %q en %v", want, errs)
		}
	}
}

func TestKnownTabsIncludesWorkflows(t *testing.T) {
	found := false
	for _, tab := range KnownTabs {
		if tab == "workflows" {
			found = true
		}
	}
	if !found {
		t.Fatalf("KnownTabs debe incluir workflows: %v", KnownTabs)
	}
}
