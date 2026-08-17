package domain

import (
	"strings"
	"testing"
)

func repeatSentence(s string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

const shortMethod = "MÉTODO\nprimera línea del método\n...\núltima línea del método."

func TestAdjustPlanDocumentToBudget_ContenidoQueCabeEnteroSeConserva(t *testing.T) {
	context := repeatSentence("Una decisión previa registrada en el historial del proyecto.", 3)
	got := AdjustPlanDocumentToBudget(shortMethod, context, 10000)

	if !strings.HasPrefix(got, "MÉTODO") || !strings.Contains(got, "última línea del método.") {
		t.Errorf("el método debe aparecer íntegro (primera y última línea), got %q", got)
	}
	if !strings.Contains(got, "Una decisión previa registrada") {
		t.Error("el historial debe aparecer si cabe entero")
	}
	if strings.Contains(got, "recortado") {
		t.Error("no debe aparecer el puntero de recorte si todo cupo")
	}
}

// TestAdjustPlanDocumentToBudget_ContextoDe24000CaracteresNoExcedeElTope
// reproduce la colisión real medida en research.md §4: Budget por defecto de
// get_context es 24000 caracteres y el método ocupa ~4200 — el documento sin
// recortar excede cualquier tope razonable de canal.
func TestAdjustPlanDocumentToBudget_ContextoDe24000CaracteresNoExcedeElTope(t *testing.T) {
	context := repeatSentence("Contexto histórico del proyecto con decisiones, bugfixes y convenciones.", 400)
	if len(context) < 20000 {
		t.Fatalf("el contexto de prueba debe simular ~24000 caracteres, got %d", len(context))
	}

	const budget = 9500
	got := AdjustPlanDocumentToBudget(shortMethod, context, budget)

	if len(got) > budget {
		t.Errorf("el resultado (%d chars) excede el tope de %d", len(got), budget)
	}
	if !strings.HasPrefix(got, "MÉTODO") || !strings.Contains(got, "última línea del método.") {
		t.Error("el método debe aparecer íntegro incluso cuando el historial se recorta")
	}
}

func TestAdjustPlanDocumentToBudget_CorteEnLimiteDeParrafo(t *testing.T) {
	context := "Primer párrafo con contenido relevante.\n\nSegundo párrafo con más contenido relevante que sigue después.\n\nTercer párrafo que probablemente quede fuera del presupuesto disponible."
	// Presupuesto pequeño, suficiente solo para el método + el primer párrafo.
	budget := len(shortMethod) + len("\n\n") + len("Primer párrafo con contenido relevante.") + 60

	got := AdjustPlanDocumentToBudget(shortMethod, context, budget)

	if strings.Contains(got, "Segundo párraf") && !strings.Contains(got, "Segundo párrafo con más contenido relevante que sigue después.") {
		t.Errorf("el corte no debe caer a mitad del segundo párrafo, got %q", got)
	}
	if strings.HasSuffix(strings.TrimSpace(strings.Split(got, "recortado")[0]), "relev") {
		t.Errorf("el corte no debe caer a mitad de palabra, got %q", got)
	}
}

func TestAdjustPlanDocumentToBudget_PunteroCuandoSeRecorta(t *testing.T) {
	context := repeatSentence("Historial extenso que no cabe completo en el presupuesto disponible del canal.", 50)
	got := AdjustPlanDocumentToBudget(shortMethod, context, 1000)

	if !strings.Contains(got, "get_plan_context()") {
		t.Error("cuando se recorta, debe indicarse cómo recuperar el resto (get_plan_context())")
	}
}

func TestAdjustPlanDocumentToBudget_MetodoSoloNoCabeSeEmiteCompletoSinHistorial(t *testing.T) {
	context := repeatSentence("Cualquier historial, por corto que sea, no debe aparecer aquí.", 5)
	// Presupuesto menor que el propio método: ni el método cabría con margen.
	budget := len(shortMethod) - 10

	got := AdjustPlanDocumentToBudget(shortMethod, context, budget)

	if !strings.HasPrefix(got, "MÉTODO") || !strings.Contains(got, "última línea del método.") {
		t.Errorf("el método debe emitirse COMPLETO aunque exceda el presupuesto, got %q", got)
	}
	if !strings.Contains(got, "get_plan_context()") {
		t.Error("debe indicarse cómo recuperar el historial omitido")
	}
	if strings.Contains(got, "Cualquier historial") {
		t.Error("el historial debe omitirse por completo cuando el método solo ya no cabe")
	}
}

func TestAdjustPlanDocumentToBudget_TopeEsParametro(t *testing.T) {
	context := repeatSentence("Contenido de historial de prueba para verificar el parámetro de tope.", 30)

	got9500 := AdjustPlanDocumentToBudget(shortMethod, context, 9500)
	got500 := AdjustPlanDocumentToBudget(shortMethod, context, 500)

	if len(got9500) <= len(got500) {
		t.Errorf("un tope mayor debe producir (o igualar) un documento más largo: 9500=%d 500=%d",
			len(got9500), len(got500))
	}
	if len(got500) > 500 {
		t.Errorf("con tope 500 el resultado no debe exceder 500, got %d", len(got500))
	}
}

func TestAdjustPlanDocumentToBudget_SinContextoDevuelveSoloElMetodo(t *testing.T) {
	got := AdjustPlanDocumentToBudget(shortMethod, "", 9500)
	if got != shortMethod {
		t.Errorf("sin contexto, el resultado debe ser exactamente el método, got %q", got)
	}
}

func TestAdjustPlanDocumentToBudget_SinMetodoNiContextoDevuelveVacio(t *testing.T) {
	if got := AdjustPlanDocumentToBudget("", "", 9500); got != "" {
		t.Errorf("sin método ni contexto, esperaba cadena vacía, got %q", got)
	}
}
