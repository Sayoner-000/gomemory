package native

import (
	"fmt"
	"strings"
	"testing"
)

// TestEngineKeep_CompresorSinKeepDejaElBloqueLiteral fija la guarda genérica de
// ports.CompressionOptions.Keep (HCD-004): el compresor de logs no conoce keep,
// así que el bloque que perdería el literal sale entero en vez de esconderlo.
func TestEngineKeep_CompresorSinKeepDejaElBloqueLiteral(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "2026-10-29 10:00:%02d INFO worker %d procesó el lote\n", i, i)
	}
	in := b.String()
	const literal = "INFO worker 20 procesó"
	e := NewEngine(newMemStore(), nil, nil, "p")

	control, _ := e.Compress(in, maxOpts)
	if strings.Contains(control.Content, literal) {
		t.Fatalf("control: sin keep el log debía omitir la línea 20:\n%s", control.Content)
	}
	opts := maxOpts
	opts.Keep = []string{literal}
	r, _ := e.Compress(in, opts)
	if !strings.Contains(r.Content, literal) {
		t.Fatalf("con keep la línea 20 debía seguir visible:\n%s", r.Content)
	}
}

// TestCompressTableKeeping_ConservaFilaDelLiteral: la tabla conserva la fila
// que contiene un literal de keep y sigue omitiendo las demás intermedias.
func TestCompressTableKeeping_ConservaFilaDelLiteral(t *testing.T) {
	var b strings.Builder
	b.WriteString("ID Tipo Título\n-- ---- ------\n")
	for i := 0; i < 20; i++ {
		fmt.Fprintf(&b, "%d decision título %02d\n", 100+i, i)
	}
	out, n := compressTableKeeping(b.String(), "ref", maxTh, []string{"título 10"})
	if n == 0 || !strings.Contains(out, "110 decision título 10") {
		t.Fatalf("la fila 110 debía conservarse con omisiones alrededor (n=%d):\n%s", n, out)
	}
}
