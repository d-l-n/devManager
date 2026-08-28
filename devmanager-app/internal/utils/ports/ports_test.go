package ports

import (
	"context"
	"net"
	"testing"
	"time"
)

func startListener(t *testing.T) (port int, cleanup func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() { ln.Close() }
}

func TestIsPortOpen(t *testing.T) {
	port, cleanup := startListener(t)
	defer cleanup()

	if !IsPortOpen("127.0.0.1", port) {
		t.Error("puerto escuchando debe reportarse abierto")
	}
	if IsPortOpen("127.0.0.1", 1) {
		t.Error("puerto 1 no deber├¡a estar abierto")
	}
	if IsPortOpen("", 8080) || IsPortOpen("127.0.0.1", 0) || IsPortOpen("127.0.0.1", 70000) {
		t.Error("inputs inv├ílidos deben devolver false")
	}
}

func TestBuildServerCommand(t *testing.T) {
	cases := []struct {
		name, cmd string
		port      int
		want      string
	}{
		{"pnpm", "pnpm dev", 3000, "pnpm dev --port 3000"},
		{"npm run", "npm run dev", 3000, "npm run dev -- --port 3000"},
		{"npm start", "npm start", 3000, "npm start -- --port 3000"},
		{"yarn", "yarn dev", 3000, "yarn dev --port 3000"},
		{"bun run", "bun run dev", 3000, "bun run dev --port 3000"},
		{"bun dev", "bun dev", 3000, "bun dev --port 3000"},
		{"vite", "vite", 3000, "vite --port 3000"},
		{"npx vite", "npx vite", 3000, "npx vite --port 3000"},
		{"next dev", "next dev", 3000, "next dev -p 3000"},
		{"npx next dev", "npx next dev", 3000, "npx next dev -p 3000"},
		{"astro dev", "astro dev", 3000, "astro dev --port 3000"},
		{"django", "python manage.py runserver", 8000, "python manage.py runserver 8000"},
		{"uvicorn", "uvicorn main:app", 8000, "uvicorn main:app --port 8000"},
		{"flask", "flask run", 5000, "flask run --port 5000"},
		{"ya tiene flag", "vite --port 1111", 3000, "vite --port 1111"},
		{"flag corto", "vite -p 1111", 3000, "vite -p 1111"},
		{"no matchea vitest como vite", "vitest", 3000, "vitest"},
		{"desconocido intacto", "make serve", 3000, "make serve"},
		{"port inv├ílido", "vite", 0, "vite"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildServerCommand(tc.cmd, tc.port)
			if got != tc.want {
				t.Errorf("BuildServerCommand(%q, %d) = %q, want %q", tc.cmd, tc.port, got, tc.want)
			}
		})
	}
}

func TestWaitForPortReady(t *testing.T) {
	port, cleanup := startListener(t)
	defer cleanup()

	err := WaitForPort(context.Background(), "127.0.0.1", port, 3*time.Second, 50*time.Millisecond)
	if err != nil {
		t.Errorf("puerto abierto debe resolver sin error, got %v", err)
	}
}

func TestWaitForPortTimeout(t *testing.T) {
	err := WaitForPort(context.Background(), "127.0.0.1", 1, 300*time.Millisecond, 50*time.Millisecond)
	if err == nil {
		t.Error("debe expirar con error")
	}
}

func TestWaitForPortCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	err := WaitForPort(ctx, "127.0.0.1", 1, 30*time.Second, 25*time.Millisecond)
	if err == nil {
		t.Fatal("cancelaci├│n debe producir error")
	}
	if time.Since(start) > time.Second {
		t.Error("cancel debe interrumpir antes del timeout total")
	}
}
