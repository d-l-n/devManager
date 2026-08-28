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