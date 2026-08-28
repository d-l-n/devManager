package logger

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestRingAppendAndHistory(t *testing.T) {
	r := New(5)
	r.Append("line1", false)
	r.Append("line2", true)
	r.Append("line3", false)
	h := r.History()
	if len(h) != 3 {
		t.Fatalf("len = %d, want 3 (%+v)", len(h), h)
	}
	if h[0].Text != "line1" || h[0].IsError {
		t.Errorf("h[0] = %+v", h[0])
	}
	if !h[1].IsError {
		t.Errorf("h[1] should be error: %+v", h[1])
	}
	if h[1].TS == "" {
		t.Error("timestamp missing")
	}
}

func TestRingCaps(t *testing.T) {
	r := New(3)
	for i := 0; i < 10; i++ {
		r.Append(string(rune('a'+i)), false)
	}
	h := r.History()
	if len(h) != 3 {
		t.Fatalf("len = %d, want 3", len(h))
	}
	if h[0].Text != "h" || h[1].Text != "i" || h[2].Text != "j" {
		t.Errorf("ring should keep last 3: %+v", h)
	}
	if r.Len() != 3 {
		t.Errorf("Len = %d, want 3", r.Len())
	}
}

func TestRingClear(t *testing.T) {
	r := New(5)
	r.Append("x", false)
	r.Clear()
	if r.Len() != 0 {
		t.Errorf("Len = %d after clear", r.Len())
	}
	if len(r.History()) != 0 {
		t.Error("History not empty after clear")
	}
}

func TestOnLineCallback(t *testing.T) {
	r := New(5)
	got := []Entry{}
	r.SetOnLine(func(e Entry) { got = append(got, e) })
	r.Append("hello", false)
	if len(got) != 1 || got[0].Text != "hello" {
		t.Fatalf("callback got %+v", got)
	}
}

func TestWriterSplitsLines(t *testing.T) {
	r := New(10)
	restore := r.Attach()
	defer restore()

	fmt.Fprintln(os.Stdout, "hello world")
	fmt.Fprintln(os.Stdout, "second line")
	fmt.Fprintln(os.Stderr, "boom boom")
	// Esperar a que el goroutine lector procese las líneas.
	deadline := time.Now().Add(2 * time.Second)
	for r.Len() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if r.Len() != 3 {
		t.Fatalf("len = %d, want 3", r.Len())
	}
	h := r.History()
	// Búsqueda por contenido: el orden stdout/stderr puede intercalarse.
	find := func(text string, isErr bool) bool {
		for _, e := range h {
			if e.Text == text && e.IsError == isErr {
				return true
			}
		}
		return false
	}
	if !find("hello world", false) || !find("second line", false) {
		t.Errorf("missing stdout lines: %+v", h)
	}
	if !find("boom boom", true) {
		t.Errorf("missing stderr line: %+v", h)
	}
}
