package testadv

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BBox es la región que contiene diferencias (coordenadas en píxeles).
type BBox struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// VisualDiff es el resultado de comparar dos PNG del mismo tamaño.
type VisualDiff struct {
	Width        int     `json:"width"`
	Height       int     `json:"height"`
	DiffPixels   int     `json:"diffPixels"`
	TotalPixels  int     `json:"totalPixels"`
	Percent      float64 `json:"percent"`
	Region       *BBox   `json:"region,omitempty"`
	Same         bool    `json:"same"`
	SizeMismatch bool    `json:"sizeMismatch"`
}

// VisualResult es una fila del report de regresión visual.
type VisualResult struct {
	Name     string     `json:"name"`
	Status   string     `json:"status"` // same | changed | new | missing
	Baseline string     `json:"baseline,omitempty"`
	Current  string     `json:"current,omitempty"`
	DiffPath string     `json:"diffPath,omitempty"`
	Diff     VisualDiff `json:"diff"`
}

// VisualReport resume el comparativo baseline vs current.
type VisualReport struct {
	BaselineDir string         `json:"baselineDir"`
	CurrentDir  string         `json:"currentDir"`
	DiffDir     string         `json:"diffDir"`
	Tolerance   int            `json:"tolerance"`
	MaxPercent  float64        `json:"maxPercent"`
	Total       int            `json:"total"`
	Same        int            `json:"same"`
	Changed     int            `json:"changed"`
	New         int            `json:"new"`
	Missing     int            `json:"missing"`
	Available   bool           `json:"available"`
	Results     []VisualResult `json:"results"`
}

// ComparePNG compara dos PNG byte a byte con tolerancia de canal (0..255).
// Tamaños distintos se reportan como SizeMismatch con Percent 100 (nunca se
// silencia: un layout roto debe ser un fallo visible).
func ComparePNG(a, b []byte, tolerance uint8) (VisualDiff, error) {
	imgA, err := decodePNG(a)
	if err != nil {
		return VisualDiff{}, fmt.Errorf("PNG baseline: %w", err)
	}
	imgB, err := decodePNG(b)
	if err != nil {
		return VisualDiff{}, fmt.Errorf("PNG current: %w", err)
	}
	ba, bb := imgA.Bounds(), imgB.Bounds()
	if ba.Dx() != bb.Dx() || ba.Dy() != bb.Dy() {
		w, h := bb.Dx(), bb.Dy()
		return VisualDiff{
			Width: w, Height: h,
			DiffPixels:   w * h,
			TotalPixels:  w * h,
			Percent:      100,
			Same:         false,
			SizeMismatch: true,
			Region:       &BBox{X: 0, Y: 0, W: w, H: h},
		}, nil
	}

	out := VisualDiff{Width: ba.Dx(), Height: ba.Dy(), TotalPixels: ba.Dx() * ba.Dy()}
	minX, minY := ba.Dx(), ba.Dy()
	maxX, maxY := -1, -1
	for y := ba.Min.Y; y < ba.Max.Y; y++ {
		for x := ba.Min.X; x < ba.Max.X; x++ {
			if channelDelta(imgA.At(x, y), imgB.At(x, y)) > tolerance {
				out.DiffPixels++
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if out.TotalPixels > 0 {
		out.Percent = round2(float64(out.DiffPixels) / float64(out.TotalPixels) * 100)
	}
	out.Same = out.DiffPixels == 0
	if maxX >= 0 {
		out.Region = &BBox{X: minX, Y: minY, W: maxX - minX + 1, H: maxY - minY + 1}
	}
	return out, nil
}

// ComparePNGFiles lee dos ficheros y los compara. Los paths se resuelven
// relativos a projectPath cuando no son absolutos.
func ComparePNGFiles(projectPath, baseline, current string, tolerance uint8) (VisualDiff, error) {
	a, err := os.ReadFile(resolve(projectPath, baseline))
	if err != nil {
		return VisualDiff{}, fmt.Errorf("leer baseline: %w", err)
	}
	b, err := os.ReadFile(resolve(projectPath, current))
	if err != nil {
		return VisualDiff{}, fmt.Errorf("leer current: %w", err)
	}
	return ComparePNG(a, b, tolerance)
}

// WriteDiffPNG genera una imagen de diagnóstico: gris atenuado para los
// píxeles iguales y rojo para los distintos. Requiere mismo tamaño.
func WriteDiffPNG(outPath string, a, b []byte, tolerance uint8) error {
	imgA, err := decodePNG(a)
	if err != nil {
		return fmt.Errorf("PNG baseline: %w", err)
	}
	imgB, err := decodePNG(b)
	if err != nil {
		return fmt.Errorf("PNG current: %w", err)
	}
	ba, bb := imgA.Bounds(), imgB.Bounds()
	if ba.Dx() != bb.Dx() || ba.Dy() != bb.Dy() {
		return fmt.Errorf("no se puede generar diff: tamaños distintos (%dx%d vs %dx%d)",
			ba.Dx(), ba.Dy(), bb.Dx(), bb.Dy())
	}

	out := image.NewRGBA(image.Rect(0, 0, ba.Dx(), ba.Dy()))
	changed := color.RGBA{R: 255, G: 0, B: 0, A: 255}
	for y := ba.Min.Y; y < ba.Max.Y; y++ {
		for x := ba.Min.X; x < ba.Max.X; x++ {
			p := imgA.At(x, y)
			if channelDelta(p, imgB.At(x, y)) > tolerance {
				out.Set(x-ba.Min.X, y-ba.Min.Y, changed)
				continue
			}
			r, g, bl, _ := p.RGBA()
			gray := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 256 / 4)
			out.Set(x-ba.Min.X, y-ba.Min.Y, color.RGBA{R: gray, G: gray, B: gray, A: 255})
		}
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return fmt.Errorf("crear dir de diffs: %w", err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return fmt.Errorf("codificar diff: %w", err)
	}
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// CompareDirs compara baselineDir contra currentDir (recursivo, solo *.png).
// Un par se marca "changed" cuando su diferencia supera maxPercent; el PNG de
// diagnóstico se escribe en diffDir (nunca se sobrescribe el current).
func CompareDirs(baselineDir, currentDir, diffDir string, tolerance uint8, maxPercent float64) (VisualReport, error) {
	report := VisualReport{
		BaselineDir: baselineDir,
		CurrentDir:  currentDir,
		DiffDir:     diffDir,
		Tolerance:   int(tolerance),
		MaxPercent:  maxPercent,
		Results:     []VisualResult{},
	}
	baseline, err := listPNGs(baselineDir)
	if err != nil {
		return report, fmt.Errorf("listar baseline: %w", err)
	}
	current, err := listPNGs(currentDir)
	if err != nil {
		return report, fmt.Errorf("listar current: %w", err)
	}
	if len(baseline) == 0 && len(current) == 0 {
		return report, nil
	}
	report.Available = true

	names := map[string]bool{}
	for n := range baseline {
		names[n] = true
	}
	for n := range current {
		names[n] = true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)

	for _, name := range sorted {
		basePath, hasBase := baseline[name]
		curPath, hasCur := current[name]
		res := VisualResult{Name: name, Baseline: basePath, Current: curPath}
		switch {
		case hasBase && !hasCur:
			res.Status = "missing"
		case hasCur && !hasBase:
			res.Status = "new"
		default:
			a, err := os.ReadFile(basePath)
			if err != nil {
				res.Status = "changed"
				report.Results = append(report.Results, res)
				continue
			}
			b, err := os.ReadFile(curPath)
			if err != nil {
				res.Status = "changed"
				report.Results = append(report.Results, res)
				continue
			}
			diff, err := ComparePNG(a, b, tolerance)
			if err != nil {
				res.Status = "changed"
				report.Results = append(report.Results, res)
				continue
			}
			res.Diff = diff
			if diff.Percent > maxPercent {
				res.Status = "changed"
				if !diff.SizeMismatch && diffDir != "" {
					out := filepath.Join(diffDir, filepath.FromSlash(name))
					if err := WriteDiffPNG(out, a, b, tolerance); err == nil {
						res.DiffPath = out
					}
				}
			} else {
				res.Status = "same"
			}
		}
		switch res.Status {
		case "same":
			report.Same++
		case "changed":
			report.Changed++
		case "new":
			report.New++
		case "missing":
			report.Missing++
		}
		report.Results = append(report.Results, res)
	}
	report.Total = len(report.Results)
	return report, nil
}

// PromoteBaseline copia todos los PNG de currentDir a baselineDir (aceptar la
// nueva referencia). Devuelve cuántos ficheros copió.
func PromoteBaseline(currentDir, baselineDir string) (int, error) {
	current, err := listPNGs(currentDir)
	if err != nil {
		return 0, fmt.Errorf("listar current: %w", err)
	}
	if len(current) == 0 {
		return 0, fmt.Errorf("no hay screenshots en %s", currentDir)
	}
	copied := 0
	for name, src := range current {
		data, err := os.ReadFile(src)
		if err != nil {
			return copied, fmt.Errorf("leer %s: %w", name, err)
		}
		dst := filepath.Join(baselineDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return copied, fmt.Errorf("crear dir baseline: %w", err)
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return copied, fmt.Errorf("escribir baseline %s: %w", name, err)
		}
		copied++
	}
	return copied, nil
}

// ---- helpers ----

func decodePNG(data []byte) (image.Image, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("PNG inválido: %w", err)
	}
	return img, nil
}

// channelDelta devuelve la mayor diferencia absoluta por canal RGBA.
func channelDelta(a, b color.Color) uint8 {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	delta := func(x, y uint32) uint32 {
		if x > y {
			return x - y
		}
		return y - x
	}
	max := delta(ar, br)
	for _, d := range []uint32{delta(ag, bg), delta(ab, bb), delta(aa, ba)} {
		if d > max {
			max = d
		}
	}
	return uint8(max >> 8)
}

func resolve(projectPath, p string) string {
	if p == "" || filepath.IsAbs(p) || strings.TrimSpace(projectPath) == "" {
		return p
	}
	return filepath.Join(projectPath, p)
}

// listPNGs indexa *.png (recursivo) por path relativo con separador '/'.
func listPNGs(dir string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(dir) == "" {
		return out, nil
	}
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // directorio ilegible: se ignora el subárbol
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".png") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		out[filepath.ToSlash(rel)] = path
		return nil
	})
	return out, err
}
