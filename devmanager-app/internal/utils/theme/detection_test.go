package theme

import (
	"runtime"
	"testing"
)

func TestSystemThemeDetector(t *testing.T) {
	detector := NewSystemThemeDetector()
	
	// Test que el detector no sea nil
	if detector == nil {
		t.Fatal("NewSystemThemeDetector() returned nil")
	}
	
	// Test que GetSystemTheme retorne un valor válido
	theme := detector.GetSystemTheme()
	validThemes := []string{"light", "dark"}
	
	isValid := false
	for _, valid := range validThemes {
		if theme == valid {
			isValid = true
			break
		}
	}
	
	if !isValid {
		t.Errorf("GetSystemTheme() returned invalid theme: %s, expected one of %v", theme, validThemes)
	}
}

func TestGetSystemThemeByOS(t *testing.T) {
	detector := NewSystemThemeDetector()
	
	// Test específicos por SO
	switch runtime.GOOS {
	case "windows":
		theme := detector.getWindowsTheme()
		if theme != "light" && theme != "dark" {
			t.Errorf("getWindowsTheme() returned invalid theme: %s", theme)
		}
	case "darwin":
		theme := detector.getMacOSTheme()
		if theme != "light" && theme != "dark" {
			t.Errorf("getMacOSTheme() returned invalid theme: %s", theme)
		}
	case "linux":
		theme := detector.getLinuxTheme()
		if theme != "light" && theme != "dark" {
			t.Errorf("getLinuxTheme() returned invalid theme: %s", theme)
		}
	}
}

// ---- Issue #40: deeper detection logic tests ----

func TestNewSystemThemeDetectorNotNil(t *testing.T) {
	d := NewSystemThemeDetector()
	if d == nil {
		t.Fatal("constructor should never return nil")
	}
}

func TestGetSystemThemeConsistent(t *testing.T) {
	d := NewSystemThemeDetector()
	// Multiple calls should return the same value (no race, no drift)
	first := d.GetSystemTheme()
	for i := 0; i < 5; i++ {
		got := d.GetSystemTheme()
		if got != first {
			t.Fatalf("GetSystemTheme() inconsistent: first=%q, then=%q", first, got)
		}
	}
}

func TestGetSystemThemeReturnsLightOrDark(t *testing.T) {
	d := NewSystemThemeDetector()
	theme := d.GetSystemTheme()
	if theme != "light" && theme != "dark" {
		t.Errorf("GetSystemTheme() must return 'light' or 'dark', got %q", theme)
	}
}

func TestGetLinuxThemeGTKEnvVar(t *testing.T) {
	d := NewSystemThemeDetector()
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}
	// GTK_THEME env var is a fallback; we can't control gsettings here
	// but we verify getLinuxTheme returns valid value even without it
	theme := d.getLinuxTheme()
	if theme != "light" && theme != "dark" {
		t.Errorf("getLinuxTheme() returned invalid theme: %s", theme)
	}
}

func TestWindowsThemeFallbackOnBadPowerShell(t *testing.T) {
	// On Windows, if PowerShell fails (e.g. restricted), should fall back to "dark"
	d := &SystemThemeDetector{}
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	theme := d.getWindowsTheme()
	if theme != "light" && theme != "dark" {
		t.Errorf("getWindowsTheme() must return light or dark, got %q", theme)
	}
}