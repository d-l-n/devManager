package obscura

import "testing"

func TestAssetForPlatform(t *testing.T) {
	cases := []struct {
		goos   string
		goarch string
		file   string
		arc    string
		bin    string
		shaLen int
		ok     bool
	}{
		{"windows", "amd64", "obscura-x86_64-windows.zip", "zip", "obscura.exe", 64, true},
		{"linux", "amd64", "obscura-x86_64-linux.tar.gz", "targz", "obscura", 64, true},
		{"linux", "arm64", "obscura-aarch64-linux.tar.gz", "targz", "obscura", 64, true},
		{"darwin", "amd64", "obscura-x86_64-macos.tar.gz", "targz", "obscura", 64, true},
		{"darwin", "arm64", "obscura-aarch64-macos.tar.gz", "targz", "obscura", 64, true},
		{"windows", "arm64", "", "", "", 0, false},
		{"windows", "386", "", "", "", 0, false},
		{"linux", "386", "", "", "", 0, false},
		{"freebsd", "amd64", "", "", "", 0, false},
		{"windows", "amd64", "obscura-x86_64-windows.zip", "zip", "obscura.exe", 64, true},
	}
	for _, c := range cases {
		got, err := assetForPlatform(c.goos, c.goarch)
		if c.ok {
			if err != nil {
				t.Errorf("%s/%s: unexpected error: %v", c.goos, c.goarch, err)
				continue
			}
			if got.FileName != c.file || got.Archive != c.arc || got.Binary != c.bin {
				t.Errorf("%s/%s = %+v, want %s/%s/%s", c.goos, c.goarch, got, c.file, c.arc, c.bin)
			}
			if len(got.SHA256) != c.shaLen {
				t.Errorf("%s/%s: sha256 len = %d, want %d", c.goos, c.goarch, len(got.SHA256), c.shaLen)
			}
		} else {
			if err == nil {
				t.Errorf("%s/%s: expected error, got %+v", c.goos, c.goarch, got)
			}
		}
	}
}