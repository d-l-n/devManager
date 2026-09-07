package dashboard

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sample(ts time.Time, running bool, uptime int64) Sample {
	return Sample{Ts: ts.Unix(), Running: running, Uptime: uptime}
}

func TestAppendAndSince(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard-history.json")
	store := NewStore(path)

	now := time.Now()
	if err := store.Append("alpha", sample(now, true, 60)); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if err := store.Append("beta", sample(now.Add(-2*time.Hour), false, 0)); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Ventana 1h: beta (hace 2h) queda fuera.
	hist := store.Since(time.Hour)
	if len(hist) != 1 || hist[0].Name != "alpha" {
		t.Fatalf("Since(1h) = %+v, want solo alpha", hist)
	}
	if len(hist[0].Samples) != 1 || !hist[0].Samples[0].Running {
		t.Errorf("samples de alpha incorrectos: %+v", hist[0].Samples)
	}

	// Ventana 24h: ambos.
	if got := len(store.Since(24 * time.Hour)); got != 2 {
		t.Errorf("Since(24h) = %d proyectos, want 2", got)
	}
}

func TestRetentionPrunes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard-history.json")
	store := NewStore(path)

	now := time.Now()
	// 3 samples: hace 25h (fuera), hace 2h, ahora.
	_ = store.Append("alpha", sample(now.Add(-25*time.Hour), false, 0))
	_ = store.Append("alpha", sample(now.Add(-2*time.Hour), true, 60))
	_ = store.Append("alpha", sample(now, true, 120))

	hist := store.Since(24 * time.Hour)
	if len(hist) != 1 || len(hist[0].Samples) != 2 {
		t.Fatalf("retención no aplicada: %+v", hist)
	}
	// El sample de hace 25h no debe revivir en ventanas mayores.
	if got := len(store.Since(48 * time.Hour)); got != 1 {
		t.Errorf("retención dura 24h: Since(48h) = %d proyectos", got)
	}
}

func TestPruneAllDropsEmptySeries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dashboard-history.json")
	store := NewStore(path)

	now := time.Now()
	_ = store.Append("viejo", sample(now.Add(-48*time.Hour), true, 60))
	_ = store.Append("vivo", sample(now, true, 60))

	hist := store.Since(24 * time.Hour)
	if len(hist) != 1 || hist[0].Name != "vivo" {
		t.Fatalf("serie muerta debería desaparecer: %+v", hist)
	}
}

func TestPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dashboard-history.json")
	store := NewStore(path)

	now := time.Now()
	want := sample(now, true, 3600)
	if err := store.Append("alpha", want); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Nueva instancia sobre el mismo archivo.
	reloaded := NewStore(path)
	hist := reloaded.Since(24 * time.Hour)
	if len(hist) != 1 || len(hist[0].Samples) != 1 {
		t.Fatalf("round-trip: %+v", hist)
	}
	got := hist[0].Samples[0]
	if got.Ts != want.Ts || got.Running != want.Running || got.Uptime != want.Uptime {
		t.Errorf("sample persistido ≠ original:\ngot  %+v\nwant %+v", got, want)
	}
}

func TestLoadCorruptFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dashboard-history.json")
	if err := os.WriteFile(path, []byte("{corrupto"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	if got := store.Since(time.Hour); len(got) != 0 {
		t.Errorf("archivo corrupto debe dar historial vacío, got %+v", got)
	}
	// Y debe poder seguir escribiendo sin problema.
	if err := store.Append("alpha", sample(time.Now(), true, 1)); err != nil {
		t.Errorf("Append tras corrupto: %v", err)
	}
}
