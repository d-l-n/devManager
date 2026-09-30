package env

import "sort"

// Estados de DiffRow: added (solo en B), removed (solo en A),
// changed (distinto valor), same (igual).
const (
	DiffAdded   = "added"
	DiffRemoved = "removed"
	DiffChanged = "changed"
	DiffSame    = "same"
)

// DiffRow compara una key entre dos mapas de vars.
type DiffRow struct {
	Key    string `json:"key"`
	A      string `json:"a"`
	B      string `json:"b"`
	Status string `json:"status"`
}

// Diff compara a vs b, ordenado por key. Los valores deben venir ya
// enmascarados si corresponden a secretos (nunca loguear valores).
func Diff(a, b map[string]string) []DiffRow {
	keys := map[string]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	rows := make([]DiffRow, 0, len(keys))
	for k := range keys {
		va, oka := a[k]
		vb, okb := b[k]
		row := DiffRow{Key: k, A: va, B: vb}
		switch {
		case !oka:
			row.Status = DiffAdded
		case !okb:
			row.Status = DiffRemoved
		case va != vb:
			row.Status = DiffChanged
		default:
			row.Status = DiffSame
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
	return rows
}
