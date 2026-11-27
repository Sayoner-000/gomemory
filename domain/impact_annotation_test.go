package domain

import (
	"fmt"
	"testing"
)

// legacyImpactNote reproduce la nota que InsertMemory escribía antes de
// HCD-069: las memorias antiguas la conservan y StripImpactAnnotation la quita.
func legacyImpactNote(symbol string, fanIn int) string {
	return fmt.Sprintf("\n\n[impacto: %s es un hotspot con %d llamadores directos]", symbol, fanIn)
}

// S-001 (3ª revisión 031): solo se quita la nota con su forma exacta. Un texto
// citado que empieza como la nota y acaba igual no debe perder su cola.
func TestStripImpactAnnotation_SoloQuitaLaNotaExacta(t *testing.T) {
	quoted := "real\n\n[impacto: x] texto citado largo que sigue es un hotspot con 3 llamadores directos]"
	if got := StripImpactAnnotation(quoted); got != quoted {
		t.Fatalf("un texto con forma parecida no debe truncarse: %q", got)
	}
}

// S-001 (4ª revisión 031): el símbolo es un nombre del grafo de código, sin
// espacios pero con corchetes posibles (Cache[T]); su nota también se quita.
func TestStripImpactAnnotation_SimboloConCorchetes(t *testing.T) {
	base := "caché genérica"
	if got := StripImpactAnnotation(base + legacyImpactNote("Cache[T]", 3)); got != base {
		t.Fatalf("la nota de un símbolo con corchetes debe quitarse: %q", got)
	}
}

func TestStripImpactAnnotation(t *testing.T) {
	base := "usamos redis como caché"
	if got := StripImpactAnnotation(base + legacyImpactNote("BuildContext", 42)); got != base {
		t.Fatalf("quitar la anotación debe devolver el contenido original: %q", got)
	}
	for _, untouched := range []string{
		base,
		"el texto menciona [impacto: algo] en medio y sigue",
		base + "\n\n[impacto: incompleto",
	} {
		if got := StripImpactAnnotation(untouched); got != untouched {
			t.Fatalf("sin la anotación final el contenido no cambia: %q → %q", untouched, got)
		}
	}
}
