package detection

import (
	"os"
	"path/filepath"
	"testing"
)

func tempProject(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDetectProjectConfigDefaultsWhenMissing(t *testing.T) {
	cfg := DetectProjectConfig("", nil)
	if cfg.ServerCommand != "npm run dev" || cfg.Port != 5173 {
		t.Fatalf("defaults wrong: %+v", cfg)
	}

	cfg = DetectProjectConfig("/path/to/nonexistent", nil)
	if cfg.ServerCommand != "npm run dev" || cfg.Port != 5173 {
		t.Fatalf("nonexistent path should keep defaults: %+v", cfg)
	}
}

func TestDetectProjectConfigVite(t *testing.T) {
	dir := tempProject(t, map[string]string{
		"package.json": `{"name":"my-app","scripts":{"dev":"vite"},"devDependencies":{"vite":"^5"}}`,
	})
	cfg := DetectProjectConfig(dir, nil)
	if cfg.Name != "My App" {
		t.Errorf("name = %q, want 'My App'", cfg.Name)
	}
	if cfg.ServerCommand != "npm run dev" {
		t.Errorf("command = %q, want 'npm run dev'", cfg.ServerCommand)
	}
	if cfg.Port != 5173 {
		t.Errorf("port = %d, want 5173", cfg.Port)
	}
	if cfg.URL != "http://localhost:5173" {
		t.Errorf("url = %q", cfg.URL)
	}
}

func TestDetectProjectConfigNextFramework(t *testing.T) {
	dir := tempProject(t, map[string]string{
		"package.json": `{"name":"blog","scripts":{"dev":"next dev"},"dependencies":{"next":"^14"}}`,
	})
	cfg := DetectProjectConfig(dir, nil)
	if cfg.Port != 3000 {
		t.Errorf("port = %d, want 3000", cfg.Port)
	}
}

func TestDetectProjectConfigPlaywrightDetection(t *testing.T) {
	dir := tempProject(t, map[string]string{
		"package.json":         `{"devDependencies":{"@playwright/test":"^1"}}`,
		"playwright.config.ts": "",
	})
	cfg := DetectProjectConfig(dir, nil)
	if !cfg.PlaywrightEnabled {
		t.Error("playwright_enabled should be true")
	}
}

func TestDetectProjectConfigEnvPortOverride(t *testing.T) {
	dir := tempProject(t, map[string]string{
		"package.json": `{"scripts":{"dev":"vite"},"devDependencies":{"vite":"^5"}}`,
		".env":         "PORT=4242\n",
	})
	cfg := DetectProjectConfig(dir, nil)
	if cfg.Port != 4242 {
		t.Errorf("port = %d, want 4242", cfg.Port)
	}
	if cfg.URL != "http://localhost:4242" {
		t.Errorf("url = %q", cfg.URL)
	}
}

func TestDetectProjectConfigAvoidsCollision(t *testing.T) {
	dir := tempProject(t, map[string]string{
		"package.json": `{"scripts":{"dev":"vite"},"devDependencies":{"vite":"^5"}}`,
	})
	cfg := DetectProjectConfig(dir, []int{5173, 5174})
	if cfg.Port != 5175 {
		t.Errorf("port = %d, want 5175 (collision avoided)", cfg.Port)
	}
	if cfg.URL != "http://localhost:5175" {
		t.Errorf("url = %q", cfg.URL)
	}
}

func TestDetectProjectConfigPythonDjango(t *testing.T) {
	dir := tempProject(t, map[string]string{
		"manage.py": "#!/usr/bin/env python",
	})
	cfg := DetectProjectConfig(dir, nil)
	if cfg.ServerCommand != "python manage.py runserver" {
		t.Errorf("command = %q", cfg.ServerCommand)
	}
	if cfg.Port != 8000 {
		t.Errorf("port = %d, want 8000", cfg.Port)
	}
}
