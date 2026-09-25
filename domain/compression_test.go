package domain

import (
	"strings"
	"testing"
)

func TestRenderTextMarker(t *testing.T) {
	got := RenderTextMarker(Omission{Ref: "c7a1f9e2b3d4", Summary: "×310 líneas similares"})
	if got != "⟦mem⟧ ×310 líneas similares · ref=c7a1f9e2b3d4" {
		t.Errorf("marcador inesperado: %q", got)
	}
	// Sin ref no se promete una recuperación imposible.
	if got := RenderTextMarker(Omission{Summary: "x"}); strings.Contains(got, "ref=") {
		t.Errorf("marcador sin ref no debe llevar ref=: %q", got)
	}
}

func TestRenderJSONMarker(t *testing.T) {
	m := RenderJSONMarker(Omission{Ref: "abc", Summary: "omitidos 3 de 9 elementos"}, map[string]string{"status": "200×3"})
	if m[MarkerTag] != "omitidos 3 de 9 elementos" || m["ref"] != "abc" {
		t.Errorf("marcador JSON incompleto: %v", m)
	}
	if _, ok := m["campos"]; !ok {
		t.Errorf("falta el resumen de campos: %v", m)
	}
	if _, ok := RenderJSONMarker(Omission{Summary: "s"}, nil)["campos"]; ok {
		t.Error("sin extra no debe emitir campos")
	}
}

func TestIsMarkerLine(t *testing.T) {
	if !IsMarkerLine("  ⟦mem⟧ cuerpo omitido · ref=1") {
		t.Error("debe reconocer el marcador")
	}
	if IsMarkerLine("[mem] no es un marcador") {
		t.Error("no debe confundir corchetes normales")
	}
}

func TestRefFromHash(t *testing.T) {
	h := strings.Repeat("ab", 32)
	if got := RefFromHash(h, RefLen); len(got) != 12 {
		t.Errorf("ref de 12 esperada, obtenida %q", got)
	}
	if got := RefFromHash(h, RefLenCollision); len(got) != 16 || !strings.HasPrefix(h, got) {
		t.Errorf("ref ampliada inválida: %q", got)
	}
	if got := RefFromHash(h, 0); got != h {
		t.Error("n fuera de rango debe devolver la huella completa")
	}
}

func TestIsPrivateContent(t *testing.T) {
	if !IsPrivateContent("x <private>secreto</private> y") {
		t.Error("<private> debe detectarse")
	}
	if IsPrivateContent("texto normal sin secretos") {
		t.Error("falso positivo en texto normal")
	}
}
