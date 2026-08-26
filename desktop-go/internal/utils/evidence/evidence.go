// Package evidence scans Playwright test artifacts and locates HTML reports.
package evidence

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	KindImage = "image"
	KindVideo = "video"
	KindTrace = "trace"
)

var imageExts = map[string]struct{}{".png": {}, ".jpg": {}, ".jpeg": {}}
var videoExts = map[string]struct{}{".webm": {}}
var traceExts = map[string]struct{}{".zip": {}}

var skipDirs = map[string]struct{}{
	"node_modules": {},
	".git":         {},
	"venv":         {},
	".venv":        {},
	"__pycache__":  {},
}

type File struct {
	Path    string `json:"path"`
	RelPath string `json:"relPath"`
	Kind    string `json:"kind"`
	TestDir string `json:"testDir"`
	MTime   int64  `json:"mtime"`
}

func classify(ext string) string {
	switch {
	case contains(imageExts, ext):
		return KindImage
	case contains(videoExts, ext):
		return KindVideo
	case contains(traceExts, ext):
		return KindTrace
	default:
		return ""
	}
}

func contains(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}

// Scan walks <projectPath>/test-results and returns classified artifacts
// sorted by mtime descending, trimmed to maxItems. Nil if the dir is absent.
func Scan(projectPath string, maxItems int) []File {
	resultsRoot := filepath.Join(projectPath, "test-results")
	info, err := os.Stat(resultsRoot)
	if err != nil || !info.IsDir() {
		return nil
	}

	var found []File
	_ = filepath.WalkDir(resultsRoot, func(path string, d fs.DirEntry, err error) error {
		if d == nil || err != nil {
			return nil // os.walk paridad: entradas ilegibles se saltan
		}
		if d.IsDir() {
			if _, skipped := skipDirs[d.Name()]; skipped {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		kind := classify(ext)
		if kind == "" {
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		rel, rerr := filepath.Rel(projectPath, path)
		if rerr != nil {
			rel = path
		}
		found = append(found, File{
			Path:    path,
			RelPath: filepath.ToSlash(rel),
			Kind:    kind,
			TestDir: filepath.Dir(path),
			MTime:   fi.ModTime().Unix(),
		})
		return nil
	})

	sort.SliceStable(found, func(i, j int) bool { return found[i].MTime > found[j].MTime })
	if maxItems >= 0 && len(found) > maxItems {
		found = found[:maxItems]
	}
	return found
}

// FindHTMLReport returns <projectPath>/playwright-report/index.html when it is
// a regular file, otherwise an empty string.
func FindHTMLReport(projectPath string) string {
	candidate := filepath.Join(projectPath, "playwright-report", "index.html")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}
