package main

import (
	"github.com/d-l-n/devmanager/internal/utils/deps"
)

// ---- Deps (Issue #61/#62: dashboard + outdated + audit) ----

// GetDeps lista dependencias del proyecto con su estado de actualización.
func (a *App) GetDeps(index int) deps.ManagerResult {
	p := a.projectPath(index)
	if p == "" {
		return deps.ManagerResult{Error: "proyecto inválido"}
	}
	r := deps.ListDeps(p)
	if r.Manager != "" && r.Error == "" {
		r.Deps = deps.CheckOutdated(p, r.Deps)
	}
	return r
}

// GetDepsAudit corre audit de seguridad sobre las dependencias del proyecto.
func (a *App) GetDepsAudit(index int) deps.AuditResult {
	p := a.projectPath(index)
	if p == "" {
		return deps.AuditResult{Error: "proyecto inválido"}
	}
	return deps.RunAudit(p)
}
