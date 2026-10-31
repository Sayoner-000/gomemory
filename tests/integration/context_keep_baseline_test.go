package main

import (
	"strings"
	"testing"
)

// OpenCode reconstruye el system en cada turno: lo elidido desaparece.
func TestContextKeepBaseline(t *testing.T) {
	bin := buildMemBinary(t)
	dir := proyectoCompresion(t, `{"context_compression_level": "max"}`)
	correrMem(t, bin, dir, nil, "", "session", "start")
	const cuerpo = "Universal Agent Instructions"
	for turno := 1; turno <= 3; turno++ {
		if out := correrMem(t, bin, dir, nil, "", "context", "--keep-baseline").stdout; !strings.Contains(out, cuerpo) {
			t.Fatalf("turno %d sin baseline:\n%s", turno, out)
		}
	}
	correrMem(t, bin, dir, nil, "", "context")
	if out := correrMem(t, bin, dir, nil, "", "context").stdout; strings.Contains(out, cuerpo) {
		t.Fatal("sin el flag el delta debe seguir eliminando el baseline repetido")
	}
}
