package obscura

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// sha256Hex calcula el digest de un archivo.
func sha256Hex(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestVerifySHA256(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.bin")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	good := sha256Hex(t, path)
	if err := verifySHA256(path, good); err != nil {
		t.Errorf("digest correcto debería pasar: %v", err)
	}
	if err := verifySHA256(path, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Error("digest incorrecto debería fallar")
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "x.zip")
	zf, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("obscura.exe")
	if _, err := w.Write([]byte("BINRENDER")); err != nil {
		t.Fatal(err)
	}
	w2, _ := zw.Create("obscura-worker.exe")
	if _, err := w2.Write([]byte("WORKER")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zf.Close()

	if err := extractBinary(assetSpec{FileName: "x.zip", Archive: "zip", Binary: "obscura.exe"}, dir); err != nil {
		t.Fatalf("extract zip: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "obscura.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "BINRENDER" {
		t.Errorf("contenido = %q, want BINRENDER", b)
	}
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "x.tar.gz")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{Name: "obscura", Mode: 0o755, Size: int64(len("BINRENDER"))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte("BINRENDER")); err != nil {
		t.Fatal(err)
	}
	tw.Close()
	gw.Close()
	f.Close()

	if err := extractBinary(assetSpec{FileName: "x.tar.gz", Archive: "targz", Binary: "obscura"}, dir); err != nil {
		t.Fatalf("extract targz: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "obscura"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "BINRENDER" {
		t.Errorf("contenido = %q, want BINRENDER", b)
	}
}

func TestExtractMissingBinary(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "x.zip")
	zf, _ := os.Create(archivePath)
	zw := zip.NewWriter(zf)
	w, _ := zw.Create("README.md")
	if _, err := w.Write([]byte("docs")); err != nil {
		t.Fatal(err)
	}
	zw.Close()
	zf.Close()
	if err := extractFromZip(archivePath, "obscura.exe", dir); err == nil {
		t.Error("debería fallar si el binario no está en el zip")
	}
}

func TestManagerEmptyInputs(t *testing.T) {
	m := NewManager(Callbacks{})
	if err := m.Screenshot("", "out.png"); err == nil {
		t.Error("Screenshot sin URL debería fallar")
	}
	if err := m.Dump("", "text"); err == nil {
		t.Error("Dump sin URL debería fallar")
	}
	if err := m.Eval("http://x", ""); err == nil {
		t.Error("Eval sin JS debería fallar")
	}
	// Estado error tras un fallo solicitado.
	if m.State() != StateError {
		t.Errorf("state = %s, want error", m.State())
	}
}

func TestJoinForLog(t *testing.T) {
	got := joinForLog([]string{"fetch", "http://a", "--allow-private-network", "-e", "1 + 1"})
	want := `"fetch" "http://a" "--allow-private-network" "-e" "1 + 1"`
	if got != want {
		t.Errorf("joinForLog = %s, want %s", got, want)
	}
}