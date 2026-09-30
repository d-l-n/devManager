package help

import (
	"strings"
	"time"
)

// Step es un paso de tutorial. Target es un selector CSS del elemento que hay
// que resaltar (la vista los aplica de forma best-effort).
type Step struct {
	Index   int    `json:"index"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	Target  string `json:"target,omitempty"`
	TopicID string `json:"topicId,omitempty"`
}

// Tutorial es un recorrido guiado por una tarea concreta.
type Tutorial struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Steps   []Step `json:"steps"`
}

// TutorialProgress es el avance persistido de un tutorial. Step es el índice
// del último paso completado (-1 = empezado sin completar ninguno).
type TutorialProgress struct {
	Step      int       `json:"step"`
	Total     int       `json:"total"`
	Completed bool      `json:"completed"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}

// Progresses es el mapa tutorialID → progreso.
type Progresses map[string]TutorialProgress

// Tutorials devuelve los tutoriales disponibles.
func Tutorials() []Tutorial {
	out := make([]Tutorial, len(tutorials))
	copy(out, tutorials)
	return out
}

// FindTutorial busca un tutorial por ID.
func FindTutorial(id string) (Tutorial, bool) {
	for _, t := range tutorials {
		if t.ID == id {
			return t, true
		}
	}
	return Tutorial{}, false
}

// NextStep devuelve el paso siguiente (0-based) dado el progreso actual.
// Si el tutorial está completo devuelve len(Steps).
func NextStep(t Tutorial, p TutorialProgress) int {
	next := p.Step + 1
	if next < 0 {
		next = 0
	}
	if next > len(t.Steps) {
		next = len(t.Steps)
	}
	return next
}

// IsComplete indica si el progreso cubre todos los pasos.
func IsComplete(t Tutorial, p TutorialProgress) bool {
	return len(t.Steps) > 0 && p.Step >= len(t.Steps)-1
}

// CompletionPct devuelve el porcentaje completado (0..100).
func CompletionPct(t Tutorial, p TutorialProgress) float64 {
	if len(t.Steps) == 0 {
		return 0
	}
	done := p.Step + 1
	if done <= 0 {
		return 0
	}
	if done > len(t.Steps) {
		done = len(t.Steps)
	}
	return round2pct(float64(done) / float64(len(t.Steps)) * 100)
}

func round2pct(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// NormalizeStep acota un índice de paso al rango válido del tutorial.
func NormalizeStep(t Tutorial, step int) int {
	if step < -1 {
		return -1
	}
	if step > len(t.Steps)-1 {
		return len(t.Steps) - 1
	}
	return step
}

// CompletedTutorials lista los IDs completados según el progreso guardado.
func CompletedTutorials(p Progresses) []string {
	out := []string{}
	for _, t := range tutorials {
		if prog, ok := p[t.ID]; ok && (prog.Completed || IsComplete(t, prog)) {
			out = append(out, t.ID)
		}
	}
	return out
}

// TutorialProgressSummary resume el avance global (para la portada de ayuda).
func TutorialProgressSummary(p Progresses) (completed, total int, pct float64) {
	total = len(tutorials)
	completed = len(CompletedTutorials(p))
	if total == 0 {
		return 0, 0, 0
	}
	return completed, total, round2pct(float64(completed) / float64(total) * 100)
}

// StepsOfTopic devuelve los IDs de tutorial que enlazan a un tema.
func StepsOfTopic(topicID string) []string {
	out := []string{}
	for _, t := range tutorials {
		for _, s := range t.Steps {
			if strings.EqualFold(s.TopicID, topicID) {
				out = append(out, t.ID)
				break
			}
		}
	}
	return out
}

var tutorials = []Tutorial{
	{
		ID: "first-project", Title: "Start your first project",
		Summary: "Add a project, start its server and open it in the browser.",
		Steps: []Step{
			{Index: 0, Title: "Add a project", Target: "#btn-add", TopicID: "getting-started",
				Body: "Click + Add (or press Ctrl+N) and pick your project folder. " +
					"devManager auto-detects the server command, package manager and Playwright."},
			{Index: 1, Title: "Review the detected config", Target: "#btn-edit", TopicID: "project-dialog",
				Body: "Open the project dialog (Ctrl+E) and confirm command, port and URL. " +
					"Raise Startup Timeout if the first cold build is slow."},
			{Index: 2, Title: "Start the server", Target: "#btn-start", TopicID: "servers",
				Body: "Select the project and press Start (or F5). The state badge moves to " +
					"starting and then running once the port answers."},
			{Index: 3, Title: "Open the app", Target: "#btn-open-url", TopicID: "servers",
				Body: "With the server running, press Open URL in Browser to check the site. " +
					"Uptime starts counting in the Server panel."},
		},
	},
	{
		ID: "run-tests", Title: "Run tests and review evidence",
		Summary: "Execute Playwright, watch logs and inspect screenshots and traces.",
		Steps: []Step{
			{Index: 0, Title: "Open the Playwright tab", Target: ".tab[data-tab=\"playwright\"]", TopicID: "playwright",
				Body: "Select a project and switch to the Playwright tab. If no server is running, " +
					"it is started automatically before the run."},
			{Index: 1, Title: "Run the suite", Target: "#pw-run", TopicID: "playwright",
				Body: "Press Run Tests (Ctrl+T). Output streams to the Logs tab and the status " +
					"badge shows passed or failed."},
			{Index: 2, Title: "Check the report", Target: "#pw-report", TopicID: "playwright",
				Body: "View Test Report opens the HTML report of the last run."},
			{Index: 3, Title: "Review evidence", Target: ".tab[data-tab=\"evidence\"]", TopicID: "evidence",
				Body: "The Evidence tab collects screenshots, videos and traces from test-results/, " +
					"with preview and trace viewer."},
		},
	},
	{
		ID: "switch-environment", Title: "Work with environments",
		Summary: "Switch between dev, staging and prod without leaving the app.",
		Steps: []Step{
			{Index: 0, Title: "Inspect the active environment", Target: "#env-badge", TopicID: "environments",
				Body: "The header badge shows which environment is active for the selected project."},
			{Index: 1, Title: "Stop the server", Target: "#btn-stop", TopicID: "servers",
				Body: "Switching is blocked while a server runs: stop it first (or let the switcher " +
					"offer to stop it for you)."},
			{Index: 2, Title: "Switch", Target: "#env-switcher", TopicID: "environments",
				Body: "Pick the target environment in the selector. Port, command and variables " +
					"come from that environment's config."},
			{Index: 3, Title: "Edit variables and secrets", Target: "#btn-env-vars", TopicID: "environments",
				Body: "Ctrl+Shift+E opens the variables editor: load/save dotenv files, mark keys as " +
					"secret and compare two environments side by side."},
		},
	},
	{
		ID: "safe-backups", Title: "Keep your configuration safe",
		Summary: "Configure backups, create one manually and restore it.",
		Steps: []Step{
			{Index: 0, Title: "Open Settings", Target: "#btn-settings", TopicID: "settings",
				Body: "Ctrl+, opens Settings, where backup frequency and retention live."},
			{Index: 1, Title: "Create a backup", TopicID: "backups",
				Body: "Create a manual backup before risky changes: projects.json and settings.json " +
					"are archived together."},
			{Index: 2, Title: "Review the catalog", TopicID: "backups",
				Body: "The catalog marks each archive as valid or broken, so a corrupted backup can " +
					"never silently replace your config."},
			{Index: 3, Title: "Restore when needed", TopicID: "backups",
				Body: "Restoring validates the archive first and reloads the app afterwards."},
		},
	},
}
