// Package detection porta app/utils/detection.py.
package detection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ProjectConfig replica el dict result de detect_project_config.
type ProjectConfig struct {
	Name              string `json:"name"`
	ServerCommand     string `json:"server_command"`
	Port              int    `json:"port"`
	URL               string `json:"url"`
	PlaywrightEnabled bool   `json:"playwright_enabled"`
}

// prettify convierte "my-project_name" → "My Project Name" (paridad .title()).
func prettify(s string) string {
	return strings.Title(strings.ReplaceAll(strings.ReplaceAll(s, "-", " "), "_", " "))
}

// DetectProjectConfig replica detect_project_config: inspecciona el folder y
// autodetecta nombre/gestor/comando de server/puerto/URL/playwright.
// existingPorts son los puertos ya configurados; evita colisiones sumando.
func DetectProjectConfig(projectPath string, existingPorts []int) ProjectConfig {
	result := ProjectConfig{
		Name:              "",
		ServerCommand:     "npm run dev",
		Port:              5173,
		URL:               "http://localhost:5173",
		PlaywrightEnabled: false,
	}

	if projectPath == "" {
		return result
	}
	if fi, err := os.Stat(projectPath); err != nil || !fi.IsDir() {
		return result
	}

	// Nombre default desde basename del folder
	folderName := filepath.Base(filepath.Clean(projectPath))
	result.Name = prettify(folderName)

	pkgMgr := DetectPackageManager(projectPath)

	// 1. Inspect package.json (ecosistema Node)
	pkgJSONPath := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgJSONPath); err == nil {
		if data, err := os.ReadFile(pkgJSONPath); err == nil {
			var pkg struct {
				Name            string            `json:"name"`
				Scripts         map[string]string `json:"scripts"`
				Dependencies    map[string]string `json:"dependencies"`
				DevDependencies map[string]string `json:"devDependencies"`
			}
			if json.Unmarshal(data, &pkg) == nil {
				if pkg.Name != "" {
					result.Name = prettify(pkg.Name)
				}
				deps := map[string]string{}
				for k, v := range pkg.Dependencies {
					deps[k] = v
				}
				for k, v := range pkg.DevDependencies {
					deps[k] = v
				}

				// Elegir comando dev
				if _, ok := pkg.Scripts["dev"]; ok {
					if pkgMgr == "pnpm" {
						result.ServerCommand = "pnpm dev"
					} else {
						result.ServerCommand = pkgMgr + " run dev"
					}
				} else if _, ok := pkg.Scripts["start"]; ok {
					result.ServerCommand = pkgMgr + " start"
				} else if _, ok := pkg.Scripts["serve"]; ok {
					result.ServerCommand = pkgMgr + " run serve"
				}

				// Firmas de framework → puerto default
				if _, ok := deps["astro"]; ok {
					result.Port = 4321
				} else if _, ok := deps["next"]; ok {
					result.Port = 3000
				} else if _, ok := deps["nuxt"]; ok {
					result.Port = 3000
				} else if _, ok := deps["@nuxt/core"]; ok {
					result.Port = 3000
				} else if _, ok := deps["@angular/core"]; ok {
					result.Port = 4200
				} else if _, ok := deps["@angular/cli"]; ok {
					result.Port = 4200
				} else if _, ok := deps["vite"]; ok {
					result.Port = 5173
				} else if _, ok := deps["@sveltejs/kit"]; ok {
					result.Port = 5173
				} else if _, ok := deps["react-scripts"]; ok {
					result.Port = 3000
				} else if _, ok := deps["@remix-run/react"]; ok {
					result.Port = 3000
				} else if _, ok := deps["vue"]; ok {
					if _, hasVite := deps["vite"]; !hasVite {
						result.Port = 8080
					}
				}

				// Playwright en dependencias
				if _, ok := deps["@playwright/test"]; ok {
					result.PlaywrightEnabled = true
				}
				if _, ok := deps["playwright"]; ok {
					result.PlaywrightEnabled = true
				}
			}
		}
	}

	// 2. Frameworks Python si no hay package.json o hay manage.py
	if _, err := os.Stat(filepath.Join(projectPath, "manage.py")); err == nil {
		result.ServerCommand = "python manage.py runserver"
		result.Port = 8000
	} else if _, err := os.Stat(filepath.Join(projectPath, "main.py")); err == nil {
		if _, notPkg := os.Stat(pkgJSONPath); notPkg != nil {
			result.ServerCommand = "uvicorn main:app --reload"
			result.Port = 8000
		}
	}

	// 3. Override de puerto via .env PORT
	for _, envFile := range []string{".env", ".env.local", ".env.development"} {
		envPath := filepath.Join(projectPath, envFile)
		if data, err := os.ReadFile(envPath); err == nil {
			envRE := regexp.MustCompile(`(?i)^(?:PORT|VITE_PORT|SERVER_PORT)\s*=\s*(\d+)`)
			for _, line := range strings.Split(string(data), "\n") {
				m := envRE.FindStringSubmatch(strings.TrimSpace(line))
				if m != nil {
					if port, perr := strconv.Atoi(m[1]); perr == nil {
						result.Port = port
						break
					}
				}
			}
		}
	}

	// 4. Archivos de config de Playwright
	for _, pw := range []string{"playwright.config.ts", "playwright.config.js", "playwright.config.mjs"} {
		if _, err := os.Stat(filepath.Join(projectPath, pw)); err == nil {
			result.PlaywrightEnabled = true
			break
		}
	}

	// Evitar colisiones con puertos ya configurados
	used := map[int]bool{}
	for _, p := range existingPorts {
		used[p] = true
	}
	for used[result.Port] {
		result.Port++
	}

	result.URL = "http://localhost:" + strconv.Itoa(result.Port)
	return result
}

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
