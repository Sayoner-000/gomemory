package cli

import (
	"strings"
	"testing"

	"mem/domain"
)

func TestRenderSearchResults_UsaExtractoNoIntegro(t *testing.T) {
	full := strings.Repeat("contenido largo de la memoria ", 20) // ~600 chars
	mems := []domain.Memory{
		{ID: 7, Type: domain.Decision, Title: "una decisión", Content: full},
	}
	out := renderSearchResults(mems)

	if strings.Contains(out, full) {
		t.Fatalf("search_memories no debe volcar el contenido íntegro, got:\n%s", out)
	}
	if !strings.Contains(out, "[7]") || !strings.Contains(out, "una decisión") {
		t.Fatalf("esperaba id y título en el resultado, got:\n%s", out)
	}
	// Cada resultado acotado (referencia ~160 chars + formato); muy por debajo del íntegro.
	if len(out) >= len(full) {
		t.Fatalf("el resultado debería ser mucho más corto que el íntegro (%d >= %d)", len(out), len(full))
	}
}

func TestRenderMemoryDetail_DevuelveIntegro(t *testing.T) {
	full := strings.Repeat("detalle completo sin truncar ", 20)
	m := domain.Memory{ID: 3, Type: domain.Bugfix, Title: "bug", Content: full, CreatedAt: "2026-01-01"}
	out := renderMemoryDetail(m)

	if !strings.Contains(out, full) {
		t.Fatalf("get_memory (detalle) debe devolver el contenido íntegro, got:\n%s", out)
	}
}
