package help

import "sort"

// Shortcut describe un atajo real de la aplicación. Keys usa el formato que se
// muestra en la UI (Ctrl, Shift, Alt, F1, Del...); Context acota cuándo aplica.
type Shortcut struct {
	Keys    string `json:"keys"`
	Action  string `json:"action"`
	Context string `json:"context"`
	Group   string `json:"group"`
}

// Shortcuts devuelve el registro ordenado por grupo y luego por atajo.
// Mantener en sincronía con wireKeyboardShortcuts() de main.js.
func Shortcuts() []Shortcut {
	out := make([]Shortcut, len(shortcuts))
	copy(out, shortcuts)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Group != out[j].Group {
			return out[i].Group < out[j].Group
		}
		return out[i].Keys < out[j].Keys
	})
	return out
}

// ShortcutGroups devuelve los grupos en orden alfabético.
func ShortcutGroups() []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range Shortcuts() {
		if !seen[s.Group] {
			seen[s.Group] = true
			out = append(out, s.Group)
		}
	}
	return out
}

// FindShortcut devuelve el atajo cuya acción contiene el texto dado.
func FindShortcut(action string) (Shortcut, bool) {
	for _, s := range shortcuts {
		if s.Action == action {
			return s, true
		}
	}
	return Shortcut{}, false
}

var shortcuts = []Shortcut{
	// Proyectos
	{Keys: "Ctrl+N", Action: "Add project", Group: "Projects", Context: "Global"},
	{Keys: "Ctrl+E", Action: "Edit selected project", Group: "Projects", Context: "Global"},
	{Keys: "Del", Action: "Remove selected project", Group: "Projects", Context: "Global"},
	{Keys: "Ctrl+F", Action: "Focus the project search box", Group: "Projects", Context: "Global (works inside inputs)"},
	{Keys: "Ctrl+R", Action: "Reload projects.json", Group: "Projects", Context: "Global"},

	// Servers
	{Keys: "F5", Action: "Start or restart the selected server", Group: "Servers", Context: "Global"},
	{Keys: "Shift+F5", Action: "Stop the selected server", Group: "Servers", Context: "Global"},

	// Testing
	{Keys: "Ctrl+T", Action: "Run Playwright tests", Group: "Testing", Context: "Project selected"},
	{Keys: "Ctrl+L", Action: "Clear the project log", Group: "Testing", Context: "Project selected"},

	// Environments
	{Keys: "Ctrl+Shift+E", Action: "Open the environment variables editor", Group: "Environments", Context: "Project selected"},

	// Tools
	{Keys: "Ctrl+Alt+M", Action: "Open the Monitor window", Group: "Tools", Context: "Global"},
	{Keys: "Ctrl+Alt+L", Action: "Open the app log window", Group: "Tools", Context: "Global"},
	{Keys: "Ctrl+K", Action: "Open the app log window", Group: "Tools", Context: "Global"},
	{Keys: "Ctrl+O", Action: "Open project folder in Explorer", Group: "Tools", Context: "Project selected"},
	{Keys: "Ctrl+`", Action: "Open the project terminal", Group: "Tools", Context: "Project selected"},
	{Keys: "Ctrl+Alt+T", Action: "Open the project terminal", Group: "Tools", Context: "Project selected"},
	{Keys: "Ctrl+Shift+T", Action: "Open the project terminal", Group: "Tools", Context: "Project selected"},
	{Keys: "Ctrl+Shift+C", Action: "Open project in VS Code", Group: "Tools", Context: "Project selected"},
	{Keys: "Ctrl+Shift+O", Action: "Open project in OpenCode", Group: "Tools", Context: "Project selected"},
	{Keys: "Ctrl+Shift+R", Action: "Restart devManager", Group: "Tools", Context: "Global"},

	// App
	{Keys: "Ctrl+,", Action: "Open Settings", Group: "Application", Context: "Global (works inside inputs)"},
	{Keys: "Ctrl+Q", Action: "Quit devManager", Group: "Application", Context: "Global"},
	{Keys: "F1", Action: "Open this help window", Group: "Application", Context: "Global"},
	{Keys: "Ctrl+?", Action: "Open the keyboard shortcuts list", Group: "Application", Context: "Global"},
}
