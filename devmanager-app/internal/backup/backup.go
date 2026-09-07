// Package backup implementa el motor de backups locales (Issue #71):
// archivos .dmbak (gzip) con un manifiesto + los archivos de configuración
// embebidos (base64, byte-exact), catálogo con validación sha256, retención
// y restore con copia de seguridad .pre-restore.
package backup

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

const (
	// ArchiveExt es la extensión de los archivos de backup.
	ArchiveExt = ".dmbak"
	// ManifestVersion es la versión del schema del manifiesto embebido.
	ManifestVersion = 1
	// NameProjects / NameSettings son las keys canónicas de los archivos
	// embebidos dentro del backup.
	NameProjects = "projects.json"
	NameSettings = "settings.json"
)

// FileEntry describe un archivo embebido en el backup.
type FileEntry struct {
	Name   string `json:"name"`
	Size   int    `json:"size"`
	SHA256 string `json:"sha256"`
}

// Manifest describe el contenido de un backup.
type Manifest struct {
	Version    int         `json:"version"`
	CreatedAt  time.Time   `json:"created_at"`
	AppVersion string      `json:"app_version,omitempty"`
	Files      []FileEntry `json:"files"`
}

// payload es el documento JSON (comprimido con gzip) que vive dentro del .dmbak.
// El contenido va en base64 para restaurar byte-exacto aunque el archivo de
// origen tenga bytes no-UTF8.
type payload struct {
	Manifest Manifest          `json:"manifest"`
	Content  map[string]string `json:"content"` // name -> base64 del archivo
}

// Entry es la vista pública de un backup del catálogo (binding → frontend).
type Entry struct {
	Filename         string `json:"filename"`
	CreatedAt        string `json:"createdAt"` // RFC3339
	SizeBytes        int64  `json:"sizeBytes"`
	AppVersion       string `json:"appVersion"`
	Valid            bool   `json:"valid"`
	ProjectsCount    int    `json:"projectsCount"`
	SettingsIncluded bool   `json:"settingsIncluded"`
	Error            string `json:"error,omitempty"`
}

// filenameRe valida nombres generados por CreateBackup (sin paths ni traversal).
var filenameRe = regexp.MustCompile(`^backup-\d{8}-\d{6}(-\d+)?\.dmbak$`)

// ValidFilename reporta si name tiene el formato exacto de un backup creado
// por este paquete. Usado por la capa de bindings para rechazar input
// arbitrario del frontend antes de tocar el filesystem.
func ValidFilename(name string) bool {
	return filenameRe.MatchString(name)
}

// CreateBackup empaqueta projectsPath (obligatorio) y settingsPath (opcional,
// se omite si no existe) en backupDir/backup-<timestamp>.dmbak y devuelve el
// nombre del archivo generado. now es inyectable para tests.
func CreateBackup(backupDir, projectsPath, settingsPath, appVersion string, now time.Time) (string, error) {
	projData, err := os.ReadFile(projectsPath)
	if err != nil {
		return "", fmt.Errorf("leer projects.json: %w", err)
	}

	pl := payload{
		Manifest: Manifest{
			Version:    ManifestVersion,
			CreatedAt:  now.UTC(),
			AppVersion: appVersion,
		},
		Content: map[string]string{},
	}
	pl.addFile(NameProjects, projData)
	if setData, err := os.ReadFile(settingsPath); err == nil {
		pl.addFile(NameSettings, setData)
	}

	jsonBytes, err := json.MarshalIndent(pl, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializar manifiesto: %w", err)
	}

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return "", fmt.Errorf("crear directorio de backups: %w", err)
	}

	name := uniqueName(backupDir, now)
	final := filepath.Join(backupDir, name)
	tmp := final + ".tmp"

	if err := writeGzip(tmp, jsonBytes); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("publicar backup: %w", err)
	}
	return name, nil
}

// addFile registra name con checksum sha256 y contenido base64 en el payload.
func (pl *payload) addFile(name string, data []byte) {
	sum := sha256.Sum256(data)
	pl.Manifest.Files = append(pl.Manifest.Files, FileEntry{
		Name:   name,
		Size:   len(data),
		SHA256: hex.EncodeToString(sum[:]),
	})
	pl.Content[name] = base64.StdEncoding.EncodeToString(data)
}

// uniqueName genera backup-<fecha>.dmbak; si ya existe (dos backups en el
// mismo segundo) añade sufijo -1, -2… para no pisar backups previos.
func uniqueName(backupDir string, now time.Time) string {
	base := "backup-" + now.Format("20060102-150405")
	name := base + ArchiveExt
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(backupDir, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s-%d%s", base, i, ArchiveExt)
	}
}

// writeGzip escribe data comprimido en path (atómico vía tmp + rename por el caller).
func writeGzip(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("crear archivo de backup: %w", err)
	}
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(data); err != nil {
		gz.Close()
		f.Close()
		return fmt.Errorf("comprimir backup: %w", err)
	}
	if err := gz.Close(); err != nil {
		f.Close()
		return fmt.Errorf("cerrar gzip: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("cerrar archivo de backup: %w", err)
	}
	return nil
}

// readArchive abre y parsea un .dmbak. gzip.Reader valida su propio CRC32,
// lo que detecta archivos truncados o corruptos a nivel gzip.
func readArchive(path string) (*payload, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("abrir backup: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip inválido: %w", err)
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("leer backup: %w", err)
	}
	var pl payload
	if err := json.Unmarshal(data, &pl); err != nil {
		return nil, fmt.Errorf("manifiesto inválido: %w", err)
	}
	if pl.Manifest.Version != ManifestVersion {
		return nil, fmt.Errorf("versión de manifiesto no soportada: %d", pl.Manifest.Version)
	}
	if len(pl.Manifest.Files) == 0 || pl.Content == nil {
		return nil, fmt.Errorf("manifiesto sin archivos embebidos")
	}
	return &pl, nil
}

// verify comprueba que cada archivo del manifiesto esté presente y su sha256
// coincida con el contenido embebido (detección de corrupción post-gzip).
func (pl *payload) verify() error {
	for _, fe := range pl.Manifest.Files {
		b64, ok := pl.Content[fe.Name]
		if !ok {
			return fmt.Errorf("falta el contenido de %s", fe.Name)
		}
		data, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return fmt.Errorf("contenido de %s no es base64 válido: %w", fe.Name, err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != fe.SHA256 {
			return fmt.Errorf("checksum de %s no coincide (backup corrupto)", fe.Name)
		}
		if fe.Size != len(data) {
			return fmt.Errorf("tamaño de %s no coincide con el manifiesto", fe.Name)
		}
	}
	return nil
}

// ValidateFile valida un .dmbak y devuelve su Entry con Valid/Error.
func ValidateFile(path string) (Entry, error) {
	base := filepath.Base(path)
	st, err := os.Stat(path)
	if err != nil {
		return Entry{Filename: base, Valid: false, Error: err.Error()}, err
	}
	entry := Entry{
		Filename:  base,
		CreatedAt: st.ModTime().UTC().Format(time.RFC3339),
		SizeBytes: st.Size(),
	}
	pl, err := readArchive(path)
	if err != nil {
		entry.Error = err.Error()
		return entry, err
	}
	entry.CreatedAt = pl.Manifest.CreatedAt.UTC().Format(time.RFC3339)
	entry.AppVersion = pl.Manifest.AppVersion
	entry.ProjectsCount = countProjects(pl)
	_, entry.SettingsIncluded = pl.Content[NameSettings]
	if err := pl.verify(); err != nil {
		entry.Error = err.Error()
		return entry, err
	}
	entry.Valid = true
	return entry, nil
}

// countProjects cuenta los proyectos dentro del projects.json embebido
// (0 si el contenido no parsea; el backup sigue siendo válido: se respaldó
// byte-exacto lo que había en disco).
func countProjects(pl *payload) int {
	b64, ok := pl.Content[NameProjects]
	if !ok {
		return 0
	}
	data, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return 0
	}
	var ff struct {
		Projects []json.RawMessage `json:"projects"`
	}
	if json.Unmarshal(data, &ff) != nil {
		return 0
	}
	return len(ff.Projects)
}
