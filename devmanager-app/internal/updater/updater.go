package updater

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/blang/semver/v4"
)

// UpdateInfo contains information about an available update
type UpdateInfo struct {
	Version         string    `json:"version"`
	ReleaseNotes    string    `json:"releaseNotes"`
	PublishedAt     time.Time `json:"publishedAt"`
	DownloadURL     string    `json:"downloadUrl"`
	SignatureURL    string    `json:"signatureUrl"`
	Prerelease      bool      `json:"prerelease"`
	MinimumVersion  string    `json:"minimumVersion"`
	ForceUpdate     bool      `json:"forceUpdate"`
	Platform        string    `json:"platform"`
	Architecture    string    `json:"architecture"`
	FileSize        int64     `json:"fileSize"`
	SHA256          string    `json:"sha256"`
}

// UpdateConfig contains configuration for the updater
type UpdateConfig struct {
	CurrentVersion    string        `json:"currentVersion"`
	CheckInterval     time.Duration `json:"checkInterval"`
	AutoDownload      bool          `json:"autoDownload"`
	AutoInstall       bool          `json:"autoInstall"`
	PrereleaseEnabled bool          `json:"prereleaseEnabled"`
	ProxyURL          string        `json:"proxyUrl"`
	UpdateServer      string        `json:"updateServer"`
	SignatureKey      string        `json:"signatureKey"`
}

// UpdateProgress represents update download progress
type UpdateProgress struct {
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Percentage      float64 `json:"percentage"`
	Speed           float64 `json:"speed"`
	ETA             int64   `json:"eta"`
}

// UpdateStatus represents the current update status
type UpdateStatus string

const (
	UpdateStatusIdle        UpdateStatus = "idle"
	UpdateStatusChecking    UpdateStatus = "checking"
	UpdateStatusAvailable   UpdateStatus = "available"
	UpdateStatusDownloading UpdateStatus = "downloading"
	UpdateStatusDownloaded  UpdateStatus = "downloaded"
	UpdateStatusInstalling  UpdateStatus = "installing"
	UpdateStatusInstalled   UpdateStatus = "installed"
	UpdateStatusFailed      UpdateStatus = "failed"
	UpdateStatusUpToDate    UpdateStatus = "up_to_date"
)

// Updater handles automatic updates
type Updater struct {
	config     *UpdateConfig
	status     UpdateStatus
	updateInfo *UpdateInfo
	progress   *UpdateProgress
	httpClient *http.Client
	ctx        context.Context
	cancel     context.CancelFunc
	events     chan UpdateEvent
}

// UpdateEvent represents an update event
type UpdateEvent struct {
	Type    UpdateStatus  `json:"type"`
	Message string        `json:"message"`
	Data    interface{}   `json:"data,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// NewUpdater creates a new updater instance
func NewUpdater(config *UpdateConfig) *Updater {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Updater{
		config: config,
		status: UpdateStatusIdle,
		progress: &UpdateProgress{},
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		ctx:    ctx,
		cancel: cancel,
		events: make(chan UpdateEvent, 100),
	}
}

// Start begins the update checking process
func (u *Updater) Start() {
	go u.updateLoop()
}

// Stop stops the updater
func (u *Updater) Stop() {
	u.cancel()
}

// CheckForUpdates checks for available updates
func (u *Updater) CheckForUpdates() (*UpdateInfo, error) {
	u.setStatus(UpdateStatusChecking, "Checking for updates...")
	
	updateInfo, err := u.fetchUpdateInfo()
	if err != nil {
		u.setStatus(UpdateStatusFailed, fmt.Sprintf("Failed to check for updates: %v", err))
		return nil, err
	}
	
	if updateInfo == nil {
		u.setStatus(UpdateStatusUpToDate, "You are using the latest version")
		return nil, nil
	}
	
	u.updateInfo = updateInfo
	u.setStatus(UpdateStatusAvailable, fmt.Sprintf("Update available: %s", updateInfo.Version))
	
	return updateInfo, nil
}

// DownloadUpdate downloads the available update
func (u *Updater) DownloadUpdate() error {
	if u.updateInfo == nil {
		return fmt.Errorf("no update available")
	}
	
	u.setStatus(UpdateStatusDownloading, "Downloading update...")
	
	// Create download directory
	downloadDir, err := u.getDownloadDir()
	if err != nil {
		return fmt.Errorf("failed to create download directory: %w", err)
	}
	
	// Download file
	filePath, err := u.downloadFile(u.updateInfo.DownloadURL, downloadDir)
	if err != nil {
		u.setStatus(UpdateStatusFailed, fmt.Sprintf("Download failed: %v", err))
		return fmt.Errorf("failed to download update: %w", err)
	}
	
	// Verify checksum
	if err := u.verifyChecksum(filePath, u.updateInfo.SHA256); err != nil {
		u.setStatus(UpdateStatusFailed, fmt.Sprintf("Checksum verification failed: %v", err))
		return fmt.Errorf("checksum verification failed: %w", err)
	}
	
	u.updateInfo.DownloadURL = filePath
	u.setStatus(UpdateStatusDownloaded, "Update downloaded successfully")
	
	return nil
}

// InstallUpdate installs the downloaded update
func (u *Updater) InstallUpdate() error {
	if u.updateInfo == nil || u.updateInfo.DownloadURL == "" {
		return fmt.Errorf("no update downloaded")
	}
	
	u.setStatus(UpdateStatusInstalling, "Installing update...")
	
	// Create installation script
	scriptPath, err := u.createInstallScript(u.updateInfo.DownloadURL)
	if err != nil {
		u.setStatus(UpdateStatusFailed, fmt.Sprintf("Failed to create install script: %v", err))
		return fmt.Errorf("failed to create install script: %w", err)
	}
	
	// Execute installation
	if err := u.executeInstallScript(scriptPath); err != nil {
		u.setStatus(UpdateStatusFailed, fmt.Sprintf("Installation failed: %v", err))
		return fmt.Errorf("failed to install update: %w", err)
	}
	
	u.setStatus(UpdateStatusInstalled, "Update installed successfully")
	
	return nil
}

// GetStatus returns the current update status
func (u *Updater) GetStatus() UpdateStatus {
	return u.status
}

// GetProgress returns the current update progress
func (u *Updater) GetProgress() *UpdateProgress {
	return u.progress
}

// GetUpdateInfo returns the available update info
func (u *Updater) GetUpdateInfo() *UpdateInfo {
	return u.updateInfo
}

// GetEvents returns the events channel
func (u *Updater) GetEvents() <-chan UpdateEvent {
	return u.events
}

// updateLoop runs the update checking loop
func (u *Updater) updateLoop() {
	ticker := time.NewTicker(u.config.CheckInterval)
	defer ticker.Stop()
	
	// Check immediately on start
	u.CheckForUpdates()
	
	for {
		select {
		case <-u.ctx.Done():
			return
		case <-ticker.C:
			u.CheckForUpdates()
		}
	}
}

// fetchUpdateInfo fetches update information from the server
func (u *Updater) fetchUpdateInfo() (*UpdateInfo, error) {
	url := fmt.Sprintf("%s/api/updates/latest", u.config.UpdateServer)
	
	// Add platform and architecture parameters
	platform := runtime.GOOS
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	
	req, err := http.NewRequestWithContext(u.ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	q := req.URL.Query()
	q.Add("platform", platform)
	q.Add("architecture", arch)
	q.Add("currentVersion", u.config.CurrentVersion)
	if !u.config.PrereleaseEnabled {
		q.Add("stable", "true")
	}
	req.URL.RawQuery = q.Encode()
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %d", resp.StatusCode)
	}
	
	var updateInfo UpdateInfo
	if err := json.NewDecoder(resp.Body).Decode(&updateInfo); err != nil {
		return nil, err
	}
	
	// Check if update is newer
	currentVersion, err := semver.Parse(u.config.CurrentVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid current version: %w", err)
	}
	
	updateVersion, err := semver.Parse(updateInfo.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid update version: %w", err)
	}
	
	if updateVersion.LTE(currentVersion) {
		return nil, nil // No update available
	}
	
	// Check minimum version requirement
	if updateInfo.MinimumVersion != "" {
		minVersion, err := semver.Parse(updateInfo.MinimumVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid minimum version: %w", err)
		}
		
		if currentVersion.LT(minVersion) {
			updateInfo.ForceUpdate = true
		}
	}
	
	return &updateInfo, nil
}

// downloadFile downloads a file with progress tracking
func (u *Updater) downloadFile(url, dir string) (string, error) {
	req, err := http.NewRequestWithContext(u.ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}
	
	// Extract filename from URL or Content-Disposition
	filename := u.extractFilename(url, resp.Header.Get("Content-Disposition"))
	filePath := filepath.Join(dir, filename)
	
	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	// Track progress
	u.progress.TotalBytes = resp.ContentLength
	u.progress.DownloadedBytes = 0
	startTime := time.Now()
	
	// Copy with progress tracking
	buffer := make([]byte, 32*1024) // 32KB buffer
	for {
		n, err := resp.Body.Read(buffer)
		if n > 0 {
			if _, writeErr := file.Write(buffer[:n]); writeErr != nil {
				return "", writeErr
			}
			
			u.progress.DownloadedBytes += int64(n)
			u.progress.Percentage = float64(u.progress.DownloadedBytes) / float64(u.progress.TotalBytes) * 100
			
			// Calculate speed and ETA
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				u.progress.Speed = float64(u.progress.DownloadedBytes) / elapsed
				if u.progress.Speed > 0 && u.progress.TotalBytes > 0 {
					remaining := float64(u.progress.TotalBytes-u.progress.DownloadedBytes) / u.progress.Speed
					u.progress.ETA = int64(remaining)
				}
			}
			
			// Send progress event
			u.sendEvent(UpdateStatusDownloading, fmt.Sprintf("Downloading... %.1f%%", u.progress.Percentage), u.progress)
		}
		
		if err == io.EOF {
			break
		}
		
		if err != nil {
			return "", err
		}
	}
	
	return filePath, nil
}

// extractFilename extracts filename from URL or Content-Disposition header
func (u *Updater) extractFilename(url, contentDisposition string) string {
	// Try Content-Disposition header first
	if contentDisposition != "" {
		parts := strings.Split(contentDisposition, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				filename := strings.Trim(part[9:], `"`)
				if filename != "" {
					return filename
				}
			}
		}
	}
	
	// Extract from URL
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		if filename != "" {
			return filename
		}
	}
	
	// Fallback
	return "update.bin"
}

// verifyChecksum verifies the SHA256 checksum of a file
func (u *Updater) verifyChecksum(filePath, expectedChecksum string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()
	
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return err
	}
	
	actualChecksum := fmt.Sprintf("%x", hasher.Sum(nil))
	if actualChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}
	
	return nil
}

// getDownloadDir returns the download directory
func (u *Updater) getDownloadDir() (string, error) {
	dir := filepath.Join(os.TempDir(), "devmanager-updates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// createInstallScript creates an installation script
func (u *Updater) createInstallScript(updateFile string) (string, error) {
	scriptPath := filepath.Join(os.TempDir(), "devmanager-install.sh")
	
	var script string
	switch runtime.GOOS {
	case "windows":
		scriptPath = filepath.Join(os.TempDir(), "devmanager-install.bat")
		script = fmt.Sprintf(`@echo off
echo Installing devManager update...
timeout /t 2 /nobreak >nul

:: Kill existing processes
taskkill /f /im devmanager.exe 2>nul
timeout /t 2 /nobreak >nul

:: Extract and install
"%s" /S /D="%s"

:: Start new version
start "" "%s\devmanager.exe"

:: Cleanup
del "%s"
del "%s"

echo Update completed!
`, updateFile, filepath.Dir(os.Args[0]), filepath.Dir(os.Args[0]), updateFile, scriptPath)
		
	case "linux", "darwin":
		script = fmt.Sprintf(`#!/bin/bash
set -e

echo "Installing devManager update..."

# Kill existing processes
pkill -f devmanager || true
sleep 2

# Extract and install
chmod +x "%s"
"%s" --target="%s" --mode=unattended

# Start new version
nohup "%s/devmanager" > /dev/null 2>&1 &

# Cleanup
rm -f "%s"
rm -f "$0"

echo "Update completed!"
`, updateFile, updateFile, filepath.Dir(os.Args[0]), filepath.Dir(os.Args[0]), updateFile)
		
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	
	// Write script
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return "", err
	}
	
	return scriptPath, nil
}

// executeInstallScript executes the installation script
func (u *Updater) executeInstallScript(scriptPath string) error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/C", scriptPath)
	default:
		cmd = exec.Command("bash", scriptPath)
	}
	
	// Detach from current process
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	
	return cmd.Start()
}

// setStatus updates the status and sends an event
func (u *Updater) setStatus(status UpdateStatus, message string) {
	u.status = status
	u.sendEvent(status, message, nil)
}

// sendEvent sends an update event
func (u *Updater) sendEvent(eventType UpdateStatus, message string, data interface{}) {
	event := UpdateEvent{
		Type:    eventType,
		Message: message,
		Data:    data,
	}
	
	select {
	case u.events <- event:
	default:
		// Channel is full, skip event
	}
}

// IsUpdateAvailable checks if an update is available
func (u *Updater) IsUpdateAvailable() bool {
	return u.updateInfo != nil
}

// IsForceUpdate checks if the update is forced
func (u *Updater) IsForceUpdate() bool {
	return u.updateInfo != nil && u.updateInfo.ForceUpdate
}

// GetUpdateSize returns the size of the update
func (u *Updater) GetUpdateSize() int64 {
	if u.updateInfo != nil {
		return u.updateInfo.FileSize
	}
	return 0
}

// SkipUpdate skips the current update
func (u *Updater) SkipUpdate() {
	u.updateInfo = nil
	u.setStatus(UpdateStatusIdle, "Update skipped")
}

// RetryUpdate retries the update process
func (u *Updater) RetryUpdate() error {
	u.updateInfo = nil
	_, err := u.CheckForUpdates()
	return err
}