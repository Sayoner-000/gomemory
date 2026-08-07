package cli

import (
	"strings"
	"testing"
)

// planAtomicoDePrueba reproduce el formato exacto que produce el método de
// descomposición atómica: caracteres de dibujo, marcas de hoja, anotaciones de
// dependencia y paralelismo.
const planAtomicoDePrueba = `🎯 Añadir autenticación por token
├─ [1] Preparar el almacenamiento
│  ├─ [1.1] ✓ Crear la tabla tokens → migración aplicada
│  └─ [1.2] ✓ Añadir índice por user_id → índice existe   (dep: 1.1)
├─ [2] ⚠ no atómica → falta decidir la duración del token
└─ [3] ✓ Documentar el flujo en README → sección publicada   (∥)`

// TestExtractPlanFromPayload_ConservaElArbol es la verificación que D9 dejó
// pendiente: el árbol atómico debe sobrevivir intacto el viaje por el hook, sin
// que los caracteres de dibujo ni las anotaciones se mutilen. Si se perdieran,
// el plan dejaría de servir como contrato del objetivo (FR-035).
func TestExtractPlanFromPayload_ConservaElArbol(t *testing.T) {
	// Forma genérica: {"plan": "..."} — la que usan OpenCode y cualquier agente.
	generico := map[string]interface{}{"plan": planAtomicoDePrueba}
	if got := extractPlanFromPayload(generico); got != planAtomicoDePrueba {
		t.Errorf("la forma genérica mutiló el plan:\n%s", got)
	}

	// Forma de Claude Code: el plan viaja en tool_input.plan.
	claude := map[string]interface{}{
		"tool_input": map[string]interface{}{"plan": planAtomicoDePrueba},
	}
	got := extractPlanFromPayload(claude)
	if got != planAtomicoDePrueba {
		t.Errorf("la forma de Claude Code mutiló el plan:\n%s", got)
	}

	for _, marca := range []string{"├─", "│", "└─", "✓", "⚠", "dep: 1.1", "∥", "🎯"} {
		if !strings.Contains(got, marca) {
			t.Errorf("se perdió la marca %q del árbol", marca)
		}
	}
}

// TestPlanTitle_ConEmojiDeObjetivo cubre el segundo punto que D9 dejó por
// verificar: el título se deriva de la primera línea, que en el formato atómico
// empieza por el emoji de objetivo. Debe salir legible, no vacío ni truncado en
// el emoji.
func TestPlanTitle_ConEmojiDeObjetivo(t *testing.T) {
	got := planTitle(planAtomicoDePrueba)

	if !strings.Contains(got, "Añadir autenticación por token") {
		t.Errorf("el título debe conservar el objetivo legible, se obtuvo %q", got)
	}
	if strings.HasSuffix(strings.TrimSpace(got), "🎯") {
		t.Errorf("el título no debe quedarse en el emoji: %q", got)
	}
}

// TestPlanTitle_PlanVacio protege el caso degenerado.
func TestPlanTitle_PlanVacio(t *testing.T) {
	if got := planTitle("   \n\n  "); got != "Plan aprobado: plan aprobado" {
		t.Errorf("planTitle con plan vacío = %q", got)
	}
}

// TestExtractPlanFromPayload_SinPlan devuelve cadena vacía para que el hook no
// guarde una memoria sin contenido.
func TestExtractPlanFromPayload_SinPlan(t *testing.T) {
	if got := extractPlanFromPayload(map[string]interface{}{"otra": "cosa"}); got != "" {
		t.Errorf("sin campo plan se esperaba cadena vacía, se obtuvo %q", got)
	}
}
