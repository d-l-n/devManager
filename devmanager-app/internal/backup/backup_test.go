package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var testNow = time.Date(2026, 9, 7, 15, 30, 0, 0, time.UTC)

func writeProjectFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "projects.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeSettingsFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCreateBackupAndValidate(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[{"name":"demo"}]}`)
	settings := writeSettingsFile(t, dir, `{"theme":"dark"}`)
	backupDir := filepath.Join(dir, "backups")

	name, err := CreateBackup(backupDir, projects, settings, "v2.1.0", testNow)
	if err != nil {
		t.Fatalf("CreateBackup: %v", err)
	}
	if name != "backup-20260907-153000.dmbak" {
		t.Errorf("nombre inesperado: %q", name)
	}

	entry, err := ValidateFile(filepath.Join(backupDir, name))
	if err != nil {
		t.Fatalf("ValidateFile: %v", err)
	}
	if !entry.Valid {
		t.Errorf("backup debería ser válido: %+v", entry)
	}
	if entry.ProjectsCount != 1 {
		t.Errorf("ProjectsCount = %d, want 1", entry.ProjectsCount)
	}
	if !entry.SettingsIncluded {
		t.Error("SettingsIncluded = false, want true")
	}
	if entry.AppVersion != "v2.1.0" {
		t.Errorf("AppVersion = %q", entry.AppVersion)
	}
	if entry.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d", entry.SizeBytes)
	}
}

func TestCreateBackupWithoutSettings(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[]}`)
	backupDir := filepath.Join(dir, "backups")

	name, err := CreateBackup(backupDir, projects, filepath.Join(dir, "missing.json"), "dev", testNow)
	if err != nil {
		t.Fatalf("CreateBackup sin settings: %v", err)
	}
	entry, _ := ValidateFile(filepath.Join(backupDir, name))
	if !entry.Valid || entry.SettingsIncluded {
		t.Errorf("entry inesperado: %+v", entry)
	}
}

func TestCreateBackupMissingProjects(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	_, err := CreateBackup(backupDir, filepath.Join(dir, "nope.json"), "", "dev", testNow)
	if err == nil {
		t.Fatal("debería fallar sin projects.json")
	}
}

func TestUniqueNameCollision(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[]}`)
	backupDir := filepath.Join(dir, "backups")

	n1, err := CreateBackup(backupDir, projects, "", "dev", testNow)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := CreateBackup(backupDir, projects, "", "dev", testNow)
	if err != nil {
		t.Fatal(err)
	}
	if n1 == n2 {
		t.Fatalf("dos backups en el mismo segundo deben tener nombres distintos: %q", n1)
	}
	if n2 != "backup-20260907-153000-1.dmbak" {
		t.Errorf("sufijo -1 esperado, got %q", n2)
	}
}

func TestValidateCorruptArchive(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(backupDir, "backup-20260907-153000.dmbak")
	if err := os.WriteFile(corrupt, []byte("esto no es gzip"), 0o644); err != nil {
		t.Fatal(err)
	}
	entry, err := ValidateFile(corrupt)
	if err == nil {
		t.Fatal("debería fallar con gzip inválido")
	}
	if entry.Valid {
		t.Error("entry.Valid debe ser false para archivo corrupto")
	}
	if entry.Error == "" {
		t.Error("entry.Error debe explicar la corrupción")
	}
}

func TestValidateTamperedContent(t *testing.T) {
	// Backup válido al que se le corrompe un byte dentro del gzip: el CRC32
	// de gzip o el sha256 del manifiesto deben detectarlo.
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[{"name":"a"}]}`)
	backupDir := filepath.Join(dir, "backups")
	name, err := CreateBackup(backupDir, projects, "", "dev", testNow)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(backupDir, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Flip un byte del medio (dentro del stream deflate).
	raw[len(raw)/2] ^= 0xFF
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateFile(path); err == nil {
		t.Fatal("backup manipulado debe fallar validación")
	}
}

func TestListSortedNewestFirst(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[]}`)
	backupDir := filepath.Join(dir, "backups")

	t1 := testNow
	t2 := testNow.Add(2 * time.Hour)
	t3 := testNow.Add(4 * time.Hour)
	for _, ts := range []time.Time{t2, t3, t1} { // crear desordenado
		if _, err := CreateBackup(backupDir, projects, "", "dev", ts); err != nil {
			t.Fatal(err)
		}
	}
	list := List(backupDir)
	if len(list) != 3 {
		t.Fatalf("List = %d entries, want 3", len(list))
	}
	if list[0].CreatedAt <= list[1].CreatedAt || list[1].CreatedAt <= list[2].CreatedAt {
		t.Errorf("orden esperado newest→oldest: %+v", list)
	}
}

func TestApplyRetention(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[]}`)
	backupDir := filepath.Join(dir, "backups")

	for i := 0; i < 5; i++ {
		if _, err := CreateBackup(backupDir, projects, "", "dev", testNow.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := ApplyRetention(backupDir, 3)
	if err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
	if removed != 2 {
		t.Errorf("removed = %d, want 2", removed)
	}
	if got := len(List(backupDir)); got != 3 {
		t.Errorf("quedan %d backups, want 3", got)
	}

	// keep=0 es no-op defensivo (nunca borrar todo por un config corrupto).
	removed0, err := ApplyRetention(backupDir, 0)
	if err != nil || removed0 != 0 {
		t.Errorf("keep=0 debe ser no-op: removed=%d err=%v", removed0, err)
	}
}

func TestApplyRetentionDeletesInvalidFirst(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[]}`)
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 2 válidos + 1 corrupto (con nombre válido para que List lo incluya).
	for i := 0; i < 2; i++ {
		if _, err := CreateBackup(backupDir, projects, "", "dev", testNow.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	corrupt := filepath.Join(backupDir, "backup-20260907-235900.dmbak")
	if err := os.WriteFile(corrupt, []byte("garbage"), 0o644); err != nil {
		t.Fatal(err)
	}
	// mtime explícito: en los inválidos ValidateFile deriva CreatedAt del
	// ModTime (no hay manifiesto que leer), así que sin esto el orden del
	// catálogo depende del reloj real y el test pasaba o fallaba según la
	// fecha. Más nuevo que los válidos = el caso que rompía: el corrupto entra
	// primero y corría el cupo de keep una posición.
	corrTime := testNow.Add(2 * time.Hour)
	if err := os.Chtimes(corrupt, corrTime, corrTime); err != nil {
		t.Fatal(err)
	}

	removed, err := ApplyRetention(backupDir, 2)
	if err != nil {
		t.Fatalf("ApplyRetention: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed = %d, want 1 (solo el corrupto)", removed)
	}
	if _, err := os.Stat(corrupt); !os.IsNotExist(err) {
		t.Error("el corrupto debería haberse borrado")
	}
	if got := len(List(backupDir)); got != 2 {
		t.Errorf("quedan %d, want 2 válidos", got)
	}
}

func TestRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[{"name":"original"}]}`)
	settings := writeSettingsFile(t, dir, `{"theme":"oled"}`)
	backupDir := filepath.Join(dir, "backups")

	name, err := CreateBackup(backupDir, projects, settings, "v2.1.0", testNow)
	if err != nil {
		t.Fatal(err)
	}

	// Mutar el estado actual post-backup.
	if err := os.WriteFile(projects, []byte(`{"projects":[{"name":"mutado"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"theme":"light"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	restored, err := Restore(backupDir, name, projects, settings)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(restored) != 2 {
		t.Errorf("restored = %v, want 2 archivos", restored)
	}
	gotP, _ := os.ReadFile(projects)
	if string(gotP) != `{"projects":[{"name":"original"}]}` {
		t.Errorf("projects.json no restaurado byte-exacto: %q", gotP)
	}
	gotS, _ := os.ReadFile(settings)
	if string(gotS) != `{"theme":"oled"}` {
		t.Errorf("settings.json no restaurado byte-exacto: %q", gotS)
	}

	// .pre-restore con el estado mutado pre-restore.
	preP, err := os.ReadFile(projects + preRestoreExt)
	if err != nil {
		t.Fatalf(".pre-restore de projects.json no creado: %v", err)
	}
	if string(preP) != `{"projects":[{"name":"mutado"}]}` {
		t.Errorf(".pre-restore contenido inesperado: %q", preP)
	}
}

func TestRestoreRejectsBadFilename(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	for _, name := range []string{"../../etc/passwd", "pepe.dmbak", "", "backup-1.dmbak"} {
		if _, err := Restore(backupDir, name, filepath.Join(dir, "p.json"), ""); err == nil {
			t.Errorf("Restore con %q debería rechazarse", name)
		}
	}
}

func TestRestoreNonexistent(t *testing.T) {
	dir := t.TempDir()
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Restore(backupDir, "backup-20260907-153000.dmbak", filepath.Join(dir, "p.json"), "")
	if err == nil || !strings.Contains(err.Error(), "abrir backup") {
		t.Errorf("error inesperado: %v", err)
	}
}

func TestRestoreWithoutSettingsTarget(t *testing.T) {
	dir := t.TempDir()
	projects := writeProjectFile(t, dir, `{"projects":[]}`)
	settings := writeSettingsFile(t, dir, `{"theme":"dark"}`)
	backupDir := filepath.Join(dir, "backups")
	name, err := CreateBackup(backupDir, projects, settings, "dev", testNow)
	if err != nil {
		t.Fatal(err)
	}
	// settingsPath vacío: solo projects se restaura.
	restored, err := Restore(backupDir, name, projects, "")
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(restored) != 1 || restored[0] != NameProjects {
		t.Errorf("restored = %v, want [projects.json]", restored)
	}
}

func TestValidFilename(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"backup-20260907-153000.dmbak", true},
		{"backup-20260907-153000-1.dmbak", true},
		{"backup-20260907-153000.dmbak.exe", false},
		{"../../evil.dmbak", false},
		{"random.json", false},
		{"", false},
	}
	for _, c := range cases {
		if got := ValidFilename(c.name); got != c.want {
			t.Errorf("ValidFilename(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
