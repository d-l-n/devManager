package updater

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNewUpdater(t *testing.T) {
	config := &UpdateConfig{
		CurrentVersion:    "1.0.0",
		CheckInterval:     time.Hour,
		AutoDownload:      true,
		AutoInstall:       false,
		PrereleaseEnabled: false,
		UpdateServer:      "https://api.devmanager.com",
	}

	updater := NewUpdater(config)
	if updater == nil {
		t.Fatal("NewUpdater returned nil")
	}

	if updater.config != config {
		t.Error("Config not set correctly")
	}

	if updater.status != UpdateStatusIdle {
		t.Errorf("Expected status %s, got %s", UpdateStatusIdle, updater.status)
	}
}

func TestFetchUpdateInfo(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check query parameters
		platform := r.URL.Query().Get("platform")
		arch := r.URL.Query().Get("architecture")
		currentVersion := r.URL.Query().Get("currentVersion")

		if platform != runtime.GOOS {
			t.Errorf("Expected platform %s, got %s", runtime.GOOS, platform)
		}

		if arch != runtime.GOARCH {
			t.Errorf("Expected architecture %s, got %s", runtime.GOARCH, arch)
		}

		if currentVersion != "1.0.0" {
			t.Errorf("Expected current version 1.0.0, got %s", currentVersion)
		}

		// Return update info
		updateInfo := UpdateInfo{
			Version:        "1.1.0",
			ReleaseNotes:   "Test release notes",
			PublishedAt:    time.Now(),
			DownloadURL:    "http://example.com/download",
			Prerelease:     false,
			Platform:       platform,
			Architecture:   arch,
			FileSize:       1024000,
			SHA256:         "test-sha256",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateInfo)
	}))
	defer server.Close()

	config := &UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateServer:   server.URL,
	}

	updater := NewUpdater(config)
	updateInfo, err := updater.fetchUpdateInfo()
	if err != nil {
		t.Fatalf("fetchUpdateInfo failed: %v", err)
	}

	if updateInfo == nil {
		t.Fatal("fetchUpdateInfo returned nil")
	}

	if updateInfo.Version != "1.1.0" {
		t.Errorf("Expected version 1.1.0, got %s", updateInfo.Version)
	}
}

func TestFetchUpdateInfoNoUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return same version (no update)
		updateInfo := UpdateInfo{
			Version: "1.0.0",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updateInfo)
	}))
	defer server.Close()

	config := &UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateServer:   server.URL,
	}

	updater := NewUpdater(config)
	updateInfo, err := updater.fetchUpdateInfo()
	if err != nil {
		t.Fatalf("fetchUpdateInfo failed: %v", err)
	}

	if updateInfo != nil {
		t.Error("Expected no update, but got update info")
	}
}

func TestExtractFilename(t *testing.T) {
	updater := &Updater{}

	tests := []struct {
		name             string
		url              string
		contentDisposition string
		expected         string
	}{
		{
			name:              "URL with filename",
			url:               "http://example.com/devmanager-1.0.0.exe",
			contentDisposition: "",
			expected:          "devmanager-1.0.0.exe",
		},
		{
			name:              "Content-Disposition header",
			url:               "http://example.com/download",
			contentDisposition: "attachment; filename=\"devmanager-1.0.0.dmg\"",
			expected:          "devmanager-1.0.0.dmg",
		},
		{
			name:              "No filename",
			url:               "http://example.com/",
			contentDisposition: "",
			expected:          "update.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := updater.extractFilename(tt.url, tt.contentDisposition)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetDownloadDir(t *testing.T) {
	updater := &Updater{}
	dir, err := updater.getDownloadDir()
	if err != nil {
		t.Fatalf("getDownloadDir failed: %v", err)
	}

	expected := filepath.Join(os.TempDir(), "devmanager-updates")
	if dir != expected {
		t.Errorf("Expected %s, got %s", expected, dir)
	}

	// Check if directory was created
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("Download directory was not created")
	}
}

func TestUpdateStatus(t *testing.T) {
	updater := &Updater{}

	// Test initial status
	if updater.GetStatus() != UpdateStatusIdle {
		t.Errorf("Expected initial status %s, got %s", UpdateStatusIdle, updater.GetStatus())
	}

	// Test update available
	updateInfo := &UpdateInfo{
		Version: "1.1.0",
	}
	updater.updateInfo = updateInfo

	if !updater.IsUpdateAvailable() {
		t.Error("Expected update to be available")
	}

	// Test force update
	updateInfo.ForceUpdate = true
	if !updater.IsForceUpdate() {
		t.Error("Expected force update")
	}

	// Test update size
	updateInfo.FileSize = 1024000
	if updater.GetUpdateSize() != 1024000 {
		t.Errorf("Expected update size 1024000, got %d", updater.GetUpdateSize())
	}

	// Test skip update
	updater.SkipUpdate()
	if updater.IsUpdateAvailable() {
		t.Error("Expected no update after skip")
	}

	if updater.GetStatus() != UpdateStatusIdle {
		t.Errorf("Expected status %s after skip, got %s", UpdateStatusIdle, updater.GetStatus())
	}
}

func TestEvents(t *testing.T) {
	config := &UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateServer:   "http://example.com",
	}

	updater := NewUpdater(config)
	events := updater.GetEvents()

	// Send test event
	updater.sendEvent(UpdateStatusChecking, "Test message", nil)

	// Receive event
	select {
	case event := <-events:
		if event.Type != UpdateStatusChecking {
			t.Errorf("Expected event type %s, got %s", UpdateStatusChecking, event.Type)
		}
		if event.Message != "Test message" {
			t.Errorf("Expected message 'Test message', got '%s'", event.Message)
		}
	case <-time.After(time.Second):
		t.Error("Did not receive event within timeout")
	}
}

func TestContextCancellation(t *testing.T) {
	config := &UpdateConfig{
		CurrentVersion: "1.0.0",
		UpdateServer:   "http://example.com",
	}

	updater := NewUpdater(config)
	updater.Start()

	// Cancel context
	updater.Stop()

	// Give some time for goroutine to stop
	time.Sleep(100 * time.Millisecond)

	// The update loop should have stopped
	// This is a basic test - in a real scenario, you might want to add
	// more sophisticated testing for goroutine lifecycle
}

func TestProgressTracking(t *testing.T) {
	updater := &Updater{}
	progress := updater.GetProgress()

	if progress == nil {
		t.Error("Expected progress to be initialized")
	}

	// Test progress update
	updater.progress.DownloadedBytes = 500
	updater.progress.TotalBytes = 1000
	updater.progress.Percentage = 50.0

	progress = updater.GetProgress()
	if progress.DownloadedBytes != 500 {
		t.Errorf("Expected downloaded bytes 500, got %d", progress.DownloadedBytes)
	}

	if progress.TotalBytes != 1000 {
		t.Errorf("Expected total bytes 1000, got %d", progress.TotalBytes)
	}

	if progress.Percentage != 50.0 {
		t.Errorf("Expected percentage 50.0, got %f", progress.Percentage)
	}
}

// Benchmark tests
func BenchmarkNewUpdater(b *testing.B) {
	config := &UpdateConfig{
		CurrentVersion: "1.0.0",
		CheckInterval:  time.Hour,
	}

	for i := 0; i < b.N; i++ {
		NewUpdater(config)
	}
}

func BenchmarkExtractFilename(b *testing.B) {
	updater := &Updater{}
	url := "http://example.com/devmanager-1.0.0.exe"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		updater.extractFilename(url, "")
	}
}