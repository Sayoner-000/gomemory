package console

import (
	"bytes"
	"strings"
	"testing"
)

// La UI pinta título, etiquetas y pistas sin secuencias de control, y la
// selección devuelve el Value original (H-01, auditoría 2026-10-17 ronda 3).
func TestSafeUI_PintaSinSecuenciasYDevuelveElValor(t *testing.T) {
	var out bytes.Buffer
	opts := []Option{{Value: "uno\x1b", Label: "Equipo \x1b]0;PWNED\x07", Hint: "pista \x1b[2J"}, {Value: "dos", Label: "Cliente"}}
	got, err := NewPlain(strings.NewReader("1\n"), &out).Select("Pregunta \x1b]52;c;aGFjaw==\x07", opts)
	if err != nil || got != "uno\x1b" {
		t.Fatalf("got %q, %v", got, err)
	}
	if strings.ContainsAny(out.String(), "\x1b\x07") || !strings.Contains(out.String(), "]0;PWNED") {
		t.Fatalf("pintó secuencias o perdió el texto visible:\n%q", out.String())
	}
	if opts[0].Label != "Equipo \x1b]0;PWNED\x07" {
		t.Fatal("modificó las opciones del llamador")
	}
}
