package help

import (
	"sort"
	"strings"
)

// ContextHelp es la ayuda contextual de un elemento de la UI.
type ContextHelp struct {
	Target  string `json:"target"`
	TopicID string `json:"topicId"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Keys    string `json:"keys,omitempty"`
}

// ContextTargets lista los elementos con ayuda contextual (orden estable).
func ContextTargets() []string {
	out := make([]string, 0, len(contextEntries))
	for target := range contextEntries {
		out = append(out, target)
	}
	sort.Strings(out)
	return out
}

// ContextHelpFor devuelve la ayuda de un elemento. Acepta el id o un selector
// CSS simple (#id, .clase, data-help="x").
func ContextHelpFor(target string) (ContextHelp, bool) {
	key := normalizeTarget(target)
	entry, ok := contextEntries[key]
	if !ok {
		return ContextHelp{}, false
	}
	title := entry.topicID
	if t, found := FindTopic(entry.topicID); found {
		title = t.Title
	}
	return ContextHelp{
		Target:  key,
		TopicID: entry.topicID,
		Title:   title,
		Text:    entry.text,
		Keys:    entry.keys,
	}, true
}

// ContextEntries devuelve todas las entradas (para el visor de ayuda).
func ContextEntries() []ContextHelp {
	out := make([]ContextHelp, 0, len(contextEntries))
	for _, target := range ContextTargets() {
		entry, _ := ContextHelpFor(target)
		out = append(out, entry)
	}
	return out
}

// TopicsForTargets agrupa los targets por tema (útil para enlazar al revés).
func TopicsForTargets() map[string][]string {
	out := map[string][]string{}
	for _, target := range ContextTargets() {
		entry := contextEntries[target]
		out[entry.topicID] = append(out[entry.topicID], target)
	}
	return out
}

func normalizeTarget(target string) string {
	clean := strings.TrimSpace(target)
	clean = strings.TrimPrefix(clean, "#")
	if idx := strings.IndexAny(clean, "[ \t"); idx >= 0 {
		clean = clean[:idx]
	}
	return clean
}

type contextEntry struct {
	topicID string
	text    string
	keys    string
}

var contextEntries = map[string]contextEntry{
	"btn-add":          {"projects", "Add a project: folder, server command and Playwright commands (auto-detected).", "Ctrl+N"},
	"btn-edit":         {"project-dialog", "Edit the selected project: server, Playwright, user command and environments.", "Ctrl+E"},
	"btn-reload":       {"projects", "Re-read projects.json from disk without restarting the app.", "Ctrl+R"},
	"search":           {"projects", "Filter the project list by name or path.", "Ctrl+F"},
	"btn-start":        {"servers", "Start the selected server; the app waits for the port to answer.", "F5"},
	"btn-stop":         {"servers", "Stop the server and its whole process tree (safe, never touches other processes).", "Shift+F5"},
	"btn-restart":      {"servers", "Restart the server, picking up config changes.", ""},
	"btn-open-url":     {"servers", "Open the project URL in the browser once the server is running.", ""},
	"badge-server":     {"uptime", "Effective port and URL; warns when the detected port differs from the configured one.", ""},
	"btn-env-vars":     {"environments", "Environment variables editor: load/save dotenv files, mark secrets and compare environments.", "Ctrl+Shift+E"},
	"env-switcher":     {"environments", "Switch the active environment (blocked while the server runs).", ""},
	"env-badge":        {"environments", "Environment currently active for this project.", ""},
	"btn-tab-settings": {"tabs", "Hide and reorder tabs for this project; Logs can never be hidden.", ""},
	"btn-settings":     {"settings", "Preferences: polling, toasts, accents, backups and updates.", "Ctrl+,"},
	"btn-theme":        {"themes", "Cycle light / dark / OLED mode; styles are chosen in Settings.", ""},
	"errors-only":      {"logs", "Show only error lines from the running processes.", ""},
	"btn-clear-log":    {"logs", "Clear the log buffer of the selected project.", "Ctrl+L"},
	"pw-run":           {"playwright", "Run the Playwright suite headless; starts the server first if needed.", "Ctrl+T"},
	"pw-report":        {"playwright", "Open the HTML report of the last run.", ""},
	"ev-refresh":       {"evidence", "Rescan test-results/ for screenshots, videos and traces.", ""},
	"deps-run-audit":   {"deps", "Run a security audit with the project's package manager.", ""},
	"dash-start-all":   {"dashboard", "Start every enabled project in one batch.", ""},
	"dash-stop-all":    {"dashboard", "Stop every enabled project in one batch.", ""},
	"dash-tests-all":   {"dashboard", "Queue the test suite of every project with Playwright enabled.", ""},
	"dash-layout":      {"dashboard", "Choose which dashboard sections are visible.", ""},
	"custom-command":   {"scripts", "Run any command in the project folder; output goes to Logs.", ""},
	"backlog-add":      {"backlog", "Add an item to the project backlog with status and priority.", ""},
	"btn-check-update": {"updater", "Check GitHub Releases for a newer version.", ""},
	".tab":             {"tabs", "Switch panels: Logs, Scripts, Git, Deps, Playwright, Evidence, Obscura, Backlog.", ""},
	"help-search":      {"shortcuts", "Search the documentation, FAQ and shortcuts.", ""},
	"help-tutorial":    {"shortcuts", "Step-by-step guides that track your progress.", ""},
	"monitor-kill":     {"monitor", "Kill a foreign process occupying a configured port.", ""},
	"git-pull":         {"git", "Pull the current branch with live output in the Logs tab.", ""},
	"backup-create":    {"backups", "Create an immediate backup of projects.json and settings.json.", ""},
}
