package help

import "sort"

// FAQ es una pregunta frecuente con su respuesta y categoría.
type FAQ struct {
	ID       string `json:"id"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Category string `json:"category"`
	TopicID  string `json:"topicId,omitempty"`
}

// FAQList devuelve las preguntas frecuentes ordenadas por categoría.
func FAQList() []FAQ {
	out := make([]FAQ, len(faqs))
	copy(out, faqs)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		return out[i].Question < out[j].Question
	})
	return out
}

// FAQCategories devuelve las categorías presentes, en orden de aparición.
func FAQCategories() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, f := range faqs {
		if !seen[f.Category] {
			seen[f.Category] = true
			out = append(out, f.Category)
		}
	}
	return out
}

// FindFAQ busca por ID.
func FindFAQ(id string) (FAQ, bool) {
	for _, f := range faqs {
		if f.ID == id {
			return f, true
		}
	}
	return FAQ{}, false
}

var faqs = []FAQ{
	{
		ID: "where-is-config", Category: "Configuration", TopicID: "projects",
		Question: "Where does devManager store my projects?",
		Answer: "In projects.json, next to the executable. Preferences live in " +
			"%APPDATA%/devManager/settings.json, and backups in %APPDATA%/devManager/backups. " +
			"Nothing is stored inside your project folders except test artefacts (test-results/).",
	},
	{
		ID: "corrupt-config", Category: "Configuration", TopicID: "troubleshooting",
		Question: "What happens if projects.json is corrupted?",
		Answer: "The app writes projects.json.bak with the previous contents and starts with a " +
			"clean, empty configuration instead of failing to open. Use Backups to restore an " +
			"older snapshot if the .bak is not enough.",
	},
	{
		ID: "server-wont-start", Category: "Servers", TopicID: "uptime",
		Question: "Why does my server stay in starting state?",
		Answer: "devManager waits for the configured port to answer. Check the Logs tab for the " +
			"real error, confirm the port in the Monitor, and raise Startup Timeout for slow " +
			"bundlers. If the port is occupied by a foreign process, kill it from the Monitor.",
	},
	{
		ID: "port-in-use", Category: "Servers", TopicID: "monitor",
		Question: "A configured port is already in use, what now?",
		Answer: "Open the Monitor (Ctrl+Alt+M): it shows whether the port is free, used by one of " +
			"your servers, or by a foreign process. Kill the foreign process or change the port in " +
			"the project dialog.",
	},
	{
		ID: "switch-env-blocked", Category: "Environments", TopicID: "environments",
		Question: "Why can't I switch environment while the server is running?",
		Answer: "Switching changes command, port and variables, so it is blocked while a server " +
			"runs to avoid leaving a process with a mixed configuration. Stop the server and try " +
			"again; the switcher offers to stop it for you.",
	},
	{
		ID: "secrets-masked", Category: "Environments", TopicID: "environments",
		Question: "Why do secret values show as ***?",
		Answer: "Keys marked as secret are masked whenever they are read, so they never reach the " +
			"UI, logs or a screenshot by accident. Use the reveal control in the environment " +
			"editor when you need to see or edit the real value.",
	},
	{
		ID: "secrets-storage", Category: "Environments", TopicID: "environments",
		Question: "Are secrets encrypted on disk?",
		Answer: "Not yet. Values live in the dotenv file you choose and in projects.json, so treat " +
			"those files as sensitive: keep them out of git and rely on marking keys as secret to " +
			"prevent accidental exposure in the UI.",
	},
	{
		ID: "tests-fail-first-run", Category: "Testing", TopicID: "playwright",
		Question: "Tests fail when no server is running. How do I fix that?",
		Answer: "Running tests auto-starts the server and waits for the port before invoking " +
			"Playwright. If it still fails, check the Logs tab: usually the startup timeout is too " +
			"low for the first cold build.",
	},
	{
		ID: "no-coverage", Category: "Testing", TopicID: "advanced-testing",
		Question: "The Testing panel says no coverage report found. Why?",
		Answer: "The panel only reads artefacts your project already produced. Generate one of: " +
			"coverage/coverage-summary.json or coverage-final.json (nyc/c8/vitest), a cobertura " +
			"XML, or a Go coverprofile. Nothing is executed by devManager itself.",
	},
	{
		ID: "flaky-tests", Category: "Testing", TopicID: "advanced-testing",
		Question: "How does flaky detection work?",
		Answer: "Every recorded run contributes its failed test names. A test is flaky when it " +
			"failed in some runs and passed in others; tests that fail in every run are reported as " +
			"real failures rather than flaky.",
	},
	{
		ID: "app-log", Category: "Diagnostics", TopicID: "logs",
		Question: "Where do I see devManager's own errors?",
		Answer: "In the app log window (Ctrl+Alt+L or Ctrl+K). It is independent of any project and " +
			"keeps the last 3000 lines of the application's stdout/stderr.",
	},
	{
		ID: "backups-frequency", Category: "Diagnostics", TopicID: "backups",
		Question: "How often are backups created?",
		Answer: "On every startup plus on the interval configured in Settings (off by default), " +
			"with retention limiting how many archives are kept. Manual backups are one click away " +
			"in Settings.",
	},
	{
		ID: "help-shortcut", Category: "Diagnostics", TopicID: "shortcuts",
		Question: "Is there a shortcut to open this help?",
		Answer: "Yes: press F1 to open the help window at any time, or Ctrl+? to jump straight to " +
			"the shortcut list.",
	},
}
