package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSettingsMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	got := LoadSettings(path)
	want := DefaultSettings()
	if got != want {
		t.Errorf("archivo ausente debe dar defaults: got %+v want %+v", got, want)
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	custom := Settings{Theme: "oled", MonitorPolling: false, ToastsEnabled: false}
	if err := SaveSettings(path, custom); err != nil {
		t.Fatalf("save fall├│: %v", err)
	}
	got := LoadSettings(path)
	if got != custom {
		t.Errorf("round-trip: got %+v want %+v", got, custom)
	}
}

func TestLoadSettingsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{no es json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings(path)
	want := DefaultSettings()
	if got != want {
		t.Errorf("archivo corrupto debe dar defaults: got %+v want %+v", got, want)
	}
}

func TestLoadSettingsInvalidTheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	raw, err := json.Marshal(map[string]any{"theme": "neon", "monitor_polling": true, "toasts_enabled": true})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings(path)
	if got.Theme != "dark" {
		t.Errorf("tema inv├ílido debe sanear a dark, got %q", got.Theme)
	}
}

func TestSettingsPersistAcrossInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")

	first := Settings{Theme: "light", MonitorPolling: true, ToastsEnabled: false}
	if err := SaveSettings(path, first); err != nil {
		t.Fatalf("save fall├│: %v", err)
	}
	second := LoadSettings(path)
	if second != first {
		t.Errorf("persistencia entre instancias: got %+v want %+v", second, first)
	}
}

func TestSaveSettingsCreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "settings.json")
	if err := SaveSettings(path, DefaultSettings()); err != nil {
		t.Fatalf("save con dirs anidados fall├│: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("archivo no debe quedar vac├¡o")
	}
}

func TestSaveSettingsFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := SaveSettings(path, DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"theme\": \"dark\",\n  \"monitor_polling\": true,\n  \"toasts_enabled\": true\n}"
	if string(data) != want {
		t.Errorf("formato MarshalIndent 2 espacios:\ngot:\n%s\nwant:\n%s", data, want)
	}
}
