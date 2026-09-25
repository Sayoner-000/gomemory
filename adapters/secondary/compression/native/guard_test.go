package native

import (
	"regexp"
	"strings"
	"testing"
)

// T017 — la guarda acepta omitir y rechaza alterar.
func TestLiteralGuard(t *testing.T) {
	in := "línea uno\nlínea dos\nlínea tres\nlínea cuatro\n"
	ok := "línea uno\n⟦mem⟧ ×2 líneas similares · ref=abc123abc123\nlínea cuatro\n"
	if err := checkLiteral(in, ok); err != nil {
		t.Errorf("omitir debe aceptarse: %v", err)
	}
	bad := "línea uno\n⟦mem⟧ ×2 líneas similares · ref=abc123abc123\nlínea cuatrO\n"
	if checkLiteral(in, bad) == nil {
		t.Error("un carácter cambiado debe rechazarse")
	}
	reordered := "línea cuatro\nlínea uno\n"
	if checkLiteral(in, reordered) == nil {
		t.Error("reordenar debe rechazarse")
	}

	js := `[{"id": 1, "v": "a"}, {"id": 2, "v": "b"}, {"id": 3, "v": "c"}]`
	jsOK := `[{"id": 1, "v": "a"}, {"⟦mem⟧": "omitidos 1 de 3 elementos", "ref": "abc"}, {"id": 3, "v": "c"}]`
	if err := checkLiteral(js, jsOK); err != nil {
		t.Errorf("marcador JSON debe aceptarse: %v", err)
	}
	jsBad := `[{"id": 1, "v": "a"}, {"⟦mem⟧": "x", "ref": "abc"}, {"id": 3, "v": "C"}]`
	if checkLiteral(js, jsBad) == nil {
		t.Error("un valor JSON cambiado debe rechazarse")
	}

	goSrc := "func X(a int) error {\n\tx := 1\n\treturn nil\n}\n"
	goOK := "func X(a int) error { /* ⟦mem⟧ cuerpo omitido: 2 líneas · ref=abc */ }\n"
	if err := checkLiteral(goSrc, goOK); err != nil {
		t.Errorf("cuerpo Go omitido debe aceptarse: %v", err)
	}
	goBad := "func X(a int64) error { /* ⟦mem⟧ cuerpo omitido: 2 líneas · ref=abc */ }\n"
	if checkLiteral(goSrc, goBad) == nil {
		t.Error("una firma alterada debe rechazarse")
	}
}

var protectedRe = regexp.MustCompile("(/[\\w.-]+)+\\.\\w+|https?://\\S+|\\bv\\d+\\.\\d+\\.\\d+\\b|ERROR[^\\n]*")

// SC-006 — todo elemento protegido que aparece en la salida aparece literal, y
// las líneas de ERROR nunca se omiten.
func TestProtectedTokens(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 60; i++ {
		b.WriteString("2026-09-25 10:00:00 INFO sincronizando /srv/app/datos/lote.csv desde https://api.example.com/v1/lotes con cliente v2.25.0\n")
		if i == 30 {
			b.WriteString("2026-09-25 10:00:00 ERROR fallo al abrir /srv/app/datos/lote-30.csv: permission denied (https://docs.example.com/e/13)\n")
		}
	}
	in := b.String()
	out, n := compressLog(in, "abc123abc123", maxTh)
	if n == 0 {
		t.Fatal("se esperaba compresión")
	}
	if err := checkLiteral(in, out); err != nil {
		t.Fatal(err)
	}
	for _, tok := range protectedRe.FindAllString(out, -1) {
		if !strings.Contains(in, tok) {
			t.Errorf("elemento protegido alterado: %q", tok)
		}
	}
	if !strings.Contains(out, "ERROR fallo al abrir /srv/app/datos/lote-30.csv: permission denied (https://docs.example.com/e/13)") {
		t.Error("la línea ERROR debe conservarse literal")
	}
}
