package detection

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDetectUserCommand(t *testing.T) {
	cases := []struct {
		name string
		// setup recibe el path del proyecto temporal; dtype dice qué señal crear.
		setup func(t *testing.T, dir string)
		want  string
	}{
		{
			name: "npm script create-user",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "package.json"),
					`{"name":"demo","scripts":{"dev":"vite","create-user":"node scripts/create-user.js"}}`)
			},
			want: "npm run create-user",
		},
		{
			name: "pnpm script create:user",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")
				writeTestFile(t, filepath.Join(dir, "package.json"),
					`{"scripts":{"create:user":"tsx scripts/createUser.ts"}}`)
			},
			want: "pnpm create:user",
		},
		{
			name: "yarn script user:create",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "yarn.lock"), "")
				writeTestFile(t, filepath.Join(dir, "package.json"),
					`{"scripts":{"user:create":"node scripts/user.js"}}`)
			},
			want: "yarn user:create",
		},
		{
			name: "npm seed-user script",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "package.json"),
					`{"scripts":{"seed-user":"node scripts/seed.js"}}`)
			},
			want: "npm run seed-user",
		},
		{
			name: "unrelated script falls back to file",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "package.json"),
					`{"scripts":{"dev":"vite"}}`)
				writeTestFile(t, filepath.Join(dir, "scripts", "create-user.js"), "")
			},
			want: "node scripts/create-user.js", // sin script match → fallback archivo
		},
		{
			name: "fallback node file scripts/create-user.js",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "scripts", "create-user.js"), "")
			},
			want: "node scripts/create-user.js",
		},
		{
			name: "fallback python nested",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "server", "scripts", "create_user.py"), "")
			},
			want: "python server/scripts/create_user.py",
		},
		{
			name: "fallback go cmd",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "cmd", "createuser", "main.go"), "")
			},
			want: "go run cmd/createuser/main.go",
		},
		{
			name: "root file",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "create-user.mjs"), "")
			},
			want: "node create-user.mjs",
		},
		{
			name: "add-user script",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "package.json"),
					`{"scripts":{"add-user":"node scripts/add.js"}}`)
			},
			want: "npm run add-user",
		},
		{
			name: "playwright file ignored",
			setup: func(t *testing.T, dir string) {
				writeTestFile(t, filepath.Join(dir, "tests", "create-user.spec.ts"), "")
			},
			want: "", // tests/ no es carpeta candidata
		},
		{
			name:  "empty dir",
			setup: func(t *testing.T, dir string) {},
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			if got := DetectUserCommand(dir); got != tc.want {
				t.Errorf("DetectUserCommand() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDetectUserCommandBadInput(t *testing.T) {
	if got := DetectUserCommand(""); got != "" {
		t.Errorf("path vacío debe devolver \"\", got %q", got)
	}
	nonDir := filepath.Join(t.TempDir(), "no-existe")
	if got := DetectUserCommand(nonDir); got != "" {
		t.Errorf("path inexistente debe devolver \"\", got %q", got)
	}
}