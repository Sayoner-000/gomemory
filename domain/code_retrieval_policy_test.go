package domain

import (
	"strings"
	"testing"
)

// HCD-021: agy tampoco comprime resultados externos (PostToolUse sin salida);
// la política le dice qué tools de gomemory llegan comprimidas en origen y no
// promete compresión de get_symbol, que el servidor no comprime.
func TestCodeRetrievalPolicy_Antigravity(t *testing.T) {
	for _, want := range []string{"Antigravity", "get_context", "search_memories", "search_code", "get_symbol"} {
		if !strings.Contains(CodeRetrievalPolicy, want) {
			t.Errorf("la política no menciona %q", want)
		}
	}
	if !strings.Contains(CodeRetrievalPolicy, "get_symbol para firma, ubicación y relaciones, sin asumir que entrega cuerpos o compresión") {
		t.Error("la política no debe prometer compresión de get_symbol")
	}
}
