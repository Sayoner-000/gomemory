package native

import (
	"strings"
	"testing"
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
