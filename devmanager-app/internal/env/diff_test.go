package env

import "testing"

func TestDiffStatuses(t *testing.T) {
	a := map[string]string{"SAME": "x", "CHG": "1", "DEL": "gone"}
	b := map[string]string{"SAME": "x", "CHG": "2", "ADD": "new"}
	rows := Diff(a, b)
	if len(rows) != 4 {
		t.Fatalf("rows = %v", rows)
	}
	// Ordenado por key: ADD, CHG, DEL, SAME.
	want := []DiffRow{
		{Key: "ADD", A: "", B: "new", Status: DiffAdded},
		{Key: "CHG", A: "1", B: "2", Status: DiffChanged},
		{Key: "DEL", A: "gone", B: "", Status: DiffRemoved},
		{Key: "SAME", A: "x", B: "x", Status: DiffSame},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], w)
		}
	}
}

func TestDiffEmpty(t *testing.T) {
	if rows := Diff(nil, nil); len(rows) != 0 {
		t.Errorf("diff vacío = %v", rows)
	}
	if rows := Diff(map[string]string{}, map[string]string{"A": "1"}); len(rows) != 1 || rows[0].Status != DiffAdded {
		t.Errorf("diff = %v", rows)
	}
}

func TestDiffMaskedSecretsStaySame(t *testing.T) {
	// Secretos con distinto valor real llegan ya enmascarados desde el
	// binding: diff "same", sin exponer valores.
	a := map[string]string{"TOKEN": "***"}
	b := map[string]string{"TOKEN": "***"}
	rows := Diff(a, b)
	if len(rows) != 1 || rows[0].Status != DiffSame {
		t.Errorf("diff enmascarado = %v", rows)
	}
	if rows[0].A != "***" || rows[0].B != "***" {
		t.Errorf("valores enmascarados = %+v", rows[0])
	}
}
