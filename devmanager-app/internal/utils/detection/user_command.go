package detection

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// userCmdScriptRE normaliza: "create-user"→"createuser", "create:user"→
// "createuser", "user:create"→"usercreate". Case-insensitive. Los nombres de
// script que matchean se consideran comandos de creación de usuario.
var userCmdScriptRE = regexp.MustCompile(`(?i)^create[-_:]?user|^user[-_:]?create|^add[-_:]?user|^new[-_:]?user|^seed[-_:]?user`)

// userCmdFileRE matchea basenames de archivos candidatos:
// create-user.js, createUser.ts, create_user.py, add-user.js...
var userCmdFileRE = regexp.MustCompile(`(?i)create[-_:]?user|user[-_:]?create|add[-_:]?user`)

var userCmdDirs = []string{"scripts", "tools", "bin", "cmd", "server/scripts"}

// DetectUserCommand infiere el comando de creación de usuario del proyecto
// (aquel que consume las env vars DM_USER_EMAIL, DM_USER_NAME,
// DM_USER_PASSWORD, DM_USER_ROLE). Heurística, en orden:
//
//  1. package.json#scripts con nombre convencional (create-user, create:user,
//     user:create, add-user, seed-user, ...) → comando con el gestor detectado
//     (npm run / pnpm / yarn / bun run).
//  2. Archivos candidatos en carpetas convencionales (scripts, tools, bin,
//     cmd, server/scripts, profundidad ≤ 2) cuyo basename matchee
//     create-user/createUser/create_user/add-user... → node|python|go run <rel>.
//
// Devuelve "" si no encuentra nada (el usuario debe configurarlo a mano).
func DetectUserCommand(projectPath string) string {
	if projectPath == "" {
		return ""
	}
	if fi, err := os.Stat(projectPath); err != nil || !fi.IsDir() {
		return ""
	}

	if cmd := detectUserCmdFromScripts(projectPath); cmd != "" {
		return cmd
	}
	return detectUserCmdFromFiles(projectPath)
}

// detectUserCmdFromScripts busca en package.json#scripts un nombre convencional.
func detectUserCmdFromScripts(projectPath string) string {
	pkgJSONPath := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgJSONPath); err != nil {
		return ""
	}
	raw := readScriptsOrdered(pkgJSONPath)
	if raw == nil {
		return ""
	}
	pkgMgr := DetectPackageManager(projectPath)
	for _, s := range raw {
		if userCmdScriptRE.MatchString(s.Name) {
			return packageRunCommand(pkgMgr, s.Name)
		}
	}
	return ""
}

// packageRunCommand replica el prefijo de GetProjectScripts.
func packageRunCommand(pkgMgr, name string) string {
	switch pkgMgr {
	case "npm":
		if name == "start" || name == "test" {
			return "npm " + name
		}
		return "npm run " + name
	case "pnpm":
		return "pnpm " + name
	case "yarn":
		return "yarn " + name
	case "bun":
		return "bun run " + name
	default:
		return pkgMgr + " run " + name
	}
}

// detectUserCmdFromFiles escanea carpetas convencionales (profundidad ≤ 2) por
// archivos tipo create-user.* / add-user.* y arma el comando según el lenguaje.
// La raíz del proyecto (p.ej. create-user.js en la carpeta raíz) también cuenta.
func detectUserCmdFromFiles(projectPath string) string {
	roots := append([]string{"."}, userCmdDirs...)
	for _, root := range roots {
		base := filepath.Join(projectPath, filepath.FromSlash(root))
		info, err := os.Stat(base)
		if err != nil || !info.IsDir() {
			continue
		}
		if root == "." {
			if cmd, ok := matchUserCmdFile(projectPath, base, ""); ok {
				return cmd
			}
			continue
		}
		// depth 1: scripts/create-user.js
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() {
				continue // depth 2 se resuelve en la siguiente pasada
			}
			if userCmdFileRE.MatchString(name) {
				rel := filepath.ToSlash(filepath.Join(root, name))
				if cmd := userFileCommand(rel, name); cmd != "" {
					return cmd
				}
			}
		}
	}
	// depth 2: scripts/admin/create-user.js (solo subcarpetas de userCmdDirs)
	for _, root := range userCmdDirs {
		base := filepath.Join(projectPath, filepath.FromSlash(root))
		subs, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, sub := range subs {
			if !sub.IsDir() {
				continue
			}
			subPath := filepath.Join(base, sub.Name())
			if cmd, ok := matchUserCmdFile(projectPath, subPath, filepath.Join(root, sub.Name())); ok {
				return cmd
			}
		}
	}
	return ""
}

// matchUserCmdFile matchea archivos de userCmdFileRE dentro de dir y devuelve
// el comando armado. relPrefix es la ruta relativa del dir ("" para la raíz).
func matchUserCmdFile(projectPath, dir, relPrefix string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !userCmdFileRE.MatchString(name) {
			// cmd/createuser/main.go: el basename es main pero el dir matchea.
			dirMatches := userCmdFileRE.MatchString(filepath.Base(filepath.Clean(dir)))
			if !dirMatches || !isMainFile(name) {
				continue
			}
		}
		rel := filepath.ToSlash(filepath.Join(relPrefix, name))
		if cmd := userFileCommand(rel, name); cmd != "" {
			return cmd, true
		}
	}
	return "", false
}

// isMainFile discrimina "main.go"/"main.py"/"main.js"...
func isMainFile(name string) bool {
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	return stem == "main"
}

// userFileCommand arma el comando según la extensión del archivo candidato.
func userFileCommand(rel, name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".go":
		return "go run " + rel
	case ".py":
		return "python " + rel
	case ".js", ".mjs", ".cjs", ".ts":
		return "node " + rel
	default:
		return ""
	}
}