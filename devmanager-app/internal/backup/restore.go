package backup

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// preRestoreExt sufijo de la copia de seguridad del estado actual antes de
// restaurar (nunca destruir el estado vigente sin red de seguridad).
const preRestoreExt = ".pre-restore"

// Restore valida el backup filename dentro de backupDir y escribe su
// contenido sobre projectsPath/settingsPath. Antes de pisar cada archivo
// existente crea <path>.pre-restore con el contenido vigente. Devuelve la
// lista de archivos restaurados.
func Restore(backupDir, filename, projectsPath, settingsPath string) ([]string, error) {
	if !ValidFilename(filename) {
		return nil, fmt.Errorf("nombre de backup inválido: %q", filename)
	}
	pl, err := readArchive(filepath.Join(backupDir, filename))
	if err != nil {
		return nil, err
	}
	if err := pl.verify(); err != nil {
		return nil, err
	}

	targets := map[string]string{
		NameProjects: projectsPath,
		NameSettings: settingsPath,
	}
	restored := []string{}
	for _, fe := range pl.Manifest.Files {
		dest, ok := targets[fe.Name]
		if !ok || dest == "" {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(pl.Content[fe.Name])
		if err != nil {
			return restored, fmt.Errorf("decodificar %s: %w", fe.Name, err)
		}
		if err := writeWithPreRestore(dest, data); err != nil {
			return restored, err
		}
		restored = append(restored, fe.Name)
	}
	if len(restored) == 0 {
		return nil, fmt.Errorf("el backup no contiene archivos restaurables")
	}
	return restored, nil
}

// writeWithPreRestore escribe data en path; si path existe, antes copia el
// contenido vigente a path+preRestoreExt (con timestamp de escritura).
func writeWithPreRestore(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		current, err := os.ReadFile(path)
		if err == nil { // si la lectura falla, se pisa igual: el backup es la red
			_ = os.WriteFile(path+preRestoreExt, current, 0o644)
		}
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("crear directorio destino: %w", err)
		}
	}
	// Escritura atómica: tmp + rename para no dejar un config a medio escribir.
	tmp := path + ".tmp-" + fmt.Sprint(time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("escribir %s: %w", filepath.Base(path), err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("publicar %s: %w", filepath.Base(path), err)
	}
	return nil
}
