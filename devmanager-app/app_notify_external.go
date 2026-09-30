package main

import (
	"path/filepath"
	"strings"

	wails "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/d-l-n/devmanager/internal/models"
	"github.com/d-l-n/devmanager/internal/notify"
)

// ---- Notificaciones avanzadas (Issue #66) ----
//
// Dispatcher + store se crean en startup junto al resto de schedulers. El
// dispatch de eventos del ciclo de vida se engancha en createServerManager
// (fan-out como workflows) y en emitNotify (evento genérico "app").

// notifyDir es %APPDATA%/devManager (junto a settings.json): notify.json +
// notify-history.jsonl viven ahí.
func (a *App) notifyDir() string {
	return filepath.Dir(a.settingsPath)
}

// initNotify crea store + dispatcher y carga la config persistida.
func (a *App) initNotify() {
	a.notifyStore = notify.NewStore(a.notifyDir())
	a.notifyStore.Load()
	a.notifyDispatcher = notify.NewDispatcher(a.notifyStore)
}

// dispatchNotifyGated solo envía si el gate global está activo (settings).
// Corre en su propia goroutine: nunca bloquea al emisor del evento.
func (a *App) dispatchNotifyGated(ev notify.Event) {
	a.mu.Lock()
	gate := a.settings.ExternalNotifications
	a.mu.Unlock()
	if !gate {
		return
	}
	if a.notifyDispatcher == nil {
		return
	}
	a.notifyDispatcher.Dispatch(ev)
}

// NotifyEventFire es la entrada de prueba desde la UI (dispara un evento
// sintético para verificar la config sin esperar un evento real).
func (a *App) NotifyEventFire(index int, eventType string) []string {
	if !notify.ValidEvent(eventType) {
		return []string{"invalid event type " + eventType}
	}
	p := a.currentProject(index)
	ev := notify.Event{
		Type:     eventType,
		Project:  p.Name,
		Title:    "Test notification",
		Message:  "Test event fired from devManager",
		Priority: notify.PriorityInfo,
	}
	a.dispatchNotifyGated(ev)
	return nil
}

// NotifyGetConfig devuelve plataformas y reglas (payload único del panel).
func (a *App) NotifyGetConfig() NotifyConfig {
	if a.notifyStore == nil {
		a.initNotify()
	}
	ps, rs := a.notifyStore.Config()
	return NotifyConfig{Platforms: ps, Rules: rs,
		Events: notify.KnownEvents(), Priorities: notify.KnownPriorities()}
}

// NotifyConfig es el payload de NotifyGetConfig.
type NotifyConfig struct {
	Platforms  []notify.PlatformConfig `json:"platforms"`
	Rules      []notify.Rule           `json:"rules"`
	Events     []string                `json:"events"`
	Priorities []string                `json:"priorities"`
}

// NotifySavePlatform inserta/actualiza un destino externo.
func (a *App) NotifySavePlatform(c notify.PlatformConfig) []string {
	if a.notifyStore == nil {
		a.initNotify()
	}
	if errs := a.notifyStore.SavePlatform(c); len(errs) > 0 {
		return errs
	}
	wails.EventsEmit(a.ctx, "notify:config_changed")
	return nil
}

// NotifyDeletePlatform elimina un destino por ID.
func (a *App) NotifyDeletePlatform(id string) []string {
	if a.notifyStore == nil {
		a.initNotify()
	}
	if errs := a.notifyStore.DeletePlatform(id); len(errs) > 0 {
		return errs
	}
	wails.EventsEmit(a.ctx, "notify:config_changed")
	return nil
}

// NotifySaveRule inserta/actualiza una regla de enrutado.
func (a *App) NotifySaveRule(r notify.Rule) []string {
	if a.notifyStore == nil {
		a.initNotify()
	}
	if errs := a.notifyStore.SaveRule(r); len(errs) > 0 {
		return errs
	}
	wails.EventsEmit(a.ctx, "notify:config_changed")
	return nil
}

// NotifyDeleteRule elimina una regla por ID.
func (a *App) NotifyDeleteRule(id string) []string {
	if a.notifyStore == nil {
		a.initNotify()
	}
	if errs := a.notifyStore.DeleteRule(id); len(errs) > 0 {
		return errs
	}
	wails.EventsEmit(a.ctx, "notify:config_changed")
	return nil
}

// NotifyHistory devuelve las últimas `limit` entregas (0 → default 100).
func (a *App) NotifyHistory(limit int) []notify.Delivery {
	if a.notifyStore == nil {
		a.initNotify()
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	return a.notifyStore.History(limit)
}

// hookNotifyServerState nota: este hook se invoca desde OnStateChange en
// app.go createServerManager (junto al fan-out de workflows #65).

// fireNotifyEvent es el helper de los hooks del ciclo de vida: compone el
// evento con nombre de proyecto y prioridad correcta, y dispatcha en background.
func (a *App) fireNotifyEvent(index int, eventType, priority, title, message string) {
	p := a.currentProject(index)
	ev := notify.Event{
		Type:     eventType,
		Project:  p.Name,
		Title:    title,
		Message:  message,
		Priority: priority,
	}
	go a.dispatchNotifyGated(ev)
}

// hookNotifyServerState mapea transiciones de estado del servidor a eventos
// de notificación. Se llama desde OnStateChange (paridad del fan-out de #65).
func (a *App) hookNotifyServerState(index int, state models.ServerState) {
	switch state {
	case models.StateRunning:
		a.fireNotifyEvent(index, notify.EventServerStarted, notify.PriorityInfo,
			"Server started", "Dev server is up and running")
	case models.StateStopped:
		a.fireNotifyEvent(index, notify.EventServerStopped, notify.PriorityInfo,
			"Server stopped", "Dev server stopped")
	case models.StateError:
		reason := ""
		if sm := a.servers[index]; sm != nil {
			reason = sm.FailureReason()
		}
		if strings.TrimSpace(reason) == "" {
			reason = "dev server entered error state"
		}
		a.fireNotifyEvent(index, notify.EventServerCrashed, notify.PriorityCritical,
			"Server error", reason)
	}
}
