package domain

import "strings"

// planBudgetPointer indica cómo recuperar el material omitido cuando el
// historial se recorta (contracts/hook-plan-entered.md, "Presupuesto del
// canal"). Va como último elemento del documento, nunca a mitad de frase.
const planBudgetPointer = "\n\n[...] Historial recortado por espacio. Llama a get_plan_context() para verlo completo."

// AdjustPlanDocumentToBudget ajusta el documento de planificación (método +
// historial, ya compuestos por el caso de uso correspondiente) al presupuesto
// del canal, con prioridad estricta (data-model.md, contracts/hook-plan-entered.md):
//
//  1. El método va SIEMPRE completo — nunca se recorta, ni siquiera si por sí
//     solo excede budget: su final contiene la autoverificación y el formato
//     de salida, la parte operativa del método.
//  2. El historial se recorta con lo que quede de presupuesto.
//  3. Si algo se omitió, se añade un puntero indicando cómo recuperar el
//     resto (get_plan_context()).
//
// budget es un parámetro, no una constante escondida: el presupuesto real de
// un canal de hook (9500 por defecto, ver hookPlanEntered) puede diferir del
// de otros canales o agentes.
func AdjustPlanDocumentToBudget(method, context string, budget int) string {
	method = strings.TrimSpace(method)
	context = strings.TrimSpace(context)

	if method == "" {
		return truncateAtBoundary(context, budget)
	}
	if context == "" {
		return method
	}

	remaining := budget - len(method) - len("\n\n") - len(planBudgetPointer)
	if remaining <= 0 {
		// El método (con el puntero) ya no cabe con margen: se emite
		// completo de todas formas, y se omite el historial por entero,
		// con el puntero indicando que hay más disponible.
		return method + planBudgetPointer
	}

	if len(context) <= remaining {
		return method + "\n\n" + context
	}

	trimmed := truncateAtBoundary(context, remaining)
	return method + "\n\n" + trimmed + planBudgetPointer
}

// truncateAtBoundary recorta s a lo sumo a n caracteres, cayendo en el
// último límite de párrafo ("\n\n") que quepa; si no hay ninguno, en el
// último límite de línea ("\n") que quepa; si tampoco, en el límite de n a
// secas. Nunca corta a mitad de frase cuando existe un límite más limpio.
func truncateAtBoundary(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	cut := s[:n]
	if idx := strings.LastIndex(cut, "\n\n"); idx > 0 {
		return strings.TrimSpace(cut[:idx])
	}
	if idx := strings.LastIndex(cut, "\n"); idx > 0 {
		return strings.TrimSpace(cut[:idx])
	}
	return cut
}
