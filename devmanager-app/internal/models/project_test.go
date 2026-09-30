package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const fullJSON = `{
	"name": "MPoints Tracker",
	"path": "D:/Mi Home/Desktop/proyectos/mpoints-tracker",
	"server": {
		"enabled": true,
		"command": "npm run dev",
		"port": 5173,
		"url": "http://localhost:5173",
		"startup_timeout": 15000
	},
	"playwright": {
		"enabled": true,
		"command": "npx playwright test",
		"ui_command": "npx playwright test --ui",
		"debug_command": "npx playwright test --debug",
		"report_command": "npx playwright show-report"
	},
	"pinned": false
}`

func TestParseProjectFull(t *testing.T) {
	p, err := ParseProject([]byte(fullJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "MPoints Tracker" || p.Path != "D:/Mi Home/Desktop/proyectos/mpoints-tracker" {
		t.Errorf("bad name/path: %+v", p)
	}
	if p.Server.Port != 5173 || p.Server.Command != "npm run dev" || p.Server.StartupTimeout != 15000 {
		t.Errorf("bad server config: %+v", p.Server)
	}
	if p.Playwright.UICommand != "npx playwright test --ui" {
		t.Errorf("bad ui_command: %+v", p.Playwright)
	}
	if !p.User.Enabled {
		t.Error("user.enabled default debe ser true")
	}
	if p.User.Command != "" {
		t.Errorf("user.command default = %q", p.User.Command)
	}
}

func TestParseProjectDefaults(t *testing.T) {
	p, err := ParseProject([]byte(`{"name":"x","path":"y","server":{"port":3000}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.Server.Enabled {
		t.Error("server.enabled default debe ser true")
	}
	if p.Server.Command != "npm run dev" {
		t.Errorf("command default = %q", p.Server.Command)
	}
	if p.Server.Port != 3000 {
		t.Errorf("port explícito debe respetarse, got %d", p.Server.Port)
	}
	if p.Server.URL != "http://localhost:5173" {
		t.Errorf("url default = %q", p.Server.URL)
	}
	if p.Server.StartupTimeout != 15000 {
		t.Errorf("startup_timeout default = %d", p.Server.StartupTimeout)
	}
	if p.Playwright.Command != "npx playwright test" {
		t.Errorf("playwright.command default = %q", p.Playwright.Command)
	}
	if p.Pinned {
		t.Error("pinned default = false")
	}
}

func TestRoundTripStable(t *testing.T) {
	p, err := ParseProject([]byte(fullJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	p2, err := ParseProject(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if p.Name != p2.Name || p.Path != p2.Path || p.Server != p2.Server || p.Playwright != p2.Playwright || p.User != p2.User || !reflect.DeepEqual(p.Tabs, p2.Tabs) || p.Pinned != p2.Pinned {
		t.Errorf("round-trip inestable:\nin=%+v\nout=%+v", p, p2)
	}
	
	// Compare backlog slices
	if len(p.Backlog) != len(p2.Backlog) {
		t.Errorf("backlog length mismatch: %d vs %d", len(p.Backlog), len(p2.Backlog))
	} else {
		for i := range p.Backlog {
			if p.Backlog[i].ID != p2.Backlog[i].ID || p.Backlog[i].Title != p2.Backlog[i].Title || p.Backlog[i].Description != p2.Backlog[i].Description || p.Backlog[i].Status != p2.Backlog[i].Status || p.Backlog[i].Priority != p2.Backlog[i].Priority {
				t.Errorf("backlog item %d mismatch:\nin=%+v\nout=%+v", i, p.Backlog[i], p2.Backlog[i])
			}
		}
	}
	s := string(out)
	for _, key := range []string{`"startup_timeout"`, `"ui_command"`, `"debug_command"`, `"report_command"`} {
		if !strings.Contains(s, key) {
			t.Errorf("falta clave snake_case %s en %s", key, s)
		}
	}
}

func TestParseUserConfig(t *testing.T) {
	p, err := ParseProject([]byte(`{"name":"x","path":"y","user":{"enabled":false,"command":"npm run create-user"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.User.Enabled {
		t.Error("user.enabled=false debe respetarse")
	}
	if p.User.Command != "npm run create-user" {
		t.Errorf("user.command = %q", p.User.Command)
	}
}

func TestParseUserConfigPartial(t *testing.T) {
	// Solo command, sin enabled: enabled debe quedar en default (true).
	p, err := ParseProject([]byte(`{"name":"x","path":"y","user":{"command":"tsx scripts/create-user.ts"}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p.User.Enabled {
		t.Error("user.enabled default debe ser true cuando solo se define command")
	}
	if p.User.Command != "tsx scripts/create-user.ts" {
		t.Errorf("user.command = %q", p.User.Command)
	}
}

func TestParseLegacySynthesizesDev(t *testing.T) {
	// projects.json viejo sin envs → dev sintetizado desde top-level.
	p, err := ParseProject([]byte(fullJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ActiveEnv != "dev" {
		t.Errorf("active_env = %q, want dev", p.ActiveEnv)
	}
	dev, ok := p.Envs["dev"]
	if !ok {
		t.Fatalf("envs sin dev: %+v", p.Envs)
	}
	if dev.Server != p.Server || dev.Playwright != p.Playwright || dev.User != p.User {
		t.Errorf("dev debe copiar top-level:\ndev=%+v\ntop=%+v/%+v/%+v", dev, p.Server, p.Playwright, p.User)
	}
	if len(p.Validate()) != 0 {
		t.Errorf("legacy sintetizado debe validar: %v", p.Validate())
	}
}

func TestRoundTripWithEnvsStable(t *testing.T) {
	raw := `{"name":"x","path":"y","server":{"port":3000},"active_env":"staging","envs":{"dev":{"server":{"port":5173}},"staging":{"server":{"port":3000}}}}`
	p, err := ParseProject([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ActiveEnv != "staging" {
		t.Fatalf("active_env = %q", p.ActiveEnv)
	}
	if p.Envs["staging"].Server.Port != 3000 || p.Envs["dev"].Server.Port != 5173 {
		t.Fatalf("envs mal parseados: %+v", p.Envs)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	for _, key := range []string{`"active_env"`, `"envs"`, `"staging"`, `"dev"`} {
		if !strings.Contains(s, key) {
			t.Errorf("falta clave %s en %s", key, s)
		}
	}
	p2, err := ParseProject(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if p2.ActiveEnv != p.ActiveEnv || !reflect.DeepEqual(p2.Envs, p.Envs) {
		t.Errorf("round-trip envs inestable:\nin=%+v\nout=%+v", p.Envs, p2.Envs)
	}
	// EffectiveServer refleja el active.
	if p2.EffectiveServer().Port != 3000 {
		t.Errorf("EffectiveServer = %+v, want port 3000", p2.EffectiveServer())
	}
}

func TestValidateEnvNames(t *testing.T) {
	valid := Project{Name: "a", Path: "b", ActiveEnv: "dev",
		Envs: map[string]EnvConfig{"dev": {}, "staging-1": {}, "feat_x": {}}}
	if len(valid.Validate()) != 0 {
		t.Errorf("envs válidos no deben fallar: %v", valid.Validate())
	}
	cases := []struct {
		name string
		p    Project
	}{
		{"bad chars", Project{Name: "a", Path: "b", ActiveEnv: "Dev!",
			Envs: map[string]EnvConfig{"Dev!": {}}}},
		{"empty name", Project{Name: "a", Path: "b", ActiveEnv: "",
			Envs: map[string]EnvConfig{"dev": {}}}},
		{"too long", Project{Name: "a", Path: "b", ActiveEnv: strings.Repeat("a", 33),
			Envs: map[string]EnvConfig{strings.Repeat("a", 33): {}}}},
		{"active missing", Project{Name: "a", Path: "b", ActiveEnv: "prod",
			Envs: map[string]EnvConfig{"dev": {}}}},
		{"last deleted", Project{Name: "a", Path: "b", ActiveEnv: "dev",
			Envs: map[string]EnvConfig{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.p.Validate()) == 0 {
				t.Errorf("esperaba error de validación en %s", tc.name)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	p := Project{Name: "", Path: " "}
	errs := p.Validate()
	if len(errs) != 2 {
		t.Fatalf("esperaba 2 errores, got %v", errs)
	}
	ok := Project{Name: "a", Path: "b"}
	if len(ok.Validate()) != 0 {
		t.Error("proyecto válido no debe tener errores")
	}
}

func TestParseTabsConfig(t *testing.T) {
	// Ausente → vacío (comportamiento default: no-op).
	p, err := ParseProject([]byte(`{"name":"x","path":"y"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Tabs.Hidden) != 0 || len(p.Tabs.Order) != 0 {
		t.Errorf("tabs default = %+v, esperaba vacío", p.Tabs)
	}

	// Presente → round-trip respeta hidden + order.
	p, err = ParseProject([]byte(`{"name":"x","path":"y","tabs":{"hidden":["obscura"],"order":["scripts","deps","playwright"]}}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Tabs.Hidden) != 1 || p.Tabs.Hidden[0] != "obscura" {
		t.Errorf("hidden = %v", p.Tabs.Hidden)
	}
	if len(p.Tabs.Order) != 3 || p.Tabs.Order[2] != "playwright" {
		t.Errorf("order = %v", p.Tabs.Order)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	p2, err := ParseProject(out)
	if err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if !reflect.DeepEqual(p.Tabs, p2.Tabs) {
		t.Errorf("round-trip tabs inestable: %+v vs %+v", p.Tabs, p2.Tabs)
	}
}

func TestValidateTabs(t *testing.T) {
	cases := []struct {
		name string
		tabs TabsConfig
		want []string
	}{
		{"empty", TabsConfig{}, nil},
		{"valid", TabsConfig{Hidden: []string{"obscura"}, Order: []string{"scripts", "deps"}}, nil},
		{"hidden logs", TabsConfig{Hidden: []string{"logs"}}, []string{`tab "logs" cannot be hidden`}},
		{"unknown hidden", TabsConfig{Hidden: []string{"bogus"}}, []string{`unknown tab "bogus" in hidden`}},
		{"dup hidden", TabsConfig{Hidden: []string{"deps", "deps"}}, []string{`duplicate tab "deps" in hidden`}},
		{"unknown order", TabsConfig{Order: []string{"nope"}}, []string{`unknown tab "nope" in order`}},
		{"dup order", TabsConfig{Order: []string{"git", "git"}}, []string{`duplicate tab "git" in order`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Project{Name: "a", Path: "b", Tabs: tc.tabs}
			errs := p.Validate()
			if len(tc.want) == 0 {
				if len(errs) != 0 {
					t.Errorf("esperaba 0 errores, got %v", errs)
				}
				return
			}
			if len(errs) != len(tc.want) {
				t.Fatalf("esperaba %d errores %v, got %v", len(tc.want), tc.want, errs)
			}
			for i := range tc.want {
				if errs[i] != tc.want[i] {
					t.Errorf("error %d = %q, want %q", i, errs[i], tc.want[i])
				}
			}
		})
	}
}

func TestDefaultEnvFile(t *testing.T) {
	cases := map[string]string{
		"dev": ".env", "staging": ".env.staging", "prod": ".env.prod",
		"custom": ".env.custom", "feat-x": ".env.feat-x",
	}
	for name, want := range cases {
		if got := DefaultEnvFile(name); got != want {
			t.Errorf("DefaultEnvFile(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestRoundTripVarsPreserved(t *testing.T) {
	raw := `{"name":"x","path":"y","active_env":"staging","envs":{` +
		`"dev":{"vars":{"FOO":"bar"},"env_file":".env"},` +
		`"staging":{"server":{"port":3000},"vars":{"API_URL":"https://x","SPACED":"a b"}}}}`
	p, err := ParseProject([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Envs["dev"].Vars["FOO"] != "bar" {
		t.Errorf("dev vars = %+v", p.Envs["dev"].Vars)
	}
	if p.Envs["staging"].Vars["SPACED"] != "a b" {
		t.Errorf("staging vars = %+v", p.Envs["staging"].Vars)
	}
	// staging sin env_file explícito → default.
	if p.Envs["staging"].EnvFile != ".env.staging" {
		t.Errorf("staging env_file = %q", p.Envs["staging"].EnvFile)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	s := string(out)
	for _, key := range []string{`"vars"`, `"env_file"`, `"API_URL"`} {
		if !strings.Contains(s, key) {
			t.Errorf("falta clave snake_case %s en %s", key, s)
		}
	}
	p2, err := ParseProject(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(p2.Envs, p.Envs) {
		t.Errorf("round-trip vars inestable:\nin=%+v\nout=%+v", p.Envs, p2.Envs)
	}
}

func TestParseLegacyDefaultsVars(t *testing.T) {
	// JSON viejo sin vars/env_file → maps no-nil + defaults.
	p, err := ParseProject([]byte(fullJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dev := p.Envs["dev"]
	if dev.Vars == nil {
		t.Error("dev Vars debe ser no-nil")
	}
	if dev.EnvFile != ".env" {
		t.Errorf("dev env_file = %q, want .env", dev.EnvFile)
	}
}

func TestMaskedVars(t *testing.T) {
	vars := map[string]string{"PUBLIC": "hi", "TOKEN": "supersecret"}
	got := MaskedVars(vars, []string{"TOKEN"})
	if got["PUBLIC"] != "hi" {
		t.Errorf("PUBLIC = %q", got["PUBLIC"])
	}
	if got["TOKEN"] != MaskedValue {
		t.Errorf("TOKEN debe enmascararse, got %q", got["TOKEN"])
	}
	if vars["TOKEN"] != "supersecret" {
		t.Error("MaskedVars no debe mutar la entrada")
	}
	if !IsSecret([]string{"TOKEN"}, "TOKEN") || IsSecret([]string{"TOKEN"}, "PUBLIC") {
		t.Error("IsSecret inconsistente")
	}
}

func TestSecretsRoundTrip(t *testing.T) {
	raw := `{"name":"x","path":"y","active_env":"dev",` +
		`"envs":{"dev":{"vars":{"TOKEN":"abc","PLAIN":"x"},"secrets":["TOKEN"]}}}`
	p, err := ParseProject([]byte(raw))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !IsSecret(p.Envs["dev"].Secrets, "TOKEN") {
		t.Errorf("secrets = %v", p.Envs["dev"].Secrets)
	}
	if len(p.Validate()) != 0 {
		t.Errorf("validate = %v", p.Validate())
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"secrets"`) {
		t.Errorf("falta clave secrets en %s", out)
	}
	p2, err := ParseProject(out)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if !reflect.DeepEqual(p2.Envs["dev"].Secrets, []string{"TOKEN"}) {
		t.Errorf("secrets round-trip = %v", p2.Envs["dev"].Secrets)
	}
	if p2.Envs["dev"].Vars["TOKEN"] != "abc" {
		t.Errorf("vars TOKEN = %q", p2.Envs["dev"].Vars["TOKEN"])
	}
	// Enmascarado: el real nunca sale salvo reveal.
	if m := MaskedVars(p2.Envs["dev"].Vars, p2.Envs["dev"].Secrets); m["TOKEN"] != MaskedValue {
		t.Errorf("masked TOKEN = %q", m["TOKEN"])
	}
}

func TestSecretsDefaultsAndValidation(t *testing.T) {
	p, err := ParseProject([]byte(fullJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Envs["dev"].Secrets == nil {
		t.Error("secrets legacy debe ser no-nil ([])")
	}
	bad := Project{Name: "a", Path: "b", ActiveEnv: "dev",
		Envs: map[string]EnvConfig{"dev": {Secrets: []string{"bad-key"}}}}
	if len(bad.Validate()) == 0 {
		t.Error("secret inválido debe fallar Validate")
	}
}
