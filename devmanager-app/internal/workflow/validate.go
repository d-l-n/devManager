// Package workflow implementa el MVP de workflows / webhooks / CI-CD
// (Issue #65): validación de schedules, executor secuencial con retry +
// timeout, historial de runs persistido, triggers CI y webhooks salientes,
// y listener HTTP entrante. Solo stdlib (+ models).
package workflow

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/d-l-n/devmanager/internal/models"
)

// ValidSchedule reporta si s es un schedule aceptado: @every <duración>,
// @hourly, @daily o cron simple de 5 campos (min hora dom mes dow).
func ValidSchedule(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	switch s {
	case "@hourly", "@daily":
		return true
	}
	if strings.HasPrefix(s, "@every ") {
		d, err := time.ParseDuration(strings.TrimSpace(strings.TrimPrefix(s, "@every ")))
		return err == nil && d > 0
	}
	if strings.HasPrefix(s, "@") {
		return false
	}
	return validCron(s)
}

// validCron valida 5 campos con átomos "*", "*/n", "n", "a,b", "a-b".
func validCron(s string) bool {
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return false
	}
	maxs := []int{59, 23, 31, 12, 6}
	for i, f := range fields {
		if !validCronField(f, 0, maxs[i]) {
			return false
		}
	}
	return true
}

func validCronField(f string, min, max int) bool {
	for _, part := range strings.Split(f, ",") {
		if !validCronAtom(strings.TrimSpace(part), min, max) {
			return false
		}
	}
	return true
}

func validCronAtom(a string, min, max int) bool {
	if a == "*" {
		return true
	}
	if strings.HasPrefix(a, "*/") {
		n, err := strconv.Atoi(strings.TrimPrefix(a, "*/"))
		return err == nil && n > 0 && n <= max+1
	}
	if strings.Contains(a, "-") {
		se := strings.SplitN(a, "-", 2)
		lo, err1 := strconv.Atoi(se[0])
		hi, err2 := strconv.Atoi(se[1])
		return err1 == nil && err2 == nil && lo >= min && hi <= max && lo <= hi
	}
	n, err := strconv.Atoi(a)
	return err == nil && n >= min && n <= max
}

// ValidateWorkflow valida un workflow suelto (misma base que
// models.Project.Validate más la sintaxis del schedule). Devuelve errores.
func ValidateWorkflow(w models.Workflow) []string {
	p := models.Project{
		Name: "x", Path: "x",
		Workflows: []models.Workflow{w},
	}
	errs := p.Validate()
	// Filtra los errores del proyecto ficticio (nunca hay: Name/Path dados).
	var out []string
	for _, e := range errs {
		out = append(out, e)
	}
	if w.Trigger == models.WorkflowTriggerSchedule && !ValidSchedule(w.Schedule) {
		out = append(out, fmt.Sprintf("workflows: invalid schedule %q (use @every 5m, @hourly, @daily or cron \"M H DOM MON DOW\")", w.Schedule))
	}
	return out
}
