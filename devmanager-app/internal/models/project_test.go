package models

import (
	"encoding/json"
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
	if p.Name != p2.Name || p.Path != p2.Path || p.Server != p2.Server || p.Playwright != p2.Playwright || p.Pinned != p2.Pinned {
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
