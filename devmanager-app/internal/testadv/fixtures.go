package testadv

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// fixtureEntryCap limita cuántas entradas se devuelven por set (una carpeta de
// fixtures puede tener miles de ficheros y la UI no necesita todos).
const fixtureEntryCap = 200

// Fixture es un fichero o carpeta dentro de un set de datos de test.
type Fixture struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"` // relativo al proyecto, con '/'
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
	IsDir    bool      `json:"isDir"`
}

// FixtureSet es un directorio de datos de test detectado en el proyecto.
type FixtureSet struct {
	Dir       string    `json:"dir"` // relativo al proyecto
	Count     int       `json:"count"`
	Files     int       `json:"files"`
	Bytes     int64     `json:"bytes"`
	Truncated bool      `json:"truncated"`
	Entries   []Fixture `json:"entries"`
}

// fixtureCandidates son los directorios de datos de test habituales.
var fixtureCandidates = []string{
	"test-data",
	filepath.Join("tests", "fixtures"),
	"fixtures",
	filepath.Join("tests", "data"),
	filepath.Join("e2e", "fixtures"),
	filepath.Join("test", "fixtures"),
}

// FixtureDirNames expone los directorios que se inspeccionan (para la UI).
func FixtureDirNames() []string {
	out := make([]string, 0, len(fixtureCandidates))
	for _, c := range fixtureCandidates {
		out = append(out, filepath.ToSlash(c))
	}
	return out
}

// ListFixtures escanea los directorios candidatos y devuelve un set por cada
// uno que exista. Una entrada por fichero/directorio (no recursivo en
// profundidad: los sets anidados se listan como directorios).
func ListFixtures(projectPath string) []FixtureSet {
	out := []FixtureSet{}
	if projectPath == "" {
		return out
	}
	for _, rel := range fixtureCandidates {
		abs := filepath.Join(projectPath, rel)
		st, err := os.Stat(abs)
		if err != nil || !st.IsDir() {
			continue
		}
		set := FixtureSet{Dir: filepath.ToSlash(rel), Entries: []Fixture{}}
		entries, err := os.ReadDir(abs)
		if err != nil {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), ".") {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			set.Count++
			if !e.IsDir() {
				set.Files++
				set.Bytes += info.Size()
			}
			if len(set.Entries) < fixtureEntryCap {
				set.Entries = append(set.Entries, Fixture{
					Name:     e.Name(),
					Path:     filepath.ToSlash(filepath.Join(rel, e.Name())),
					Size:     sizeOf(info, e.IsDir()),
					Modified: info.ModTime(),
					IsDir:    e.IsDir(),
				})
			} else {
				set.Truncated = true
			}
		}
		out = append(out, set)
	}
	return out
}

// sizeOf devuelve el tamaño del fichero o 0 para directorios (evita un
// WalkDir por cada carpeta dentro del listado).
func sizeOf(info fs.FileInfo, isDir bool) int64 {
	if isDir {
		return 0
	}
	return info.Size()
}
