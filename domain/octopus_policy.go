package domain

// --- Política de enrutamiento (feature 027) ---
//
// Este archivo es el corazón de Octopus. Todo lo que hay aquí es una función
// pura: sin I/O, sin reloj, sin aleatoriedad y sin recorrer mapas para producir
// salida ordenada. Esas cuatro prohibiciones no son estilo — son lo que hace
// que la decisión sea reproducible (SC-006) y auditable sin ejecutar un modelo.

// Topes de fábrica. Se declaran UNA sola vez, aquí, y se sobrescriben desde la
// configuración del proyecto o desde la política del llamador. La constitución
// prohíbe repetir valores de configuración en los sitios de uso.
const (
	// DefaultMaxDelegationDepth = 1 significa: agente principal → subagente, y
	// se acabó. El hijo no crea otro hijo (INV-AAR-009). La delegación
	// ilimitada no se admite en ningún caso.
	DefaultMaxDelegationDepth = 1
	// DefaultMaxDelegationRetries acota los reintentos automáticos de una
	// delegación fallida (INV-AAR-011).
	DefaultMaxDelegationRetries = 1
	// DefaultMaxSubagentsPerPlan impide que un plan de 50 tareas produzca 50
	// agentes (INV-AAR-010). Cantidad de tareas y cantidad de agentes son
	// conceptos independientes.
	DefaultMaxSubagentsPerPlan = 4
	// DefaultMaxParallel es el tope propio de Octopus. Frente al del runtime
	// gana siempre el más restrictivo (INV-AAR-008).
	DefaultMaxParallel = 3
	// DefaultMaxContextExpansions acota la ampliación de contexto tras un
	// INSUFFICIENT_CONTEXT (FR-042).
	DefaultMaxContextExpansions = 1
)

// Sobrecostos de fábrica, en tokens, de delegar una unidad. Son estimaciones
// deliberadamente groseras: la especificación exige operar sin conteo exacto
// (INV-AAR-016) y afinarlas es justo lo que hace la evidencia histórica después.
const (
	// CoordinationOverheadTokens: describir la tarea, arrancar el agente y
	// recoger su respuesta.
	CoordinationOverheadTokens = 400
	// IntegrationOverheadTokens: incorporar el resultado al hilo del padre.
	IntegrationOverheadTokens = 250
	// NoIsolationPenaltyTokens: recargo cuando el runtime declara que NO aísla
	// contexto. No prohíbe delegar; encarece la delegación, que es lo que la
	// especificación pide (FR-036).
	NoIsolationPenaltyTokens = 600
	// Risk y ruta crítica no cambian la pureza ni crean reglas ocultas: elevan
	// el costo de coordinación/integración que la política ya compara.
	HighRiskCoordinationTokens    = 300
	MediumRiskCoordinationTokens  = 100
	CriticalPathIntegrationTokens = 200
	// MinDelegableContextTokens: por debajo de este contexto la tarea es
	// demasiado pequeña para que delegar compense el arranque.
	MinDelegableContextTokens = 500
)

// PolicyOverrides son las anulaciones explícitas del llamador (FR-050). Los
// topes en 0 significan "sin opinión": se usa el valor de fábrica.
type PolicyOverrides struct {
	DelegationDisabled bool
	DelegationForced   bool
	MaxSubagents       int
	MaxParallel        int
	MaxDepth           int
	MaxRetries         int
	PreferInline       bool
	TokenBudget        int
	// AllowValidationReserve autoriza de forma explícita a consumir la reserva
	// de validación. Sin esta bandera, la reserva es intocable (INV-AAR-006).
	AllowValidationReserve bool
}

// menorPositivo devuelve el más restrictivo de dos topes, tratando 0 y los
// negativos como "sin opinión". Es la regla de resolución de todos los topes de
// Octopus: entre llamador, configuración y runtime, gana el más restrictivo.
func menorPositivo(a, b int) int {
	switch {
	case a <= 0:
		return b
	case b <= 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}

func (p PolicyOverrides) MaxDepthEfectiva() int {
	return menorPositivo(p.MaxDepth, DefaultMaxDelegationDepth)
}

func (p PolicyOverrides) MaxSubagentsEfectivo() int {
	return menorPositivo(p.MaxSubagents, DefaultMaxSubagentsPerPlan)
}

func (p PolicyOverrides) MaxParallelEfectivo() int {
	return menorPositivo(p.MaxParallel, DefaultMaxParallel)
}

func (p PolicyOverrides) MaxRetriesEfectivo() int {
	return menorPositivo(p.MaxRetries, DefaultMaxDelegationRetries)
}

// MaxBoundedScopeFiles es cuántos artefactos puede tocar una unidad y seguir
// considerándose de alcance acotado. Por encima, el contexto necesario deja de
// ser aislable y delegar empieza a duplicar el contexto del padre.
const MaxBoundedScopeFiles = 5

// DefaultOutputBudgetTokens es el tope de salida que se asigna a una unidad
// delegada que no declaró el suyo. Existe para que ninguna delegación salga sin
// contrato de salida (FR-026).
const DefaultOutputBudgetTokens = 1000

// Umbrales a partir de los cuales la evidencia histórica deja de ser ruido.
const (
	MinEvidenceExecutions  = 5
	MinEvidenceSuccessRate = 0.80
)

// ClassEvidence es la evidencia histórica AGREGADA de un patrón de tarea. Nunca
// contiene contenido, transcripciones ni razonamiento: solo cifras (INV-AAR-013).
type ClassEvidence struct {
	Class                     TaskClass
	Executions                int
	InlineAvgTokens           int
	DelegatedAvgTokens        int
	DelegatedAvgContextTokens int
	SuccessRate               float64
}

// Favorece indica si la evidencia respalda delegar este patrón. Es ASESORA: solo
// interviene en el desempate de la regla 12 y jamás salta las reglas 1 a 11
// (FR-049). El receptor nil es válido a propósito — "sin historial" es el caso
// normal el primer día, no una anomalía.
func (e *ClassEvidence) Favorece() bool {
	if e == nil || e.Executions < MinEvidenceExecutions {
		return false
	}
	if e.SuccessRate < MinEvidenceSuccessRate {
		return false
	}
	return e.DelegatedAvgTokens > 0 && e.InlineAvgTokens > 0 &&
		e.DelegatedAvgTokens < e.InlineAvgTokens
}

// RouteInput es todo lo que la política necesita para decidir. Repara en lo que
// NO hay aquí: ningún puerto, ningún contador de tokens, ninguna conexión. Las
// cifras llegan ya medidas por el caso de uso.
type RouteInput struct {
	Unit         WorkUnit
	Resolved     map[string]bool
	Capabilities RuntimeCapabilities
	Budget       Budget
	Policy       PolicyOverrides
	Evidence     *ClassEvidence
	// Depth es la profundidad de delegación actual: 0 = agente principal.
	Depth int
	// DuplicateWork lo determina el caso de uso consultando memoria y trabajo en
	// curso (FR-013). El dominio no consulta nada.
	DuplicateWork bool
	// DelegatedSoFar son los agentes ya comprometidos en este plan. Lo puebla
	// RoutePlan; en una decisión suelta vale 0.
	DelegatedSoFar int
	// InlineCostTokens es el costo estimado de hacer el trabajo inline, medido
	// por el caso de uso. 0 = desconocido, y entonces la regla 8 no aplica.
	InlineCostTokens int
	// ContractTokens es el tamaño medido del contrato de ejecución.
	ContractTokens int
}

// pendientes devuelve las dependencias sin resolver EN EL ORDEN DECLARADO. El
// orden viene del slice Dependencies, nunca de recorrer un mapa: el orden de
// iteración de un mapa en Go es aleatorio a propósito y produciría una lista
// distinta en cada corrida, rompiendo SC-006 de forma intermitente.
func (in RouteInput) pendientes() []string {
	var faltan []string
	for _, dep := range in.Unit.Dependencies {
		if !in.Resolved[dep] {
			faltan = append(faltan, dep)
		}
	}
	return faltan
}

// EstimateDelegationCost desglosa el costo estimado de delegar la unidad. Es
// aritmética sobre cifras ya medidas y constantes con nombre — el dominio no
// mide texto.
func (in RouteInput) EstimateDelegationCost(caps RuntimeCapabilities) CostEstimate {
	salida := in.Unit.ExpectedOutput.MaxTokens
	if salida <= 0 {
		salida = DefaultOutputBudgetTokens
	}
	c := CostEstimate{
		ContextTokens:      in.Unit.ContextNeed.EstimatedTokens,
		ContractTokens:     in.ContractTokens,
		OutputTokens:       salida,
		CoordinationTokens: CoordinationOverheadTokens,
		IntegrationTokens:  IntegrationOverheadTokens,
	}
	// Un runtime que no aísla contexto obliga al padre a cargar con parte del
	// contexto del hijo: delegar sigue siendo posible, pero cuesta más (FR-036).
	if !caps.IsolatedContext {
		c.CoordinationTokens += NoIsolationPenaltyTokens
	}
	switch in.Unit.Risk {
	case LevelHigh:
		c.CoordinationTokens += HighRiskCoordinationTokens
	case LevelMedium:
		c.CoordinationTokens += MediumRiskCoordinationTokens
	}
	if in.Unit.CriticalPath {
		c.IntegrationTokens += CriticalPathIntegrationTokens
	}
	return c
}

// RouteTask decide la ruta de UNA unidad de trabajo.
//
// Función pura: sin I/O, sin reloj, sin aleatoriedad. Las reglas se evalúan en
// el orden fijado por contracts/routing-policy.md y la primera que aplica gana.
// El orden es parte del contrato porque hace la decisión predecible y auditable:
// mover una regla cambia el comportamiento observable, y por eso hay una tabla
// de casos con una fila por regla.
func RouteTask(in RouteInput) RouteDecision {
	caps := in.Capabilities.Normalize()
	costo := in.EstimateDelegationCost(caps)

	d := RouteDecision{
		WorkUnitID:    in.Unit.ID,
		EstimatedCost: costo,
		Estimated:     true,
	}
	inline := func(r Reason) RouteDecision {
		d.Route = RouteInline
		d.Reason = r
		return d
	}

	// 1. La voluntad explícita del llamador manda sobre cualquier heurística.
	if in.Policy.DelegationDisabled {
		return inline(ReasonPolicyForcedInline)
	}

	// 2. Antes que nada: ¿se puede ejecutar siquiera? Va por delante incluso de
	//    las capacidades, porque esperar es cierto con y sin subagentes.
	if faltan := in.pendientes(); len(faltan) > 0 {
		d.Route = RouteWait
		d.Reason = ReasonUnresolvedDependency
		d.BlockedBy = faltan
		return d
	}

	// 3. Sin subagentes no hay delegación posible, ni siquiera forzada (AC-003).
	if !caps.Subagents {
		return inline(ReasonNoSubagents)
	}

	// 4. La recursión está acotada: un hijo no engendra otro hijo.
	if in.Depth >= in.Policy.MaxDepthEfectiva() {
		return inline(ReasonDepthLimit)
	}

	// 5. Reutilizar trabajo equivalente siempre gana a rehacerlo delegando.
	if in.DuplicateWork {
		return inline(ReasonDuplicateWork)
	}

	// 6. Lo trivial no amortiza el arranque de un agente.
	if in.Unit.Complexity == LevelTrivial {
		return inline(ReasonTrivial)
	}

	// 7. Si necesita casi todo el contexto del padre, delegarlo lo duplica en vez
	//    de aislarlo: el aislamiento era justamente el beneficio buscado.
	if in.Unit.ContextNeed.NearlyFullParent {
		return inline(ReasonContextNearlyFull)
	}

	// 8. La comparación honesta: delegar contra hacerlo aquí mismo.
	if in.InlineCostTokens > 0 && costo.Total() >= in.InlineCostTokens {
		return inline(ReasonOverheadExceedsBenefit)
	}

	// 9 y 10. Presión de presupuesto. El orden entre las dos importa: si el fondo
	// está agotado PERO la reserva de validación tiene tokens y la unidad es
	// prescindible, la razón precisa no es "no hay presupuesto" sino "lo que
	// queda es la reserva y no se toca" — le dice al usuario algo distinto.
	if in.Budget.Declarado() && !in.Budget.Cabe(costo.Total()) {
		conReserva := in.Budget.DelegationRemaining() + in.Budget.ValidationReserve
		if in.Policy.AllowValidationReserve && costo.Total() <= conReserva {
			// Autorización explícita: se permite tirar de la reserva.
		} else if in.Unit.Optional && in.Budget.DelegationRemaining() == 0 && in.Budget.ValidationReserve > 0 {
			return inline(ReasonValidationReserveProtected)
		} else if in.Policy.DelegationForced {
			// El llamador declaró que esta unidad EXIGE delegación. La respuesta
			// honesta es que esa delegación no debe ocurrir, no "la hago yo".
			d.Route = RouteReject
			d.Reason = ReasonBudgetExhausted
			return d
		} else {
			return inline(ReasonBudgetExhausted)
		}
	}

	// 11. Cantidad de tareas y cantidad de agentes son conceptos independientes.
	if max := in.Policy.MaxSubagentsEfectivo(); in.DelegatedSoFar >= max {
		return inline(ReasonFanOutLimit)
	}

	// 12. El desempate: ¿el beneficio esperado justifica el sobrecosto?
	if razon, ok := in.razonParaDelegar(caps); ok {
		d.Route = RouteDelegate
		d.Reason = razon
		d.ContextBudget = in.presupuestoDeContexto()
		d.OutputBudget = costo.OutputTokens
		return d
	}

	// 13. Por defecto se queda en casa. Delegar es la excepción justificada, no
	//     el comportamiento por omisión.
	return inline(ReasonOverheadExceedsBenefit)
}

// presupuestoDeContexto es el MENOR presupuesto con el que la unidad puede
// completarse, no el que quepa en el fondo (FR-025). Que sobre presupuesto
// global no es razón para ampliar el contexto de una tarea pequeña.
func (in RouteInput) presupuestoDeContexto() int {
	n := in.Unit.ContextNeed.EstimatedTokens
	if n < MinDelegableContextTokens {
		return MinDelegableContextTokens
	}
	return n
}

// razonParaDelegar es el desempate de la regla 12. Devuelve la razón concreta,
// del catálogo cerrado, o false si nada justifica delegar.
func (in RouteInput) razonParaDelegar(caps RuntimeCapabilities) (Reason, bool) {
	// Una unidad con contexto minúsculo no amortiza el arranque del agente,
	// por mucho que su clase parezca aislable.
	// Cero significa "no se midió", no "contexto diminuto". El presupuesto de
	// contexto ya tiene un mínimo seguro; convertir esa ausencia en un veto hizo
	// inerte a RoutePlan y MCP cuando el runtime no inyectaba context_tokens.
	if in.Unit.ContextNeed.EstimatedTokens > 0 && in.Unit.ContextNeed.EstimatedTokens < MinDelegableContextTokens && !in.Policy.DelegationForced {
		return "", false
	}

	aislable := in.Unit.Class.Aislable()
	acotada := len(in.Unit.Scope.Files) > 0 && len(in.Unit.Scope.Files) <= MaxBoundedScopeFiles

	// PreferInline no prohíbe delegar: sube el listón a lo inequívocamente
	// aislable, que es lo que pide quien marca esa preferencia.
	if in.Policy.PreferInline && !in.Policy.DelegationForced {
		if aislable && acotada && caps.IsolatedContext {
			return ReasonIsolatableInvestigation, true
		}
		return "", false
	}

	switch {
	case aislable:
		return ReasonIsolatableInvestigation, true
	case acotada && in.Unit.Complexity >= LevelLow:
		return ReasonBoundedInterface, true
	case in.Evidence.Favorece():
		return ReasonHistoricalEvidence, true
	case in.Policy.DelegationForced:
		return ReasonBoundedInterface, true
	default:
		return "", false
	}
}
