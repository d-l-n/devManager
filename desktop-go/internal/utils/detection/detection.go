// Package detection porta app/utils/detection.py (Fase 1: solo logs).
package detection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var portURLRe = regexp.MustCompile(`(?i)https?://(?:localhost|127\.0\.0\.1|0\.0\.0\.0|\[::1\]|\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):(\d{2,5})`)

// ExtractPortFromLog replica extract_port_from_log.
// Devuelve 0 si no hay match o el puerto está fuera de rango.
func ExtractPortFromLog(line string) int {
	if line == "" {
		return 0
	}
	m := portURLRe.FindStringSubmatch(line)
	if m == nil {
		return 0
	}
	port, err := strconv.Atoi(m[1])
	if err != nil || port <= 0 || port > 65535 {
		return 0
	}
	return port
}

// Script es una entrada ordenada de package.json#scripts.
type Script struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// DetectPackageManager replica detect_package_manager.
func DetectPackageManager(projectPath string) string {
	if projectPath == "" {
		return "npm"
	}
	if fi, err := os.Stat(projectPath); err != nil || !fi.IsDir() {
		return "npm"
	}
	if exists(filepath.Join(projectPath, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if exists(filepath.Join(projectPath, "yarn.lock")) {
		return "yarn"
	}
	if exists(filepath.Join(projectPath, "bun.lockb")) {
		return "bun"
	}
	return "npm"
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readScriptsOrdered decodifica package.json preservando el orden de
// declaración de "scripts" (json.Unmarshal sobre map lo perdería).
// Devuelve nil si el JSON es inválido o la clave falta (paridad try/except).
func readScriptsOrdered(pkgJSONPath string) []Script {
	f, err := os.Open(pkgJSONPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	tok, err := dec.Token()
	if err != nil {
		return nil
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil
		}
		key, _ := keyTok.(string)
		if key != "scripts" {
			var skip interface{}
			if err := dec.Decode(&skip); err != nil {
				return nil
			}
			continue
		}
		inner, err := dec.Token() // '{'
		if err != nil {
			return nil
		}
		if d, ok := inner.(json.Delim); !ok || d != '{' {
			return nil
		}
		var out []Script
		for dec.More() {
			nameTok, err := dec.Token()
			if err != nil {
				return nil
			}
			name, _ := nameTok.(string)
			var cmd string
			if err := dec.Decode(&cmd); err != nil {
				return nil
			}
			out = append(out, Script{Name: name, Command: cmd})
		}
		if _, err := dec.Token(); err != nil { // '}'
			return nil
		}
		return out
	}
	return nil
}

// GetProjectScripts replica get_project_scripts: nombre → comando completo
// según el gestor detectado. Orden de package.json preservado.
func GetProjectScripts(projectPath string) []Script {
	if projectPath == "" {
		return nil
	}
	if fi, err := os.Stat(projectPath); err != nil || !fi.IsDir() {
		return nil
	}
	pkgJSONPath := filepath.Join(projectPath, "package.json")
	raw := readScriptsOrdered(pkgJSONPath)
	if raw == nil {
		return nil
	}

	pkgMgr := DetectPackageManager(projectPath)
	out := make([]Script, 0, len(raw))
	for _, s := range raw {
		var runCmd string
		switch pkgMgr {
		case "npm":
			if s.Name == "start" || s.Name == "test" {
				runCmd = "npm " + s.Name
			} else {
				runCmd = "npm run " + s.Name
			}
		case "pnpm":
			runCmd = "pnpm " + s.Name
		case "yarn":
			runCmd = "yarn " + s.Name
		case "bun":
			runCmd = "bun run " + s.Name
		default:
			runCmd = pkgMgr + " run " + s.Name
		}
		out = append(out, Script{Name: s.Name, Command: runCmd})
	}
	return out
}
