package backup

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List escanea backupDir y devuelve el catálogo de backups ordenado de más
// nuevo a más viejo (por CreatedAt del manifiesto). Los inválidos se incluyen
// con Valid=false + Error para que el usuario pueda verlos y borrarlos.
func List(backupDir string) []Entry {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return []Entry{}
	}
	out := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ArchiveExt) {
			continue
		}
		entry, _ := ValidateFile(filepath.Join(backupDir, e.Name()))
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt > out[j].CreatedAt
	})
	return out
}

// ApplyRetention elimina los backups más viejos más allá de keep y devuelve
// cuántos borró. Los inválidos se borran primero (basura > antigüedad).
func ApplyRetention(backupDir string, keep int) (int, error) {
	if keep <= 0 {
		return 0, nil
	}
	entries := List(backupDir)
	removed := 0
	for i, e := range entries {
		if i < keep && e.Valid {
			continue
		}
		if err := os.Remove(filepath.Join(backupDir, e.Filename)); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}
