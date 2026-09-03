package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/config"
)

// ---- Settings bindings ----

func (a *App) GetSettings() config.Settings {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings
}

// SetSetting valida key/value, persiste y emite "settings:changed" con el
// valor normalizado. Inválido → []string{msg} sin guardar.
func (a *App) SetSetting(key, value string) []string {
	s := a.GetSettings()
	normalized := ""
	switch key {
	case "style":
		switch value {
		case "standard", "brutalist", "glassmorphism", "retro", "dracula":
			s.Style = value
			normalized = value
		default:
			return []string{"Invalid style value (expected standard, brutalist, glassmorphism, retro or dracula)"}
		}
	case "theme":
		switch value {
		case "light", "dark", "oled", "system":
			s.Theme = value
			normalized = value
		default:
			return []string{"Invalid theme value (expected light, dark, oled or system)"}
		}
	case "monitor_polling":
		b, err := parseStrictBool(value)
		if err != nil {
			return []string{"Invalid monitor_polling value (expected true or false)"}
		}
		s.MonitorPolling = b
		normalized = strconv.FormatBool(b)
	case "toasts_enabled":
		b, err := parseStrictBool(value)
		if err != nil {
			return []string{"Invalid toasts_enabled value (expected true or false)"}
		}
		s.ToastsEnabled = b
		normalized = strconv.FormatBool(b)
	case "accent_global":
		b, err := parseStrictBool(value)
		if err != nil {
			return []string{"Invalid accent_global value (expected true or false)"}
		}
		s.AccentGlobal = b
		normalized = strconv.FormatBool(b)
	case "accent_global_color":
		if value != "" && !isValidHexColor(value) {
			return []string{"Invalid accent_global_color value (expected hex color like #ff5500)"}
		}
		s.AccentGlobalColor = value
		normalized = value
	default:
		// Per-style accent overrides: accent_override.<style>
		if strings.HasPrefix(key, "accent_override.") {
			style := strings.TrimPrefix(key, "accent_override.")
			if !validAccentStyle(style) {
				return []string{"Invalid accent override style: " + style}
			}
			if value == "" || value == "default" {
				delete(s.AccentOverrides, style)
				normalized = ""
			} else {
				if !isValidHexColor(value) {
					return []string{"Invalid accent color value (expected hex color like #ff5500)"}
				}
				if s.AccentOverrides == nil {
					s.AccentOverrides = make(map[string]string)
				}
				s.AccentOverrides[style] = value
				normalized = value
			}
		} else {
			return []string{"Unknown setting key: " + key}
		}
	}

	if err := config.SaveSettings(a.settingsPath, s); err != nil {
		return []string{err.Error()}
	}
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	wails.EventsEmit(a.ctx, "settings:changed", map[string]string{
		"key": key, "value": normalized,
	})
	return nil
}

func parseStrictBool(v string) (bool, error) {
	switch v {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid bool %q", v)
	}
}

// isValidHexColor validates a CSS hex color string (#RGB, #RRGGBB, #RRGGBBAA).
func isValidHexColor(s string) bool {
	if len(s) < 4 || s[0] != '#' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	l := len(s)
	return l == 4 || l == 5 || l == 7 || l == 9
}

// validAccentStyle checks if the style name supports accent overrides.
func validAccentStyle(s string) bool {
	switch s {
	case "standard", "brutalist", "glassmorphism", "retro", "dracula":
		return true
	}
	return false
}

// ---- Updater (Issue #58) ----

// Version is set at build time via ldflags: -ldflags "-X main.Version=v2.0.1"
var Version = "dev"

// GetVersion returns the current application version string.
func (a *App) GetVersion() string {
	return Version
}

// UpdateInfo holds the result of a GitHub release check.
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	UpdateURL      string `json:"updateUrl"`
	DownloadURL    string `json:"downloadUrl"`
	ReleaseNotes   string `json:"releaseNotes"`
	IsUpToDate     bool   `json:"isUpToDate"`
	Error          string `json:"error,omitempty"`
}

// CheckForUpdate queries GitHub Releases API for the latest release and
// compares it against the current Version. Returns UpdateInfo for the
// frontend to display.
func (a *App) CheckForUpdate() UpdateInfo {
	const repoOwner = "d-l-n"
	const repoName = "devManager"
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repoOwner, repoName)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return UpdateInfo{CurrentVersion: Version, Error: err.Error()}
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := client.Do(req)
	if err != nil {
		return UpdateInfo{CurrentVersion: Version, Error: err.Error()}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return UpdateInfo{CurrentVersion: Version, IsUpToDate: true}
	}
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return UpdateInfo{CurrentVersion: Version, Error: "GitHub API rate limited. Try again later."}
	}
	if resp.StatusCode != http.StatusOK {
		return UpdateInfo{CurrentVersion: Version, Error: fmt.Sprintf("GitHub API returned %d", resp.StatusCode)}
	}

	var release struct {
		TagName    string `json:"tag_name"`
		HTMLURL    string `json:"html_url"`
		Body       string `json:"body"`
		Assets     []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return UpdateInfo{CurrentVersion: Version, Error: err.Error()}
	}

	latestVersion := release.TagName
	isUpToDate := compareVersions(Version, latestVersion) >= 0

	// Find platform-specific download URL
	downloadURL := ""
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if runtime.GOOS == "windows" && strings.HasSuffix(name, ".exe") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
		if runtime.GOOS == "darwin" && (strings.HasSuffix(name, ".dmg") || strings.HasSuffix(name, ".app.zip")) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
		if runtime.GOOS == "linux" && strings.HasSuffix(name, ".appimage") {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}
	// Fallback: first asset
	if downloadURL == "" && len(release.Assets) > 0 {
		downloadURL = release.Assets[0].BrowserDownloadURL
	}

	return UpdateInfo{
		CurrentVersion: Version,
		LatestVersion:  latestVersion,
		UpdateURL:      release.HTMLURL,
		DownloadURL:    downloadURL,
		ReleaseNotes:   release.Body,
		IsUpToDate:     isUpToDate,
	}
}

// compareVersions compares two semver strings (vX.Y.Z). Returns:
// >0 if a > b, 0 if equal, <0 if a < b.
// Strips leading "v" if present. Non-numeric segments are compared lexically.
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		na, _ := strconv.Atoi(looseGet(partsA, i))
		nb, _ := strconv.Atoi(looseGet(partsB, i))
		if na != nb {
			return na - nb
		}
	}
	return 0
}

func looseGet(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}
	return "0"
}
