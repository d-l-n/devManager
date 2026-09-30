package testadv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "testadv.json")
	in := Store{Schedules: []Schedule{
		{ID: "a", Name: "nightly", Spec: "daily 03:00", Command: "npx playwright test", Enabled: true},
	}}
	if err := SaveStore(path, in); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	got := LoadStore(path)
	if len(got.Schedules) != 1 || got.Schedules[0].Name != "nightly" {
		t.Fatalf("store = %+v", got)
	}
	if got.Schedules[0].Command != "npx playwright test" {
		t.Errorf("command perdido: %+v", got.Schedules[0])
	}
}

func TestLoadStoreTolerante(t *testing.T) {
	dir := t.TempDir()
	missing := LoadStore(filepath.Join(dir, "no-existe.json"))
	if missing.Schedules == nil || len(missing.Schedules) != 0 {
		t.Errorf("store ausente = %+v", missing)
	}
	corrupt := filepath.Join(dir, "roto.json")
	if err := os.WriteFile(corrupt, []byte("{no json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := LoadStore(corrupt)
	if got.Schedules == nil || len(got.Schedules) != 0 {
		t.Errorf("store corrupto debe degradar a vacío: %+v", got)
	}
}

func TestAppendRunRecorta(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	base := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		rec := RunRecord{StartedAt: base.Add(time.Duration(i) * time.Minute), Passed: i}
		if err := AppendRun(path, rec, 3); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	runs, err := LoadRuns(path, 0)
	if err != nil {
		t.Fatalf("leer: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3 (keep)", len(runs))
	}
	if runs[0].Passed != 2 || runs[2].Passed != 4 {
		t.Errorf("se deben conservar los más recientes: %+v", runs)
	}
	data, _ := os.ReadFile(path)
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("el JSONL debe terminar en salto de línea")
	}
}

func TestLoadRunsLimitYLineasCorruptas(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	content := `{"startedAt":"2026-09-30T10:00:00Z","passed":1}
basura
{"startedAt":"2026-09-30T11:00:00Z","passed":2}
{"startedAt":"2026-09-30T12:00:00Z","passed":3}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	runs, err := LoadRuns(path, 0)
	if err != nil {
		t.Fatalf("leer: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3 (la línea corrupta se ignora)", len(runs))
	}
	last, err := LoadRuns(path, 2)
	if err != nil {
		t.Fatalf("leer con límite: %v", err)
	}
	if len(last) != 2 || last[0].Passed != 2 || last[1].Passed != 3 {
		t.Errorf("últimos runs = %+v", last)
	}

	empty, err := LoadRuns(filepath.Join(t.TempDir(), "nope.jsonl"), 0)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Errorf("fichero ausente debe dar slice vacío sin error: %v %+v", err, empty)
	}
}

func TestWriteAtomicNoDejaTmp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.json")
	if err := WriteAtomic(path, []byte("{}"), 0o644); err != nil {
		t.Fatalf("escribir: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "x.json" {
		t.Errorf("no debe quedar fichero temporal: %+v", entries)
	}
}

func TestSchedulesRecomputeAntesDeGuardar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testadv.json")
	st := Store{Schedules: []Schedule{{ID: "a", Name: "n", Spec: "every 30m", Enabled: true}}}
	now := time.Date(2026, 9, 30, 10, 0, 0, 0, time.UTC)
	st.Schedules[0].Recompute(now)
	if err := SaveStore(path, st); err != nil {
		t.Fatalf("guardar: %v", err)
	}
	got := LoadStore(path)
	if !got.Schedules[0].NextRun.Equal(now.Add(30 * time.Minute)) {
		t.Errorf("nextRun persistido = %v", got.Schedules[0].NextRun)
	}
}
