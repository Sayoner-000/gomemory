package usecases

import (
	"strings"

	"mem/application/ports"
)

// PlanContext compone el documento que el agente recibe al entrar en modo plan
// (feature 013): el método de descomposición atómica seguido del contexto
// histórico del proyecto.
//
// El método llega como texto INYECTADO, no lo lee este caso de uso: la capa de
// aplicación no importa infraestructura (constitución, principio I), y así las
// tres ramas de salida quedan comprobables con un doble de prueba.
//
// El contexto se obtiene SIEMPRE llamando a ContextBuilder.Build(), nunca
// reconstruyéndolo aquí: el techo de caracteres de settings.Budget se aplica
// dentro de ese caso de uso, así que duplicar la lógica rompería el presupuesto
// en silencio (feature 013, FR-007).
type PlanContext struct {
	method  string
	context ports.ContextBuilder
}

func NewPlanContext(method string, context ports.ContextBuilder) *PlanContext {
	return &PlanContext{method: method, context: context}
}

// Build devuelve el documento de planificación. Nunca falla por causas
// ambientales: un modo plan no puede quedar interrumpido porque la memoria no
// esté inicializada (FR-034). Las tres ramas son:
//
//   - disabled=true          → salida vacía, sin construir contexto (FR-032)
//   - contexto no disponible → solo el método (FR-034)
//   - todo normal            → método + contexto
//
// La distinción entre las dos primeras es deliberada: la ausencia de historial
// es una circunstancia y no debe costarle al usuario el método, mientras que el
// apagado explícito sí es una preferencia y silencia todo.
func (p *PlanContext) Build(disabled bool) (string, error) {
	if disabled {
		return "", nil
	}

	method := strings.TrimSpace(p.method)

	// Un error aquí es esperable (proyecto sin memoria inicializada) y se trata
	// como "no hay contexto", no como fallo: degradar en silencio es el criterio
	// ya establecido para toda la integración con agentes.
	context, err := p.context.Build()
	if err != nil {
		context = ""
	}
	context = strings.TrimSpace(context)

	switch {
	case method == "":
		return context, nil
	case context == "":
		return method, nil
	default:
		return method + "\n\n---\n\n" + context, nil
	}
}
