package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) emitConfigError(message string) {
	if a.ctx != nil {
		wails.EventsEmit(a.ctx, "config:error", map[string]string{"message": message})
	} else {
		fmt.Println("config error:", message)
	}
}

// emitNotify replica _notify del Python: ventana visible → toast in-app;
// oculta/minimizada → notificación nativa de bandeja con cooldown de 3s.
func (a *App) emitNotify(title, message, level string) {
	if a.ctx != nil {
		if a.windowVisible() {
			wails.EventsEmit(a.ctx, "notify", map[string]string{
				"title": title, "message": message, "level": level,
			})
			return
		}
	}
	a.nativeNotify(title, message, level == "error" || level == "warning")
}

// windowVisible replica self.isVisible() del Python usando el flag de
// ocultar-a-bandeja (beforeClose). La ventana minimizada cuenta como visible.
func (a *App) windowVisible() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return !a.windowHidden
}

// setWindowShown marca la ventana como visible (desde tray/show/click-to-focus).
func (a *App) setWindowShown() {
	a.mu.Lock()
	a.windowHidden = false
	a.mu.Unlock()
}

// nativeNotify muestra una notificación de bandeja con cooldown de 3s y
// buffer de una pendiente (paridad _notify/_flush_pending del Python).
func (a *App) nativeNotify(title, message string, isError bool) {
	a.notifyMu.Lock()
	now := time.Now()
	if now.Sub(a.lastTrayNotify) < 3*time.Second {
		a.pendingNotify = &pendingTrayNotify{title: title, message: message, isError: isError}
		a.notifyMu.Unlock()
		return
	}
	a.lastTrayNotify = now
	a.notifyMu.Unlock()

	a.showNativeBalloon(title, message, isError)

	// Programar el flush de la pendiente (paridad QTimer.singleShot(3200)).
	go func() {
		time.Sleep(3200 * time.Millisecond)
		a.notifyMu.Lock()
		defer a.notifyMu.Unlock()
		if a.pendingNotify != nil {
			p := a.pendingNotify
			a.pendingNotify = nil
			a.lastTrayNotify = time.Now()
			a.showNativeBalloon(p.title, p.message, p.isError)
		}
	}()
}

// showNativeBalloon emite una burbuja nativa de bandeja vía PowerShell
// (System.Windows.Forms.NotifyIcon), fiel a QSystemTrayIcon.showMessage y
// sin necesidad de registrar AppUserModelID. Fallo → toast de respaldo.
func (a *App) showNativeBalloon(title, message string, isError bool) {
	icon := "Info"
	if isError {
		icon = "Error"
	}
	escQuote := func(s string) string { return strings.ReplaceAll(s, "'", "''") }
	t, m := escQuote(title), escQuote(message)
	ps := `[void][reflection.assembly]::loadwithpartialname('System.Windows.Forms');` +
		`[void][reflection.assembly]::loadwithpartialname('System.Drawing');` +
		`$n=New-Object system.windows.forms.notifyicon;` +
		`$n.icon=[system.drawing.systemicons]::Information;` +
		`$n.visible=$true;` +
		`$n.showballoontip(3500,'` + t + `','` + m + `',[system.windows.forms.tooltipicon]::` + icon + `);`
	cmd := exec.Command("powershell", "-NoProfile", "-WindowStyle", "Hidden", "-Command", ps)
	if err := cmd.Start(); err != nil {
		// Respaldo: toast normal
		if a.ctx != nil {
			wails.EventsEmit(a.ctx, "notify", map[string]string{
				"title": title, "message": message, "level": "warning",
			})
		}
	}
}
