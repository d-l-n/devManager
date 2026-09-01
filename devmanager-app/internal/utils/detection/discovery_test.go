package detection

import (
	"os"
	"path/filepath"
	"testing"
)

func writeDiscoveryFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverProjectsShallowAndExcludesConfigured(t *testing.T) {
	root := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(root, "alpha", "package.json"), `{"name":"alpha"}`)
	writeDiscoveryFile(t, filepath.Join(root, "beta", "manage.py"), "")
	writeDiscoveryFile(t, filepath.Join(root, "alpha", "nested", "package.json"), `{}`)
	writeDiscoveryFile(t, filepath.Join(root, "node_modules", "ignored", "package.json"), `{}`)

	got := DiscoverProjects(root, []string{filepath.Join(root, "BETA")})
	if len(got) != 1 || got[0].Config.Name != "Alpha" {
		t.Fatalf("got %#v", got)
	}
}

func TestDiscoverProjectsSortsAndRejectsInvalidRoot(t *testing.T) {
	root := t.TempDir()
	writeDiscoveryFile(t, filepath.Join(root, "zeta", "package.json"), `{"name":"zeta"}`)
	writeDiscoveryFile(t, filepath.Join(root, "alpha", "package.json"), `{"name":"alpha"}`)

	got := DiscoverProjects(root, nil)
	if len(got) != 2 || got[0].Config.Name != "Alpha" {
		t.Fatalf("got %#v", got)
	}
	if got := DiscoverProjects(filepath.Join(root, "missing"), nil); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestDiscoverProjectsSignalsAndSkippedDirectories(t *testing.T) {
	tests := []struct {
		name      string
		directory string
		setup     func(t *testing.T, root string)
		wantFound bool
	}{
		{
			name:      "recognizes main.py",
			directory: "python-app",
			setup: func(t *testing.T, root string) {
				writeDiscoveryFile(t, filepath.Join(root, "python-app", "main.py"), "")
			},
			wantFound: true,
		},
		{
			name:      "recognizes .git directory",
			directory: "git-project",
			setup: func(t *testing.T, root string) {
				if err := os.MkdirAll(filepath.Join(root, "git-project", ".git"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			wantFound: true,
		},
		{
			name:      "skips vendor",
			directory: "vendor",
			setup: func(t *testing.T, root string) {
				writeDiscoveryFile(t, filepath.Join(root, "vendor", "package.json"), `{}`)
			},
		},
		{
			name:      "skips build",
			directory: "build",
			setup: func(t *testing.T, root string) {
				writeDiscoveryFile(t, filepath.Join(root, "build", "package.json"), `{}`)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			test.setup(t, root)

			got := DiscoverProjects(root, nil)
			if !test.wantFound {
				if len(got) != 0 {
					t.Fatalf("got %#v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %#v", got)
			}
			wantPath := filepath.Join(root, test.directory)
			if got[0].Path != wantPath {
				t.Errorf("path = %q, want %q", got[0].Path, wantPath)
			}
		})
	}
}
