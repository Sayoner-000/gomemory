package hcdfiles

import (
	"mem/domain"
	"strings"
	"testing"
)

func TestHCDDeliveryRoundTrip(t *testing.T) {
	d := domain.Delivery{Slices: []domain.Slice{{ID: "VS-01", Observable: "Guardar datos", Limits: "Sin borrar", Verification: "Test", Stories: []string{"US1"}, Criteria: []string{"C-01"}}}, Tasks: []domain.Task{{ID: "T-001", Title: "Guardar datos", SliceID: "VS-01", Result: "Datos guardados", Scope: "Persistencia", Excluded: "Borrado", Verification: "Test"}}}
	content := RenderDelivery("HCD-001", d)
	got, e := ParseDelivery(content)
	if e != nil {
		t.Fatal(e)
	}
	if domain.ContentRevision(RenderDelivery("HCD-001", got)) != domain.ContentRevision(content) {
		t.Fatalf("roundtrip changed content: %+v", got)
	}
}

// El título del encabezado se perdía (se escribía el comportamiento observable)
// y una lista de dependencias vacía salía como «Dependencias: » sin valor.
func TestHCDDeliveryKeepsSliceTitleAndWritesNinguna(t *testing.T) {
	in := []byte("# Entrega HCD-001\n\n## VS-01. Resume sin pánico\nHistorias: US1\nCriterios: C-01\nDependencias: ninguna\n#### Comportamiento observable\nDevuelve (\"\", false)\n#### Límites\nSolo store.go\n#### Verificación esperada\ngo test\n")
	d, e := ParseDelivery(in)
	if e != nil || d.Slices[0].Title != "Resume sin pánico" {
		t.Fatalf("slice=%+v err=%v", d.Slices, e)
	}
	out := string(RenderDelivery("HCD-001", d))
	for _, want := range []string{"## VS-01. Resume sin pánico\n", "Dependencias: ninguna\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("render lacks %q:\n%s", want, out)
		}
	}
}
