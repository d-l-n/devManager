package main

import (
	"github.com/d-l-n/devmanager/internal/logger"
)

// ---- App Log global (paridad app_logger.py) ----

// GetAppLog devuelve el historial capturado del App Log (ring buffer).
func (a *App) GetAppLog() []logger.Entry {
	if a.appLog == nil {
		return []logger.Entry{}
	}
	return a.appLog.History()
}

// ClearAppLog vacía el App Log global.
func (a *App) ClearAppLog() {
	if a.appLog != nil {
		a.appLog.Clear()
		a.emitNotify("Application Log", "App log cleared", "info")
	}
}
