package testutil

import (
	"net"
	"testing"
)

// FreePort devuelve un puerto TCP libre en loopback (listen en :0 y cierre
// inmediato). Evita que los tests dependan de que un puerto fijo (5173) esté
// libre en la máquina: si otro proceso lo ocupa —p. ej. el dev server de otro
// proyecto—, el arranque del server toma la rama "puerto ocupado" y el test
// falla por algo ajeno al código.
func FreePort(t testing.TB) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("no se pudo reservar un puerto libre: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("cerrar el listener de puerto libre: %v", err)
	}
	return port
}
