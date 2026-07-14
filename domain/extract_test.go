package domain

import (
	"strings"
	"testing"
)

func TestExtract(t *testing.T) {
	long := "Primera oración corta. " + strings.Repeat("palabra ", 100)

	tests := []struct {
		name     string
		in       string
		max      int
		wantFull bool   // espera la entrada intacta (trim)
		want     string // si != "" se compara exacto
	}{
		{name: "texto corto intacto", in: "hola mundo", max: 200, wantFull: true},
		{name: "entrada vacía", in: "", max: 200, want: ""},
		{name: "espacios se recortan a vacío", in: "   \n  ", max: 200, want: ""},
		{name: "max<=0 devuelve intacto", in: long, max: 0, wantFull: true},
		{name: "primera oración cuando cabe", in: long, max: 200, want: "Primera oración corta."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Extract(tt.in, tt.max)
			if tt.wantFull {
				if got != strings.TrimSpace(tt.in) {
					t.Fatalf("esperaba intacto %q, obtuve %q", strings.TrimSpace(tt.in), got)
				}
				return
			}
			if tt.want != "" || tt.in == "" || strings.TrimSpace(tt.in) == "" {
				if got != tt.want {
					t.Fatalf("esperaba %q, obtuve %q", tt.want, got)
				}
			}
		})
	}
}

func TestExtract_TruncaSinCortarPalabra(t *testing.T) {
	// Un solo bloque sin punto: debe truncar por límite de palabra y añadir "…".
	in := strings.Repeat("palabra ", 100) // sin ". "
	got := Extract(in, 30)
	if len([]rune(got)) > 30 {
		t.Fatalf("extracto excede el techo: %d runas (%q)", len([]rune(got)), got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("esperaba sufijo … en truncado, obtuve %q", got)
	}
	// No debe terminar en una palabra cortada: el texto antes de … debe ser
	// prefijo por palabras completas de la entrada.
	body := strings.TrimSuffix(got, "…")
	if !strings.HasPrefix(strings.TrimSpace(in), strings.TrimSpace(body)) {
		t.Fatalf("truncado cortó una palabra: %q", got)
	}
}
