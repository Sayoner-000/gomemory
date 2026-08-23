package usecases

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"mem/application/ports"
)

// HashDeContenido identifica un texto para poder reconocer si ya se entregó.
//
// Compara contenido, no significado: dos textos que dicen lo mismo con palabras
// distintas no se consideran el mismo material. Es una limitación aceptada y
// declarada, no un descuido — reconocer equivalencia semántica costaría mucho
// más de lo que ahorraría.
func HashDeContenido(s string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(s)))
	return hex.EncodeToString(sum[:16])
}

// avisoDeSupresion explica por qué falta el historial y dónde está.
//
// Suprimir material sin decirlo dejaría al agente sin saber si el proyecto no
// tiene historial o si simplemente no se le reenvió (FR-007).
const avisoDeSupresion = "> El historial del proyecto ya está disponible en esta sesión: se entregó\n" +
	"> al cargar el contexto y no se repite aquí. Si lo perdiste —por ejemplo tras\n" +
	"> una compactación— vuelve a pedir el contexto del proyecto para recuperarlo."

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
	// log es opcional: sin él, el documento se entrega completo. Un proyecto
	// sin sesión activa no debe perder contexto por una optimización.
	log ports.DeliveryLog
}

func NewPlanContext(method string, context ports.ContextBuilder, log ports.DeliveryLog) *PlanContext {
	return &PlanContext{method: method, context: context, log: log}
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

	// Si el contexto general ya entregó este mismo historial en esta sesión, se
	// sustituye por el aviso: era la duplicación más cara medida en el proyecto,
	// unos 6.100 tokens cobrados dos veces. Si cambió, o si no consta entrega
	// previa, se entrega completo — la reducción nunca deja al agente sin
	// contexto (FR-009).
	if p.log != nil && context != "" {
		actual := HashDeContenido(context)
		if previo, ok := p.log.Last(ports.DeliveryContext); ok && previo == actual {
			context = avisoDeSupresion
		}
		p.log.Record(ports.DeliveryPlanContext, actual)
	}

	switch {
	case method == "":
		return context, nil
	case context == "":
		return method, nil
	default:
		return method + "\n\n---\n\n" + context, nil
	}
}
