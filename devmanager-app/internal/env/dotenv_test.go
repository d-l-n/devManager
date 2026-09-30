package env

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	vars, err := Parse([]byte("FOO=bar\nBAZ=qux\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if vars["FOO"] != "bar" || vars["BAZ"] != "qux" {
		t.Errorf("vars = %v", vars)
	}
}

func TestParseQuotes(t *testing.T) {
	vars, err := Parse([]byte("A=\"hello world\"\nB='single # kept'\nC=\"hash # kept\"\nD=\"esc \\\"q\\\"\"\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if vars["A"] != "hello world" {
		t.Errorf("A = %q", vars["A"])
	}
	if vars["B"] != "single # kept" {
		t.Errorf("B = %q", vars["B"])
	}
	if vars["C"] != "hash # kept" {
		t.Errorf("C = %q", vars["C"])
	}
	if vars["D"] != `esc "q"` {
		t.Errorf("D = %q", vars["D"])
	}
}

func TestParseCommentsExportEmpty(t *testing.T) {
	in := "# full line comment\n\nexport FOO=bar\nEMPTY=\nSPACED = spaced value\nTRAIL=val # comment\nHASH=a#b\n"
	vars, err := Parse([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if vars["FOO"] != "bar" {
		t.Errorf("export FOO = %q", vars["FOO"])
	}
	if vars["EMPTY"] != "" {
		t.Errorf("EMPTY = %q", vars["EMPTY"])
	}
	if vars["SPACED"] != "spaced value" {
		t.Errorf("SPACED = %q", vars["SPACED"])
	}
	if vars["TRAIL"] != "val" {
		t.Errorf("TRAIL = %q", vars["TRAIL"])
	}
	if vars["HASH"] != "a#b" {
		t.Errorf("HASH = %q, want raw # kept", vars["HASH"])
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []string{
		"no equals here",
		"lower=bad",
		"9BAD=x",
		"BAD-KEY=x",
		"A=\"unterminated",
		"B='unterminated",
		"C=\"ok\" trailing",
		"export",
	}
	for _, in := range cases {
		if _, err := Parse([]byte(in + "\n")); err == nil {
			t.Errorf("esperaba error para %q", in)
		}
	}
}

func TestParseErrorOmitsValues(t *testing.T) {
	_, err := Parse([]byte("lower=supersecret\n"))
	if err == nil {
		t.Fatal("esperaba error")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Errorf("el error no debe incluir valores: %q", err.Error())
	}
}

func TestValidateKey(t *testing.T) {
	for _, ok := range []string{"A", "_X", "FOO_BAR2", "VITE_PORT"} {
		if err := ValidateKey(ok); err != nil {
			t.Errorf("key %q válida rechazada: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "a", "9A", "FOO-BAR", "FOO BAR", "port"} {
		if err := ValidateKey(bad); err == nil {
			t.Errorf("key %q inválida aceptada", bad)
		}
	}
}

func TestSerializeSortedAndQuoted(t *testing.T) {
	out := Serialize(map[string]string{
		"B":     "plain",
		"A":     "with space",
		"HASH":  "a#b",
		"EMPTY": "",
		"Q":     `say "hi"`,
	})
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("líneas = %v", lines)
	}
	for i, want := range []string{"A=", "B=", "EMPTY=", "HASH=", "Q="} {
		if !strings.HasPrefix(lines[i], want) {
			t.Errorf("línea %d = %q, want prefijo %q", i, lines[i], want)
		}
	}
	if lines[0] != `A="with space"` {
		t.Errorf("A = %q", lines[0])
	}
	if lines[2] != `EMPTY=""` {
		t.Errorf("EMPTY = %q", lines[2])
	}
	if lines[4] != `Q="say \"hi\""` {
		t.Errorf("Q = %q", lines[4])
	}
}

func TestSerializeParseRoundTrip(t *testing.T) {
	in := map[string]string{
		"PLAIN":  "abc",
		"SPACED": "hello world",
		"HASH":   "a#b # c",
		"QUOTED": `say "hi"`,
		"EMPTY":  "",
		"NL":     "a\nb",
	}
	vars, err := Parse([]byte(Serialize(in)))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	for k, want := range in {
		if vars[k] != want {
			t.Errorf("%s = %q, want %q", k, vars[k], want)
		}
	}
}

func TestMerge(t *testing.T) {
	base := map[string]string{"A": "1", "B": "2"}
	over := map[string]string{"B": "3", "C": "4"}
	got := Merge(base, over)
	if got["A"] != "1" || got["B"] != "3" || got["C"] != "4" {
		t.Errorf("merge = %v", got)
	}
	if base["B"] != "2" {
		t.Error("merge mutó base")
	}
	if Merge(nil, nil) == nil {
		t.Error("merge nil debe devolver mapa no-nil")
	}
}
