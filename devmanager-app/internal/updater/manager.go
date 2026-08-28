package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Manager manages the update system
type Manager struct {
	updater     *Updater
	config      *UpdateConfig
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	eventLogger *log.Logger
}

// NewManager creates a new update manager
func NewManager(ctx context.Context, config *UpdateConfig) *Manager {
	managerCtx, cancel := context.WithCancel(ctx)
	
	return &Manager{
		updater: NewUpdater(config),
		config:  config,
		ctx:     managerCtx,
		cancel:  cancel,
	}
}

// Start starts the update manager
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updater.Start()
	go m.eventLoop()
}

// Stop stops the update manager
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updater.Stop()
	m.cancel()
}

// CheckForUpdates checks for available updates
func (m *Manager) CheckForUpdates() (*UpdateInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.CheckForUpdates()
}

// DownloadUpdate downloads the available update
func (m *Manager) DownloadUpdate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.DownloadUpdate()
}

// InstallUpdate installs the downloaded update
func (m *Manager) InstallUpdate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.InstallUpdate()
}

// GetStatus returns the current update status
func (m *Manager) GetStatus() UpdateStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.GetStatus()
}

// GetProgress returns the current update progress
func (m *Manager) GetProgress() *UpdateProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.GetProgress()
}

// GetUpdateInfo returns the available update info
func (m *Manager) GetUpdateInfo() *UpdateInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.GetUpdateInfo()
}

// IsUpdateAvailable checks if an update is available
func (m *Manager) IsUpdateAvailable() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.IsUpdateAvailable()
}

// IsForceUpdate checks if the update is forced
func (m *Manager) IsForceUpdate() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.IsForceUpdate()
}

// SkipUpdate skips the current update
func (m *Manager) SkipUpdate() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updater.SkipUpdate()
}

// RetryUpdate retries the update process
func (m *Manager) RetryUpdate() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.updater.RetryUpdate()
}

// UpdateConfig updates the configuration
func (m *Manager) UpdateConfig(config *UpdateConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	// Restart updater with new config
	m.updater.Stop()
	m.updater = NewUpdater(config)
	m.updater.Start()
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *UpdateConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent modification
	configCopy := *m.config
	return &configCopy
}

// eventLoop processes update events
func (m *Manager) eventLoop() {
	events := m.updater.GetEvents()
	
	for {
		select {
		case <-m.ctx.Done():
			return
		case event := <-events:
			m.handleEvent(event)
		}
	}
}

// handleEvent handles update events
func (m *Manager) handleEvent(event UpdateEvent) {
	// Log event
	if m.eventLogger != nil {
		m.eventLogger.Printf("Update event: %s - %s", event.Type, event.Message)
	}

	// Handle different event types
	switch event.Type {
	case UpdateStatusAvailable:
		m.handleUpdateAvailable(event)
	case UpdateStatusDownloading:
		m.handleUpdateDownloading(event)
	case UpdateStatusDownloaded:
		m.handleUpdateDownloaded(event)
	case UpdateStatusInstalled:
		m.handleUpdateInstalled(event)
	case UpdateStatusFailed:
		m.handleUpdateFailed(event)
	}
}

// handleUpdateAvailable handles update available event
func (m *Manager) handleUpdateAvailable(event UpdateEvent) {
	// Show notification to user
	if updateInfo, ok := event.Data.(*UpdateInfo); ok {
		title := "Update Available"
		message := fmt.Sprintf("Version %s is available", updateInfo.Version)
		
		runtime.EventsEmit(m.ctx, "update:available", map[string]interface{}{
			"version":      updateInfo.Version,
			"releaseNotes": updateInfo.ReleaseNotes,
			"forceUpdate":  updateInfo.ForceUpdate,
		})

		// Show system notification
		runtime.ShowNotification(m.ctx, title, message)
	}
}

// handleUpdateDownloading handles update downloading event
func (m *Manager) handleUpdateDownloading(event UpdateEvent) {
	// Emit progress event
	if progress, ok := event.Data.(*UpdateProgress); ok {
		runtime.EventsEmit(m.ctx, "update:progress", map[string]interface{}{
			"percentage": progress.Percentage,
			"speed":      progress.Speed,
			"eta":        progress.ETA,
		})
	}
}

// handleUpdateDownloaded handles update downloaded event
func (m *Manager) handleUpdateDownloaded(event UpdateEvent) {
	runtime.EventsEmit(m.ctx, "update:downloaded", map[string]interface{}{
		"message": event.Message,
	})

	// Auto-install if enabled
	if m.config.AutoInstall {
		go func() {
			if err := m.InstallUpdate(); err != nil {
				runtime.EventsEmit(m.ctx, "update:error", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()
	}
}

// handleUpdateInstalled handles update installed event
func (m *Manager) handleUpdateInstalled(event UpdateEvent) {
	runtime.EventsEmit(m.ctx, "update:installed", map[string]interface{}{
		"message": event.Message,
	})

	// Show notification
	runtime.ShowNotification(m.ctx, "Update Installed", "The application has been updated successfully")
}

// handleUpdateFailed handles update failed event
func (m *Manager) handleUpdateFailed(event UpdateEvent) {
	runtime.EventsEmit(m.ctx, "update:error", map[string]interface{}{
		"error":   event.Error,
		"message": event.Message,
	})

	// Show error notification
	runtime.ShowNotification(m.ctx, "Update Failed", event.Message)
}

// SetEventLogger sets the event logger
func (m *Manager) SetEventLogger(logger *log.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.eventLogger = logger
}

// GetUpdateHistory returns update history (placeholder for future implementation)
func (m *Manager) GetUpdateHistory() []UpdateInfo {
	// TODO: Implement update history persistence
	return []UpdateInfo{}
}

// ExportUpdateData exports update data for backup/restore
func (m *Manager) ExportUpdateData() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := map[string]interface{}{
		"config":      m.config,
		"status":      m.updater.GetStatus(),
		"updateInfo":  m.updater.GetUpdateInfo(),
		"progress":    m.updater.GetProgress(),
	}

	return json.MarshalIndent(data, "", "  ")
}

// ImportUpdateData imports update data from backup/restore
func (m *Manager) ImportUpdateData(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var importData map[string]interface{}
	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("failed to unmarshal update data: %w", err)
	}

	// Import config if present
	if configData, ok := importData["config"]; ok {
		configBytes, err := json.Marshal(configData)
		if err != nil {
			return fmt.Errorf("failed to marshal config data: %w", err)
		}

		var config UpdateConfig
		if err := json.Unmarshal(configBytes, &config); err != nil {
			return fmt.Errorf("failed to unmarshal config: %w", err)
		}

		m.config = &config
	}

	return nil
}

// ValidateUpdate validates update before installation
func (m *Manager) ValidateUpdate(updateInfo *UpdateInfo) error {
	if updateInfo == nil {
		return fmt.Errorf("update info is nil")
	}

	if updateInfo.Version == "" {
		return fmt.Errorf("update version is empty")
	}

	if updateInfo.DownloadURL == "" {
		return fmt.Errorf("download URL is empty")
	}

	// Validate version format
	currentVersion, err := semver.Parse(m.config.CurrentVersion)
	if err != nil {
		return fmt.Errorf("invalid current version: %w", err)
	}

	updateVersion, err := semver.Parse(updateInfo.Version)
	if err != nil {
		return fmt.Errorf("invalid update version: %w", err)
	}

	if updateVersion.LTE(currentVersion) {
		return fmt.Errorf("update version is not newer than current version")
	}

	// Check minimum version requirement
	if updateInfo.MinimumVersion != "" {
		minVersion, err := semver.Parse(updateInfo.MinimumVersion)
		if err != nil {
			return fmt.Errorf("invalid minimum version: %w", err)
		}

		if currentVersion.LT(minVersion) {
			updateInfo.ForceUpdate = true
		}
	}

	return nil
}

// ScheduleUpdate schedules an update for later
func (m *Manager) ScheduleUpdate(delay time.Duration) error {
	go func() {
		select {
		case <-m.ctx.Done():
			return
		case <-time.After(delay):
			if err := m.CheckForUpdates(); err != nil {
				runtime.EventsEmit(m.ctx, "update:error", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}()

	return nil
}

// CancelScheduledUpdate cancels any scheduled update
func (m *Manager) CancelScheduledUpdate() {
	m.cancel()
	// Create new context
	m.ctx, m.cancel = context.WithCancel(context.Background())
}