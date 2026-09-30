// Package help implementa el sistema de documentación y ayuda del issue #72:
// catálogo de temas, búsqueda full-text, FAQ, tutoriales con progreso,
// changelog embebido, atajos de teclado y ayuda contextual por elemento.
//
// Todo es contenido propio + stdlib: no abre red ni lee el proyecto del
// usuario. El changelog sí se lee de disco (CHANGELOG.md, best-effort).
package help

import (
	"sort"
	"strings"
)

// Topic es una entrada de documentación. Body usa un formato mínimo:
// líneas "## Titulo" son subtítulos, "- item" viñetas y el resto párrafos.
type Topic struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Section  string   `json:"section"`
	Summary  string   `json:"summary"`
	Body     string   `json:"body"`
	Keywords []string `json:"keywords,omitempty"`
	Related  []string `json:"related,omitempty"`
}

// SectionOrder fija el orden de las secciones en la UI (las desconocidas van
// al final, alfabéticas).
var SectionOrder = []string{
	"Getting started",
	"Projects & servers",
	"Testing",
	"Environments",
	"Tools",
	"Appearance",
	"Data & safety",
	"Troubleshooting",
}

// Catalog devuelve el catálogo completo (ordenado por sección y título).
func Catalog() []Topic {
	out := make([]Topic, len(topics))
	copy(out, topics)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := sectionRank(out[i].Section), sectionRank(out[j].Section)
		if si != sj {
			return si < sj
		}
		return out[i].Title < out[j].Title
	})
	return out
}

// FindTopic busca por ID (case-insensitive).
func FindTopic(id string) (Topic, bool) {
	needle := strings.ToLower(strings.TrimSpace(id))
	for _, t := range topics {
		if strings.ToLower(t.ID) == needle {
			return t, true
		}
	}
	return Topic{}, false
}

// Sections agrupa el catálogo por sección respetando SectionOrder.
func Sections() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range Catalog() {
		if !seen[t.Section] {
			seen[t.Section] = true
			out = append(out, t.Section)
		}
	}
	return out
}

func sectionRank(section string) int {
	for i, s := range SectionOrder {
		if s == section {
			return i
		}
	}
	return len(SectionOrder)
}

// topics es el contenido curado. Cada tema debe existir en el código real:
// si un feature se renombra, el tema se actualiza (hay test de integridad).
var topics = []Topic{
	{
		ID: "getting-started", Title: "Getting started", Section: "Getting started",
		Summary: "Add your first project and start its dev server.",
		Body: "devManager manages local dev projects: servers, Playwright tests, " +
			"dependencies, evidence, git and backlog from one window.\n\n" +
			"## First steps\n" +
			"- Press Ctrl+N (or + Add in the sidebar) to add a project.\n" +
			"- Pick the project folder: the app auto-detects the server command, the package manager and Playwright.\n" +
			"- Select the project and press Start (or F5) to run its dev server.\n" +
			"- Open the URL with the Open URL button when the server is running.",
		Keywords: []string{"start", "first", "onboarding", "add project", "setup"},
		Related:  []string{"projects", "servers", "shortcuts"},
	},
	{
		ID: "projects", Title: "Managing projects", Section: "Projects & servers",
		Summary: "Add, edit, pin, filter and remove projects.",
		Body: "Projects live in projects.json next to the executable (a backup is " +
			"created automatically if the file is invalid).\n\n" +
			"## Actions\n" +
			"- Add: Ctrl+N. Edit: Ctrl+E or double-click. Remove: Delete.\n" +
			"- Pin a project to keep it at the top of the sidebar.\n" +
			"- Filter with the All / Running / Stopped chips plus the search box (Ctrl+F).\n" +
			"- Ctrl+R reloads projects.json from disk without restarting.",
		Keywords: []string{"project", "pin", "filter", "search", "remove", "edit", "projects.json"},
		Related:  []string{"getting-started", "project-dialog", "troubleshooting"},
	},
	{
		ID: "project-dialog", Title: "Project settings dialog", Section: "Projects & servers",
		Summary: "Server command, port, URL, Playwright commands and user creation.",
		Body: "The project dialog groups everything devManager needs per project.\n\n" +
			"## Fields\n" +
			"- Server: enabled, command, port, URL and startup timeout.\n" +
			"- Playwright: test, UI mode, debug and report commands.\n" +
			"- User: command used by Create User (env vars DM_USER_*) and its auto-detection.\n" +
			"- Environments: per-environment overrides of the three blocks above (see Environments).\n\n" +
			"Auto-detect fills the commands from package.json, go.mod and common layouts; " +
			"you can always override them by hand.",
		Keywords: []string{"dialog", "command", "port", "url", "timeout", "autodetect", "detection"},
		Related:  []string{"projects", "servers", "playwright", "environments"},
	},
	{
		ID: "servers", Title: "Server lifecycle", Section: "Projects & servers",
		Summary: "Start, stop, restart and monitor dev servers.",
		Body: "devManager runs each project's dev command as a hidden-console child process.\n\n" +
			"## Controls\n" +
			"- Start / Stop / Restart with live state badges (stopped, starting, running, error).\n" +
			"- F5 starts or restarts the selected project; Shift+F5 stops it.\n" +
			"- Uptime is shown per server in the Server panel.\n" +
			"- Start all / Stop all act on every enabled project.\n\n" +
			"Stopping uses a safe process-tree kill in Windows, so child processes die with the server " +
			"and unrelated processes are never touched.",
		Keywords: []string{"start", "stop", "restart", "uptime", "process", "kill", "port"},
		Related:  []string{"uptime", "monitor", "troubleshooting"},
	},
	{
		ID: "uptime", Title: "Uptime and status", Section: "Projects & servers",
		Summary: "State badges, startup timeout and port availability.",
		Body: "After a start, devManager waits for the configured port to answer before " +
			"declaring the server ready.\n\n" +
			"## If a server never becomes ready\n" +
			"- Increase Startup Timeout in the project dialog.\n" +
			"- Check the detected port in the Server badge or the Monitor tab.\n" +
			"- Confirm the URL matches the port the server actually listens on.",
		Keywords: []string{"timeout", "ready", "badge", "port", "waiting"},
		Related:  []string{"servers", "monitor", "troubleshooting"},
	},
	{
		ID: "playwright", Title: "Running Playwright tests", Section: "Testing",
		Summary: "Headless runs, UI mode, debug mode and HTML reports.",
		Body: "The Playwright tab drives the test suite of the selected project.\n\n" +
			"## Modes\n" +
			"- Run Tests (Ctrl+T): headless run in the background, output goes to Logs.\n" +
			"- UI Mode: interactive Playwright UI as a persistent process.\n" +
			"- Debug Mode: Playwright Inspector.\n" +
			"- View Test Report: opens the HTML report of the last run.\n\n" +
			"If no server is running, the app starts it and waits for port availability first.",
		Keywords: []string{"playwright", "test", "ui mode", "debug", "report", "e2e"},
		Related:  []string{"evidence", "advanced-testing", "logs"},
	},
	{
		ID: "evidence", Title: "Evidence gallery", Section: "Testing",
		Summary: "Screenshots, videos and traces from test-results/.",
		Body: "The Evidence tab scans test-results/ of the project.\n\n" +
			"## Contents\n" +
			"- Thumbnails with integrated preview for screenshots and videos.\n" +
			"- Trace viewer for Playwright traces.\n" +
			"- Open externally or reveal the containing folder.\n\n" +
			"Visual diffs generated by the Testing tab (test-results/visual/diff) also show up here.",
		Keywords: []string{"evidence", "screenshot", "video", "trace", "test-results"},
		Related:  []string{"playwright", "advanced-testing"},
	},
	{
		ID: "advanced-testing", Title: "Coverage, performance and visual regression", Section: "Testing",
		Summary: "Coverage summaries, k6/artillery metrics, PNG diffs, run analytics and the browser matrix.",
		Body: "The Testing panel aggregates test artefacts already produced by your " +
			"project and never runs tools on its own.\n\n" +
			"## What it reads\n" +
			"- Coverage: coverage/coverage-summary.json, coverage-final.json, cobertura XML, go coverprofile, coverage.out.\n" +
			"- Performance: test-results/k6-summary.json or artillery-report.json (with threshold checks).\n" +
			"- Visual regression: PNGs compared baseline vs current in test-results/visual/ (tolerance + diff images).\n" +
			"- Analytics: pass rate, trend and flaky tests from the recorded run history.\n" +
			"- Matrix: browser x environment combinations resolved from the recorded runs.\n" +
			"- Test data: test-data/, fixtures/, tests/fixtures/, tests/data/.\n\n" +
			"## Tips\n" +
			"- Promote baseline accepts the current screenshots as the new reference.\n" +
			"- Recording runs gives the matrix and analytics their data.",
		Keywords: []string{"coverage", "k6", "artillery", "visual regression", "flaky", "matrix", "performance"},
		Related:  []string{"playwright", "evidence", "environments"},
	},
	{
		ID: "environments", Title: "Environments (dev, staging, prod)", Section: "Environments",
		Summary: "Per-environment server/test config, .env variables, secrets and diffs.",
		Body: "Every project can hold several named environments and switch the active one " +
			"with one click from the header selector.\n\n" +
			"## What changes per environment\n" +
			"- Server block (command, port, URL) and Playwright/User commands.\n" +
			"- Environment variables, loaded from a dotenv file (.env, .env.staging, .env.prod).\n" +
			"- Secrets: keys marked as secret are masked when read unless you reveal them.\n\n" +
			"## Rules\n" +
			"- Switching is blocked while the server is running (stop it first).\n" +
			"- Legacy projects without envs behave as a single dev environment.\n" +
			"- Ctrl+Shift+E opens the environment variables editor.",
		Keywords: []string{"environment", "env", "dotenv", "staging", "prod", "secrets", "variables"},
		Related:  []string{"project-dialog", "servers"},
	},
	{
		ID: "deps", Title: "Dependencies dashboard", Section: "Tools",
		Summary: "Installed dependencies, outdated packages and security audit.",
		Body: "The Deps tab inspects the project's package manager (npm, pnpm, yarn, bun) or Go module.\n\n" +
			"## Views\n" +
			"- Installed dependencies with versions.\n" +
			"- Outdated packages.\n" +
			"- Security audit with severity levels.\n\n" +
			"The tab only refreshes while it is active: no package commands run in the background.",
		Keywords: []string{"deps", "npm", "audit", "outdated", "pnpm", "yarn", "bun", "go mod"},
		Related:  []string{"scripts", "project-dialog"},
	},
	{
		ID: "scripts", Title: "Project scripts and custom commands", Section: "Tools",
		Summary: "Run package.json scripts or an arbitrary command.",
		Body: "The Scripts tab lists the scripts detected in the project and lets you run " +
			"a one-off command.\n\n" +
			"## Notes\n" +
			"- Filter the list with the script filter box.\n" +
			"- One script runs at a time per project; stop it before starting another.\n" +
			"- Output streams to the Logs tab.",
		Keywords: []string{"scripts", "npm run", "custom command", "run"},
		Related:  []string{"logs", "deps"},
	},
	{
		ID: "logs", Title: "Logs and the app log", Section: "Tools",
		Summary: "Live server/test output plus a global application log window.",
		Body: "Two different logs exist and it is worth keeping them apart.\n\n" +
			"## Project logs\n" +
			"- The Logs tab streams server, Playwright and script output with timestamps.\n" +
			"- Errors-only mode filters the noise. Ctrl+L clears the buffer.\n\n" +
			"## App log\n" +
			"- Ctrl+Alt+L (or Ctrl+K) opens the global application log in its own window.\n" +
			"- Useful to diagnose devManager itself, independent of any project.",
		Keywords: []string{"logs", "output", "errors", "app log", "console"},
		Related:  []string{"servers", "playwright", "troubleshooting"},
	},
	{
		ID: "git", Title: "Git tools", Section: "Tools",
		Summary: "Branch, status, pull/fetch/stash, diffs, branches and tags.",
		Body: "The Git tab is read-mostly: it shows the repository state and offers the " +
			"operations you use between commits.\n\n" +
			"## Capabilities\n" +
			"- Branch, dirty state, ahead/behind and last commit.\n" +
			"- Pull, Fetch and Stash with live output in Logs.\n" +
			"- Diff viewer, branch create/rename/checkout/delete, tag create/delete/push.",
		Keywords: []string{"git", "branch", "commit", "diff", "tag", "stash", "pull"},
		Related:  []string{"logs", "projects"},
	},
	{
		ID: "backlog", Title: "Backlog", Section: "Tools",
		Summary: "Per-project items with status and priority.",
		Body: "The Backlog tab stores items per project inside projects.json.\n\n" +
			"## Usage\n" +
			"- Add items with a title, description, status (todo, in-progress, done) and priority.\n" +
			"- Reorder items with drag and drop; edit or delete from the row actions.",
		Keywords: []string{"backlog", "todo", "task", "priority"},
		Related:  []string{"projects"},
	},
	{
		ID: "monitor", Title: "Monitor: ports, CPU and RAM", Section: "Tools",
		Summary: "Port occupancy, process tree resource usage and foreign-process kill.",
		Body: "Ctrl+Alt+M opens the Monitor window, independent of any project.\n\n" +
			"## What it shows\n" +
			"- Configured ports: free, occupied by this app or by a foreign process.\n" +
			"- CPU and RAM per server process tree, refreshed every 3 seconds.\n" +
			"- Kill for foreign processes occupying a configured port; polling is configurable in Settings.",
		Keywords: []string{"monitor", "ports", "cpu", "ram", "kill", "occupancy"},
		Related:  []string{"servers", "uptime", "settings"},
	},
	{
		ID: "dashboard", Title: "Global dashboard", Section: "Tools",
		Summary: "Every project at a glance with live metrics and history.",
		Body: "The Dashboard view summarizes all projects.\n\n" +
			"## Sections\n" +
			"- Project cards with server state and quick actions.\n" +
			"- Alerts for failing projects.\n" +
			"- Uptime history (24h) and performance history from the resource sampler.\n\n" +
			"Filters (all, running, stopped, failed tests) plus the section toggles in the " +
			"dashboard toolbar control what you see. Start all / Stop all / Run all tests act in batch.",
		Keywords: []string{"dashboard", "overview", "metrics", "history", "alerts"},
		Related:  []string{"servers", "uptime", "monitor"},
	},
	{
		ID: "user-creation", Title: "Creating users", Section: "Tools",
		Summary: "Run a per-project user creation command with DM_USER_* variables.",
		Body: "If a project has a create-user flow (Firebase, REST, DB seed), the Create User " +
			"dialog runs it for you.\n\n" +
			"## Details\n" +
			"- The command is auto-detected from package.json scripts and can be edited per project.\n" +
			"- Data travels as environment variables: DM_USER_EMAIL, DM_USER_NAME, DM_USER_PASSWORD, DM_USER_ROLE.\n" +
			"- There is no shell interpolation, so passwords with special characters are safe and never logged.",
		Keywords: []string{"user", "create user", "seed", "firebase", "credentials"},
		Related:  []string{"scripts", "environments"},
	},
	{
		ID: "tabs", Title: "Customizing per-project tabs", Section: "Appearance",
		Summary: "Hide and reorder the tabs of each project.",
		Body: "Each project can show a different set of tabs in a different order.\n\n" +
			"## How\n" +
			"- Open the tab settings icon at the end of the tab bar.\n" +
			"- Toggle visibility and drag to reorder; changes are saved per project.\n" +
			"- The Logs tab can never be hidden: it is the fallback when the active tab disappears.\n" +
			"- Some tabs hide themselves automatically when they do not apply (Playwright disabled, no evidence, not a git repo).",
		Keywords: []string{"tabs", "order", "hide", "customize", "tab bar"},
		Related:  []string{"projects", "settings"},
	},
	{
		ID: "settings", Title: "Settings", Section: "Appearance",
		Summary: "Polling intervals, toasts, accents and persisted preferences.",
		Body: "Ctrl+, opens the full-screen Settings view.\n\n" +
			"## Options\n" +
			"- Resource polling interval for the Monitor.\n" +
			"- Toast notifications on or off.\n" +
			"- Accent colors: per style and global override.\n" +
			"- Backup frequency and retention.\n\n" +
			"Settings are stored in %APPDATA%/devManager/settings.json, next to your backups.",
		Keywords: []string{"settings", "preferences", "polling", "toasts", "accent"},
		Related:  []string{"themes", "backups"},
	},
	{
		ID: "themes", Title: "Themes and styles", Section: "Appearance",
		Summary: "Light, dark and OLED modes with Brutalist, Glassmorphism, Retro and Dracula styles.",
		Body: "Appearance is split in two independent choices.\n\n" +
			"## Options\n" +
			"- Mode: light, dark or OLED (button in the header, Ctrl+Shift+T toggles from the app).\n" +
			"- Style: standard, Brutalist, Glassmorphism, Retro, Dracula.\n" +
			"- Accent color per style plus a global override, calibrated for WCAG AA contrast (>= 4.5:1).",
		Keywords: []string{"theme", "dark", "light", "oled", "glassmorphism", "retro", "dracula", "accent"},
		Related:  []string{"settings"},
	},
	{
		ID: "backups", Title: "Backups and restore", Section: "Data & safety",
		Summary: "Automatic and manual backups of projects.json and settings.json.",
		Body: "Backups are written to %APPDATA%/devManager/backups as .dmbak archives.\n\n" +
			"## Behaviour\n" +
			"- A check runs on startup plus a 15-minute ticker while the app is open.\n" +
			"- Frequency and retention are configured in Settings.\n" +
			"- The catalog lists each backup with validity status; restore validates before writing.\n\n" +
			"Restoring replaces projects.json and settings.json, so the app reloads afterwards.",
		Keywords: []string{"backup", "restore", "retention", "dmbak", "safety"},
		Related:  []string{"settings", "projects", "troubleshooting"},
	},
	{
		ID: "updater", Title: "Built-in updater", Section: "Data & safety",
		Summary: "Checks GitHub Releases and shows the available version.",
		Body: "devManager checks for updates on boot and from Settings.\n\n" +
			"## Details\n" +
			"- Current and latest versions are compared from GitHub Releases.\n" +
			"- The check is a background request; failures never block the UI.\n" +
			"- Downloads happen from the release page, not silently.",
		Keywords: []string{"update", "updater", "release", "version", "github"},
		Related:  []string{"settings"},
	},
	{
		ID: "shortcuts", Title: "Keyboard shortcuts", Section: "Appearance",
		Summary: "Every global shortcut, with its context.",
		Body: "Shortcuts are global unless noted; inside text inputs only Ctrl+F and Ctrl+, react.\n\n" +
			"See the Shortcuts section of this help window for the full list, and press F1 to " +
			"open this help at any time.",
		Keywords: []string{"shortcuts", "keyboard", "hotkeys", "keys"},
		Related:  []string{"getting-started", "settings"},
	},
	{
		ID: "troubleshooting", Title: "Troubleshooting", Section: "Troubleshooting",
		Summary: "Common problems and what to check first.",
		Body: "## The server never becomes ready\n" +
			"- Raise Startup Timeout, confirm the port and check the Logs tab for the real error.\n\n" +
			"## npm or npx is not recognized\n" +
			"- Node.js must be on the PATH of the app process; restart devManager after installing it.\n\n" +
			"## projects.json looks corrupted\n" +
			"- A .bak copy is written and a clean configuration is generated; restore from Backups if needed.\n\n" +
			"## A port is occupied by a foreign process\n" +
			"- Open the Monitor and use Kill, or change the project port.\n\n" +
			"## Tests fail only sometimes\n" +
			"- Check the Testing panel: flaky detection highlights tests that pass and fail across runs.",
		Keywords: []string{"troubleshooting", "error", "problem", "fix", "port", "timeout", "corrupt"},
		Related:  []string{"logs", "monitor", "backups", "advanced-testing"},
	},
}
