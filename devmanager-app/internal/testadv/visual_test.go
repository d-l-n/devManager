package testadv

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func solidPNG(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("codificar PNG: %v", err)
	}
	return buf.Bytes()
}

// pixelPNG dibuja un único píxel distinto sobre un fondo sólido.
func pixelPNG(t *testing.T, w, h int, bg, fg color.RGBA, px, py int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, bg)
		}
	}
	img.Set(px, py, fg)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("codificar PNG: %v", err)
	}
	return buf.Bytes()
}

var (
	red   = color.RGBA{R: 255, A: 255}
	blue  = color.RGBA{B: 255, A: 255}
	white = color.RGBA{R: 255, G: 255, B: 255, A: 255}
)

func TestComparePNGIdenticos(t *testing.T) {
	a := solidPNG(t, 10, 10, white)
	got, err := ComparePNG(a, a, 0)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !got.Same || got.DiffPixels != 0 || got.Percent != 0 {
		t.Errorf("diff = %+v", got)
	}
	if got.Region != nil {
		t.Errorf("region debería ser nil sin diferencias: %+v", got.Region)
	}
	if got.TotalPixels != 100 {
		t.Errorf("total pixels = %d", got.TotalPixels)
	}
}

func TestComparePNGDetectaDiferenciaYTolerancia(t *testing.T) {
	a := pixelPNG(t, 10, 10, white, red, 3, 4)
	b := pixelPNG(t, 10, 10, white, blue, 3, 4)

	strict, err := ComparePNG(a, b, 0)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if strict.Same || strict.DiffPixels != 1 {
		t.Errorf("diff estricto = %+v", strict)
	}
	if strict.Percent != 1 {
		t.Errorf("percent = %v, want 1", strict.Percent)
	}
	if strict.Region == nil || strict.Region.X != 3 || strict.Region.Y != 4 || strict.Region.W != 1 || strict.Region.H != 1 {
		t.Errorf("region = %+v", strict.Region)
	}

	// Rojo vs azul: el delta por canal es 255, así que ninguna tolerancia
	// razonable (<255) lo silencia.
	loose, err := ComparePNG(a, b, 200)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if loose.Same {
		t.Error("tolerancia 200 no debe ocultar un cambio de color completo")
	}

	// Un cambio sutil (1 nivel de canal) se silencia con tolerancia 2.
	plain := solidPNG(t, 10, 10, white)
	near := pixelPNG(t, 10, 10, white, color.RGBA{R: 254, G: 255, B: 255, A: 255}, 1, 1)
	subtle, err := ComparePNG(plain, near, 0)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if subtle.Same || subtle.DiffPixels != 1 {
		t.Errorf("delta de 1 con tolerancia 0 = %+v", subtle)
	}
	got, err := ComparePNG(plain, near, 2)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !got.Same {
		t.Errorf("tolerancia 2 debería absorber un delta de 1: %+v", got)
	}
}

func TestComparePNGSizeMismatch(t *testing.T) {
	a := solidPNG(t, 10, 10, white)
	b := solidPNG(t, 12, 10, white)
	got, err := ComparePNG(a, b, 0)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !got.SizeMismatch || got.Same || got.Percent != 100 {
		t.Errorf("diff = %+v", got)
	}
	if got.Width != 12 || got.Height != 10 {
		t.Errorf("dimensiones = %dx%d", got.Width, got.Height)
	}
}

func TestComparePNGEntradaInvalida(t *testing.T) {
	if _, err := ComparePNG([]byte("no soy un png"), []byte("tampoco"), 0); err == nil {
		t.Fatal("bytes inválidos deberían fallar")
	}
}

func TestWriteDiffPNG(t *testing.T) {
	a := pixelPNG(t, 8, 8, white, red, 2, 2)
	b := pixelPNG(t, 8, 8, white, blue, 2, 2)
	out := filepath.Join(t.TempDir(), "sub", "diff.png")
	if err := WriteDiffPNG(out, a, b, 0); err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("leer diff: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("el diff debe ser un PNG válido: %v", err)
	}
	if img.Bounds().Dx() != 8 {
		t.Errorf("ancho = %d", img.Bounds().Dx())
	}
	r, g, bl, _ := img.At(2, 2).RGBA()
	if r>>8 != 255 || g>>8 != 0 || bl>>8 != 0 {
		t.Errorf("el píxel cambiado debe ser rojo, got %d,%d,%d", r>>8, g>>8, bl>>8)
	}
}

func TestWriteDiffPNGRechazaTamanosDistintos(t *testing.T) {
	a := solidPNG(t, 4, 4, white)
	b := solidPNG(t, 5, 4, white)
	if err := WriteDiffPNG(filepath.Join(t.TempDir(), "x.png"), a, b, 0); err == nil {
		t.Fatal("tamaños distintos deberían fallar")
	}
}

func TestCompareDirs(t *testing.T) {
	root := t.TempDir()
	baseline := filepath.Join(root, "baseline")
	current := filepath.Join(root, "current")
	diff := filepath.Join(root, "diff")
	for _, d := range []string{baseline, current} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(dir, name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// home: igual en ambos
	write(baseline, "home.png", solidPNG(t, 6, 6, white))
	write(current, "home.png", solidPNG(t, 6, 6, white))
	// login: cambia por completo
	write(baseline, "login.png", solidPNG(t, 6, 6, white))
	write(current, "login.png", solidPNG(t, 6, 6, red))
	// solo baseline → missing; solo current → new
	write(baseline, "old.png", solidPNG(t, 6, 6, white))
	write(current, "nuevo.png", solidPNG(t, 6, 6, red))
	// tipográfico: 1 píxel sobre 36 → 2.78% (por debajo de maxPercent)
	write(baseline, "tabs.png", pixelPNG(t, 6, 6, white, red, 0, 0))
	write(current, "tabs.png", pixelPNG(t, 6, 6, white, blue, 0, 0))

	report, err := CompareDirs(baseline, current, diff, 0, 5)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if !report.Available {
		t.Fatal("report debería estar disponible con PNGs presentes")
	}
	if report.Total != 5 {
		t.Fatalf("total = %d, want 5 (%+v)", report.Total, report.Results)
	}
	if report.Same != 2 || report.Changed != 1 || report.New != 1 || report.Missing != 1 {
		t.Errorf("conteos: same=%d changed=%d new=%d missing=%d",
			report.Same, report.Changed, report.New, report.Missing)
	}
	byName := map[string]VisualResult{}
	for _, r := range report.Results {
		byName[r.Name] = r
	}
	if byName["tabs.png"].Status != "same" {
		t.Errorf("tabs.png debería quedar bajo el umbral: %+v", byName["tabs.png"])
	}
	if byName["login.png"].Status != "changed" || byName["login.png"].DiffPath == "" {
		t.Errorf("login.png = %+v", byName["login.png"])
	}
	if _, err := os.Stat(byName["login.png"].DiffPath); err != nil {
		t.Errorf("el PNG de diff debería existir: %v", err)
	}
	if byName["nuevo.png"].Status != "new" || byName["old.png"].Status != "missing" {
		t.Errorf("new/missing mal clasificados: %+v %+v", byName["nuevo.png"], byName["old.png"])
	}
}

func TestCompareDirsVacio(t *testing.T) {
	report, err := CompareDirs(filepath.Join(t.TempDir(), "nope"), filepath.Join(t.TempDir(), "nope2"), "", 0, 0)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if report.Available || report.Total != 0 {
		t.Errorf("report vacío esperado: %+v", report)
	}
}

func TestPromoteBaseline(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "current")
	baseline := filepath.Join(root, "baseline")
	if err := os.MkdirAll(filepath.Join(current, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "a.png"), solidPNG(t, 4, 4, white), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "sub", "b.png"), solidPNG(t, 4, 4, red), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := PromoteBaseline(current, baseline)
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if n != 2 {
		t.Errorf("copiados = %d, want 2", n)
	}
	if _, err := os.Stat(filepath.Join(baseline, "sub", "b.png")); err != nil {
		t.Errorf("debería preservar subdirectorios: %v", err)
	}
	if _, err := PromoteBaseline(filepath.Join(root, "vacio"), baseline); err == nil {
		t.Error("promover sin screenshots debería fallar")
	}
}
