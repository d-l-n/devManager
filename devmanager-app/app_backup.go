package main

import (
	"path/filepath"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/backup"
	"github.com/d-l-n/devmanager/internal/config"
)

// ---- Backup bindings (Issue #71) ----

// BackupResult es la respuesta común de Create/Validate (ok + mensaje para toast).
type BackupResult struct {
	OK       bool   `json:"ok"`
	Filename string `json:"filename,omitempty"`
	Message  string `json:"message,omitempty"`
}

// backupDir es %APPDATA%/devManager/backups (junto a settings.json): estable
// entre installs, no depende de dónde viva el exe (projects.json sí).
func (a *App) backupDir() string {
	return filepath.Join(filepath.Dir(a.settingsPath), "backups")
}

// CreateBackup hace un backup manual inmediato y aplica retención.
func (a *App) CreateBackup() BackupResult {
	name, err := a.runBackupNow()
	if err != nil {
		return BackupResult{OK: false, Message: err.Error()}
	}
	return BackupResult{OK: true, Filename: name, Message: "Backup created: " + name}
}

// runBackupNow crea un backup + aplica retención. Serializado con el scheduler
// vía backupMu (nunca dos backups escribiendo a la vez).
func (a *App) runBackupNow() (string, error) {
	a.backupMu.Lock()
	defer a.backupMu.Unlock()

	name, err := backup.CreateBackup(a.backupDir(), a.configPath, a.settingsPath, Version, time.Now())
	if err != nil {
		return "", err
	}
	if s := a.GetSettings(); s.BackupRetention > 0 {
		_, _ = backup.ApplyRetention(a.backupDir(), s.BackupRetention)
	}
	return name, nil
}

// ListBackups devuelve el catálogo ordenado newest→oldest (con Valid/Error).
func (a *App) ListBackups() []backup.Entry {
	entries := backup.List(a.backupDir())
	if entries == nil {
		return []backup.Entry{}
	}
	return entries
}

// ValidateBackup re-valida un archivo concreto del catálogo.
func (a *App) ValidateBackup(filename string) BackupResult {
	if !backup.ValidFilename(filename) {
		return BackupResult{OK: false, Message: "Invalid backup filename"}
	}
	entry, err := backup.ValidateFile(filepath.Join(a.backupDir(), filename))
	if err != nil {
		return BackupResult{OK: false, Filename: filename, Message: entry.Error}
	}
	return BackupResult{OK: true, Filename: filename, Message: "Backup is valid"}
}

// RestoreBackup valida y restaura projects.json/settings.json desde un backup.
// Antes de pisar crea <archivo>.pre-restore. Al terminar recarga el estado en
// vivo: cfg.Load() re-emite projects:changed y reloadSettings re-aplica settings.
func (a *App) RestoreBackup(filename string) []string {
	if _, err := backup.Restore(a.backupDir(), filename, a.configPath, a.settingsPath); err != nil {
		return []string{err.Error()}
	}
	a.cfg.Load() // re-emite projects:changed para el frontend
	a.reloadSettingsAfterRestore()
	return nil
}

// reloadSettingsAfterRestore relee settings.json restaurado y emite
// "settings:reloaded" para que el frontend re-aplique theme/style/accents
// sin persistir de nuevo (evitar bucle de eco).
func (a *App) reloadSettingsAfterRestore() {
	s := config.LoadSettings(a.settingsPath)
	a.mu.Lock()
	a.settings = s
	a.mu.Unlock()
	if a.ctx != nil {
		wails.EventsEmit(a.ctx, "settings:reloaded")
	}
}

// OpenBackupsFolder abre el directorio de backups en el explorador del sistema.
func (a *App) OpenBackupsFolder() {
	openWithRundll32(a.backupDir())
}

// startBackupScheduler lanza la goroutine de backups automáticos. Granularidad
// de 15 min: un backup "hourly" corre hasta 15 min tarde (aceptable en desktop).
func (a *App) startBackupScheduler() {
	go func() {
		a.runScheduledBackup() // chequeo inicial: cubre la app cerrada X horas
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				a.runScheduledBackup()
			}
		}
	}()
}

// runScheduledBackup corre el backup si la frecuencia está activa y el más
// reciente del catálogo es más viejo que el intervalo.
func (a *App) runScheduledBackup() {
	s := a.GetSettings()
	if _, ok := backup.Interval(s.BackupFrequency); !ok {
		return // "off" o inválida
	}
	var last time.Time
	if entries := backup.List(a.backupDir()); len(entries) > 0 {
		if parsed, err := time.Parse(time.RFC3339, entries[0].CreatedAt); err == nil {
			last = parsed
		}
	}
	if !backup.IsDue(s.BackupFrequency, last, time.Now()) {
		return
	}
	if name, err := a.runBackupNow(); err != nil {
		a.emitNotify("Backup", "Automatic backup failed: "+err.Error(), "error")
	} else {
		a.emitNotify("Backup", "Automatic backup created: "+name, "info")
	}
}
