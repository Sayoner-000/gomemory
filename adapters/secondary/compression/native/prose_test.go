package native

import (
	"fmt"
	"strings"
	"testing"

	"mem/domain"
)

// T021 — prosa: sin frases alteradas, duplicadas fuera, protegidas dentro.
func TestCompressProse(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("Primera frase del párrafo largo que explica el contexto general. ")
	for i := 0; i < 12; i++ {
		sb.WriteString("Una frase intermedia que no aporta nada nuevo al lector número ")
		sb.WriteString(strings.Repeat("x", i+1))
		sb.WriteString(". ")
	}
	sb.WriteString("La ruta afectada es adapters/primary/cli/cmd_pack.go en la versión 2.25.0. ")
	sb.WriteString("Penúltima frase de cierre. Última frase de cierre.\n\n")
	sb.WriteString("Una frase repetida que aparece dos veces en el documento completo.\n")
	sb.WriteString("Una frase repetida que aparece dos veces en el documento completo.\n")
	in := sb.String()
	out, n := compressProse(in, "abc123abc123", maxTh)
	if n == 0 {
		t.Fatal("se esperaba compresión")
	}
	if err := checkLiteral(in, out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "adapters/primary/cli/cmd_pack.go en la versión 2.25.0.") {
		t.Error("la frase con ruta y versión debe conservarse")
	}
	if strings.Count(out, "Una frase repetida que aparece dos veces") != 1 {
		t.Error("la frase duplicada debe quedar una sola vez")
	}
	if !strings.Contains(out, "Primera frase") || !strings.Contains(out, "Última frase de cierre.") {
		t.Error("cabeza y cola del párrafo deben conservarse")
	}
}

// C-004 (acr_961a1676) — una línea estructural duplicada que se omite deja su
// marca con la ref, como toda omisión con pérdida (FR-012).
func TestCompressProseDuplicateStructuralLineLeavesMarker(t *testing.T) {
	bullet := "- Se valida que el token de autenticación expira correctamente en veinticuatro horas"
	in := bullet + "\nTexto intermedio.\n" + bullet + "\n" + bullet + "\nCierre."
	out, n := compressProse(in, "abc123abc123", maxTh)
	if n != 2 {
		t.Fatalf("omisiones = %d, se esperaban 2", n)
	}
	if strings.Count(out, bullet) != 1 {
		t.Errorf("la línea duplicada debe quedar una sola vez:\n%s", out)
	}
	if strings.Count(out, "⟦mem⟧") != 1 || !strings.Contains(out, "2 líneas duplicadas omitidas · ref=abc123abc123") {
		t.Errorf("las duplicadas consecutivas deben dejar una sola marca con recuento y ref:\n%s", out)
	}
	if err := checkLiteral(in, out); err != nil {
		t.Fatal(err)
	}
}

// A-R01 (acr_e366819b) — la limpieza estructural posterior no fusiona marcas
// idénticas: cada omisión conserva la suya en su lugar (FR-012).
func TestEngineKeepsIdenticalOmissionMarkers(t *testing.T) {
	bullet := "- Se valida que el token de autenticación expira correctamente en veinticuatro horas"
	// Párrafos largos: el motor gana a la compresión estructural y no degrada.
	long := func(p int) string {
		var sb strings.Builder
		for i := 0; i < 12; i++ {
			fmt.Fprintf(&sb, "Frase %d del párrafo %d que no aporta nada nuevo al lector. ", i, p)
		}
		return sb.String()
	}
	in := bullet + "\n\n" + long(1) + "\n\n" + bullet + "\n\n" + long(2) + "\n\n" + bullet + "\n\n" + long(3) + "\n"
	r, err := NewEngine(newMemStore(), nil, nil, "p").Compress(in, maxOpts)
	if err != nil || !r.Compressed || r.FallbackReason != "" {
		t.Fatalf("se esperaba compresión del motor: %v %+v", err, r)
	}
	if got := strings.Count(r.Content, "⟦mem⟧ 1 líneas duplicadas omitidas"); got != 2 {
		t.Errorf("cada línea omitida debe conservar su marca: %d marcas\n%s", got, r.Content)
	}
}

// C-002 (acr_6793454b) — la prosa no trata como duplicadas las líneas que son
// marcas de gomemory (de entrega u omisión): borrarlas o sustituirlas quitaría
// el aviso de su sección.
func TestCompressProseKeepsRepeatedMarkerLines(t *testing.T) {
	entregada := "- **Título largo de una entrada cualquiera del proyecto** " + domain.DeliveredTag
	agrupada := "- " + domain.MarkerTag + " 3 entradas ya entregadas en esta sesión, sin cambios"
	in := "## Decisiones\n\n" + entregada + "\n" + agrupada + "\n\n## Bugfixes\n\n" + entregada + "\n" + agrupada + "\n"
	out, _ := compressProse(in, "abc123abc123", maxTh)
	if strings.Count(out, entregada) != 2 || strings.Count(out, agrupada) != 2 {
		t.Errorf("las líneas de marca repetidas deben conservarse:\n%s", out)
	}
	if strings.Contains(out, "líneas duplicadas omitidas") {
		t.Errorf("una marca no es una línea duplicada:\n%s", out)
	}
}
