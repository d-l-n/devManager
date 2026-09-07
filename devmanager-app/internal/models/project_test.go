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
