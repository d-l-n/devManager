// Package logger porta app/utils/app_logger.py: captura stdout/stderr de la
// app en un ring buffer accesible desde la UI (App Log global).
package logger

import (
	"os"
	"strings"
	"sync"
	"time"
)

// Entry es una línea capturada: timestamp, texto y si es error (stderr).
type Entry struct {
	TS      string `json:"ts"`
	Text    string `json:"text"`
	IsError bool   `json:"isError"`
}

// Ring es un buffer circular de líneas de log.
type Ring struct {
	mu     sync.Mutex
	lines  []Entry
	max    int
	next   int
	full   bool
	onLine func(Entry) // callback por línea nueva; se invoca FUERA del lock
}

// New crea un ring de hasta max líneas.
func New(max int) *Ring {
	return &Ring{max: max, lines: make([]Entry, max)}
}

// SetOnLine registra un callback por cada línea nueva (para emitir eventos
// al frontend). Se invoca SIN el lock tomado.
func (r *Ring) SetOnLine(fn func(Entry)) {
	r.mu.Lock()
	r.onLine = fn
	r.mu.Unlock()
}

// Timestamp devuelve la hora actual en HH:MM:SS (paridad strftime).
func Timestamp() string {
	return time.Now().Format("15:04:05")
}

// Append agrega una entrada al ring y notifica al callback.
func (r *Ring) Append(text string, isError bool) {
	if text == "" {
		return
	}
	e := Entry{TS: Timestamp(), Text: text, IsError: isError}
	r.mu.Lock()
	r.lines[r.next] = e
	r.next = (r.next + 1) % r.max
	if r.next == 0 {
		r.full = true
	}
	var cb func(Entry)
	if r.onLine != nil {
		cb = r.onLine
	}
	r.mu.Unlock()

	if cb != nil {
		cb(e)
	}
}

// History devuelve todas las líneas capturadas en orden cronológico.
func (r *Ring) History() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	if r.full {
		n = r.max
	} else {
		n = r.next
	}
	out := make([]Entry, 0, n)
	start := r.next - n
	if start < 0 {
		start += r.max
	}
	for i := 0; i < n; i++ {
		out = append(out, r.lines[(start+i)%r.max])
	}
	return out
}

// Clear vacía el ring.
func (r *Ring) Clear() {
	r.mu.Lock()
	r.lines = make([]Entry, r.max)
	r.next = 0
	r.full = false
	r.mu.Unlock()
}

// Len devuelve cuántas líneas hay capturadas.
func (r *Ring) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.full {
		return r.max
	}
	return r.next
}

// Attach redirige os.Stdout y os.Stderr hacia el ring por medio de pipes
// (mantiene el tipo *os.File de esos streams). El output original sigue
// escribiéndose a la consola. Devuelve una función que restaura los streams.
func (r *Ring) Attach() func() {
	origOut, origErr := os.Stdout, os.Stderr

	outW, errW := r.pipe(origOut, false), r.pipe(origErr, true)

	os.Stdout = outW
	os.Stderr = errW

	return func() {
		os.Stdout = origOut
		os.Stderr = origErr
	}
}

// pipe crea un *os.File para redirigir un stream y un goroutine lector
// que vuelca las líneas al ring y al stream original.
func (r *Ring) pipe(orig *os.File, isError bool) *os.File {
	pr, pw, err := os.Pipe()
	if err != nil {
		// Si falla, no redirigir: devolver un escritor no-op al stream original.
		return orig
	}
	go func() {
		buf := make([]byte, 4096)
		carry := []byte{}
		for {
			n, err := pr.Read(buf)
			if n > 0 {
				chunk := append(carry, buf[:n]...)
				start := 0
				for i := 0; i < len(chunk); i++ {
					if chunk[i] == '\n' {
						line := strings.TrimRight(string(chunk[start:i]), "\r\n")
						r.Append(line, isError)
						start = i + 1
					}
				}
				carry = append([]byte(nil), chunk[start:]...)
				if orig != nil {
					_, _ = orig.Write(buf[:n])
				}
			}
			if err != nil {
				return
			}
		}
	}()
	return pw
}
