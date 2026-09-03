package domain

import "strings"

// --- Resultado de una unidad delegada (feature 027) ---

// ResultStatus es el desenlace informado por el subagente.
type ResultStatus string

const (
	StatusCompleted           ResultStatus = "completed"
	StatusFailed              ResultStatus = "failed"
	StatusInsufficientContext ResultStatus = "insufficient_context"
)

// DelegatedResult es lo que vuelve al agente principal: estructurado y acotado.
// La transcripción completa del hijo NUNCA viaja aquí (INV-AAR-012) — no por
// higiene, sino porque inyectarla anularía el ahorro de contexto que justificaba
// delegar.
type DelegatedResult struct {
	TaskID          string
	Status          ResultStatus
	Summary         string
	Evidence        []string
	AffectedSymbols []string
	Artifacts       []string
	Unresolved      []string
	// Missing solo se puebla con estado insufficient_context: qué le faltó.
	Missing []string
}

// TokensAprox estima el tamaño del resultado con la misma heurística de orden de
// magnitud que el contador aproximado (~4 caracteres por token). Vive en el
// dominio porque compactar necesita una cota, y el dominio no puede llamar a
// ports.TokenCounter sin crear un ciclo de imports.
func (r DelegatedResult) TokensAprox() int {
	n := len(r.Summary)
	for _, lista := range [][]string{r.Evidence, r.AffectedSymbols, r.Artifacts, r.Unresolved, r.Missing} {
		for _, s := range lista {
			n += len(s) + 1
		}
	}
	return (n + 3) / 4
}

// Compactar reduce el resultado al presupuesto de integración.
//
// Qué se conserva y qué se descarta no es una preferencia estética: lo que
// sobrevive es lo que las tareas posteriores necesitan para continuar
// (conclusiones, evidencia, artefactos, pendientes), y lo que se recorta es lo
// que solo servía dentro de la conversación del hijo. Por eso las listas se
// preservan enteras y el recorte cae siempre sobre el resumen en prosa.
//
// Un presupuesto <= 0 significa "sin límite declarado": no se toca nada.
func (r DelegatedResult) Compactar(budgetTokens int) DelegatedResult {
	if budgetTokens <= 0 || r.TokensAprox() <= budgetTokens {
		return r
	}

	// Lo estructurado va primero y entero. El resumen se queda con lo que sobre.
	sinResumen := r
	sinResumen.Summary = ""
	restante := budgetTokens - sinResumen.TokensAprox()
	if restante <= 0 {
		r.Summary = ""
		return r
	}

	limite := restante * 4
	if len(r.Summary) > limite {
		r.Summary = recortarEnLimiteDePalabra(r.Summary, limite)
	}
	return r
}

// marcaDeRecorte señala que hubo material omitido. Ocupa sitio, así que su
// longitud se descuenta del límite ANTES de cortar: si no, el resultado
// "compactado" acabaría excediendo el presupuesto por unos pocos bytes — un
// desbordamiento pequeño sigue siendo un desbordamiento.
const marcaDeRecorte = " […]"

// recortarEnLimiteDePalabra corta en el último espacio que quepa, para no
// dejar una palabra partida a la mitad, dejando sitio para la marca.
func recortarEnLimiteDePalabra(s string, n int) string {
	if n <= len(marcaDeRecorte) {
		return ""
	}
	if len(s) <= n {
		return s
	}
	corte := s[:n-len(marcaDeRecorte)]
	if i := strings.LastIndex(corte, " "); i > 0 {
		corte = corte[:i]
	}
	return strings.TrimSpace(corte) + marcaDeRecorte
}

// --- Máquina de estados de una delegación (feature 027, Historia 7) ---
//
// Existe porque "reintentar hasta que salga" es la forma más cara de fallar: un
// ciclo de reintentos sobre una tarea que nunca va a completarse consume el
// presupuesto entero y deja al agente principal sin nada. Todos los caminos de
// aquí son ACOTADOS.

// FailurePolicy es lo que Octopus recomienda hacer tras un desenlace adverso.
type FailurePolicy string

const (
	// PolicyRetry: reintentar la delegación tal cual.
	PolicyRetry FailurePolicy = "RETRY"
	// PolicyExpandContext: reintentar UNA vez con contexto ampliado.
	PolicyExpandContext FailurePolicy = "EXPAND_CONTEXT"
	// PolicyFallbackInline: que lo haga el agente principal.
	PolicyFallbackInline FailurePolicy = "FALLBACK_INLINE"
	// PolicyAbort: ni reintentar ni asumirlo; se informa y se detiene.
	PolicyAbort FailurePolicy = "ABORT_TASK"
)

// AttemptState es lo consumido hasta ahora por una unidad delegada.
type AttemptState struct {
	Retries    int
	Expansions int
	// ParentCanDoIt indica si el agente principal puede asumir el trabajo. Lo
	// determina el llamador: el dominio no puede saberlo.
	ParentCanDoIt bool
}

// NextAfterFailure decide qué hacer tras un desenlace, dentro de los topes.
//
// El orden de las ramas es la política: primero se intenta lo barato y acotado
// (una ampliación de contexto, un reintento), y solo cuando se agotan se replica
// a inline o se aborta. Nunca hay una rama que devuelva RETRY sin haber mirado
// el contador.
func NextAfterFailure(status ResultStatus, st AttemptState, policy PolicyOverrides) FailurePolicy {
	maxRetries := policy.MaxRetriesEfectivo()

	switch status {
	case StatusInsufficientContext:
		// Una sola ampliación, y solo una (FR-042, AC-012). La segunda vez que
		// un hijo dice que le falta contexto, el problema no es el presupuesto:
		// es que la tarea no estaba bien acotada.
		if st.Expansions < DefaultMaxContextExpansions {
			return PolicyExpandContext
		}
		if st.ParentCanDoIt {
			return PolicyFallbackInline
		}
		return PolicyAbort

	case StatusFailed:
		if st.Retries < maxRetries {
			return PolicyRetry
		}
		if st.ParentCanDoIt {
			return PolicyFallbackInline
		}
		return PolicyAbort

	default:
		// Un estado completado no tiene siguiente paso adverso. Devolver Abort
		// aquí sería mentir; el llamador no debería preguntar.
		return PolicyAbort
	}
}

// ConservaResultadoParcial responde si el resultado parcial de una delegación
// fallida puede entregarse al padre (FR-043). Solo con contenido útil: un
// resultado vacío no aporta y ocuparía contexto sin dar nada a cambio.
func (r DelegatedResult) ConservaResultadoParcial() bool {
	return len(r.Evidence) > 0 || len(r.Artifacts) > 0 ||
		len(r.AffectedSymbols) > 0 || len(r.Unresolved) > 0 || len(r.Missing) > 0
}
