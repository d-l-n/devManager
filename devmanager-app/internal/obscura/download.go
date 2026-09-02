package obscura

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// BinDir devuelve el directorio donde vive el binario de Obscura:
// %APPDATA%\devManager\obscura (Usuario ConfigDir + devManager).
func BinDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "devManager", "obscura")
}

// BinName devuelve el nombre del ejecutable según la plataforma.
func BinName() string {
	if isWindows() {
		return "obscura.exe"
	}
	return "obscura"
}

// BinPath devuelve la ruta completa del binario instalado.
func BinPath() string {
	return filepath.Join(BinDir(), BinName())
}

// HasBinary comprueba si el binario ya está instalado.
func HasBinary() bool {
	info, err := os.Stat(BinPath())
	return err == nil && !info.IsDir()
}

// Install descarga, verifica SHA-256 y extrae el binario de Obscura.
// progress es opcional y se llama con un entero 0-100 durante la descarga.
func Install(progress func(int)) error {
	asset, err := currentAsset()
	if err != nil {
		return err
	}

	dir := BinDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("crear directorio %s: %w", dir, err)
	}

	archivePath := filepath.Join(dir, asset.FileName)
	if err := downloadFile(releaseBaseURL+"/"+asset.FileName, archivePath, progress); err != nil {
		return err
	}

	if err := verifySHA256(archivePath, asset.SHA256); err != nil {
		os.Remove(archivePath)
		return err
	}

	if err := extractBinary(asset, dir); err != nil {
		os.Remove(archivePath)
		return err
	}

	// El archivo extraído puede quedar sin permisos de ejecución en Unix.
	if !isWindows() {
		_ = os.Chmod(filepath.Join(dir, asset.Binary), 0o755)
	}
	// Limpiar archive tras extraer correctamente.
	os.Remove(archivePath)

	if progress != nil {
		progress(100)
	}
	return nil
}

func downloadFile(url, dest string, progress func(int)) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("descargar %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("descargar %s: HTTP %d", url, resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("crear archivo %s: %w", dest, err)
	}
	defer out.Close()

	if progress != nil {
		total := resp.ContentLength
		tee := &progressReader{r: resp.Body, total: total, fn: progress}
		_, err = io.Copy(out, tee)
		return err
	}
	_, err = io.Copy(out, resp.Body)
	return err
}

type progressReader struct {
	r     io.Reader
	total int64
	read  int64
	fn    func(int)
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if p.total > 0 && p.fn != nil {
		p.fn(int(float64(p.read) / float64(p.total) * 100))
	}
	return n, err
}

func verifySHA256(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash de %s: %w", path, err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("sha256 mismatch para %s: got %s want %s", path, got, wantHex)
	}
	return nil
}

func extractBinary(asset assetSpec, dir string) error {
	archivePath := filepath.Join(dir, asset.FileName)
	switch asset.Archive {
	case "zip":
		return extractFromZip(archivePath, asset.Binary, dir)
	case "targz":
		return extractFromTarGz(archivePath, asset.Binary, dir)
	default:
		return fmt.Errorf("formato de archivo desconocido: %s", asset.Archive)
	}
}

func extractFromZip(archivePath, want, dir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("abrir zip: %w", err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != want && filepath.Base(f.Name) != want {
			continue
		}
		return writeZipEntry(f, filepath.Join(dir, want))
	}
	return fmt.Errorf("binario %s no encontrado en %s", want, archivePath)
}

func writeZipEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

func extractFromTarGz(archivePath, want, dir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("abrir gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("leer tar: %w", err)
		}
		if hdr.Name != want && filepath.Base(hdr.Name) != want {
			continue
		}
		out, err := os.Create(filepath.Join(dir, want))
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, tr)
		return err
	}
	return fmt.Errorf("binario %s no encontrado en %s", want, archivePath)
}

func isWindows() bool {
	return runtime.GOOS == "windows"
}