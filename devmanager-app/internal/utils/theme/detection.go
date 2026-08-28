package theme

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// SystemThemeDetector detecta el tema del sistema operativo
type SystemThemeDetector struct{}

// NewSystemThemeDetector crea una nueva instancia del detector
func NewSystemThemeDetector() *SystemThemeDetector {
	return &SystemThemeDetector{}
}

// GetSystemTheme devuelve el tema actual del sistema ("light" o "dark")
func (d *SystemThemeDetector) GetSystemTheme() string {
	switch runtime.GOOS {
	case "windows":
		return d.getWindowsTheme()
	case "darwin":
		return d.getMacOSTheme()
	case "linux":
		return d.getLinuxTheme()
	default:
		return "dark" // fallback
	}
}

// getWindowsTheme detecta el tema en Windows usando PowerShell
func (d *SystemThemeDetector) getWindowsTheme() string {
	// Usamos PowerShell para leer el registro de Windows
	cmd := exec.Command("powershell", "-Command", 
		"Get-ItemProperty -Path 'HKCU:\\Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize' | Select-Object -ExpandProperty AppsUseLightTheme")
	
	output, err := cmd.Output()
	if err != nil {
		return "dark" // fallback en caso de error
	}
	
	// AppsUseLightTheme = 1 significa light theme, 0 significa dark theme
	themeValue := strings.TrimSpace(string(output))
	if themeValue == "1" {
		return "light"
	}
	return "dark"
}

// getMacOSTheme detecta el tema en macOS usando defaults
func (d *SystemThemeDetector) getMacOSTheme() string {
	cmd := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle")
	
	output, err := cmd.Output()
	if err != nil {
		// Si hay error, asumimos light theme (default en macOS)
		return "light"
	}
	
	// Si retorna "Dark", el tema es oscuro
	themeValue := strings.TrimSpace(string(output))
	if strings.Contains(themeValue, "Dark") {
		return "dark"
	}
	return "light"
}

// getLinuxTheme detecta el tema en Linux (más variable por desktop environment)
func (d *SystemThemeDetector) getLinuxTheme() string {
	// Intentar con GNOME/gsettings primero
	cmd := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "gtk-theme")
	output, err := cmd.Output()
	if err == nil {
		themeValue := strings.ToLower(strings.TrimSpace(string(output)))
		if strings.Contains(themeValue, "dark") {
			return "dark"
		}
		return "light"
	}
	
	// Fallback a variable de entorno
	if envTheme := strings.ToLower(os.Getenv("GTK_THEME")); envTheme != "" {
		if strings.Contains(envTheme, "dark") {
			return "dark"
		}
		return "light"
	}
	
	// Último fallback
	return "dark"
}