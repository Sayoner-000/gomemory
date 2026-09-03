package domain

// --- Rutas y razones (feature 027) ---

// Route es la estrategia de ejecución elegida para una unidad de trabajo.
type Route string

const (
	// RouteInline: el agente principal hace el trabajo.
	RouteInline Route = "INLINE"
	// RouteDelegate: el trabajo va a un subagente aislado.
	RouteDelegate Route = "DELEGATE"
	// RouteParallel: unidad delegada que además puede ejecutarse a la vez que
	// las otras de su grupo. No significa que Octopus ejecute nada en paralelo
	// — Octopus no ejecuta (INV-AAR-018); significa que el runtime puede.
	RouteParallel Route = "PARALLEL"
	// RouteWait: la unidad no debe ejecutarse todavía, faltan dependencias.
	RouteWait Route = "WAIT"
	// RouteReject: la delegación propuesta no debe ocurrir. NO significa que la
	// tarea no pueda completarse.
	RouteReject Route = "REJECT"
)

func AllRoutes() []Route {
	return []Route{RouteInline, RouteDelegate, RouteParallel, RouteWait, RouteReject}
}

// Delegada agrupa las rutas que implican un subagente. Las dos deben pasar por
// las validaciones de presupuesto, contrato y permisos.
func (r Route) Delegada() bool { return r == RouteDelegate || r == RouteParallel }

// Reason es el código de un catálogo CERRADO. Que sea cerrado es lo que hace
// explicable el enrutamiento sin exponer razonamiento privado (FR-007,
// INV-AAR-013): la razón nunca se compone concatenando texto libre, y por tanto
// no hay ningún resquicio por el que pueda escaparse contenido de contexto.
type Reason string

const (
	ReasonTrivial                    Reason = "trivial"
	ReasonNoSubagents                Reason = "no_subagents"
	ReasonUnresolvedDependency       Reason = "unresolved_dependency"
	ReasonContextNearlyFull          Reason = "context_nearly_full"
	ReasonOverheadExceedsBenefit     Reason = "overhead_exceeds_benefit"
	ReasonIsolatableInvestigation    Reason = "isolatable_investigation"
	ReasonBoundedInterface           Reason = "bounded_interface"
	ReasonParallelEligible           Reason = "parallel_eligible"
	ReasonBudgetExhausted            Reason = "budget_exhausted"
	ReasonValidationReserveProtected Reason = "validation_reserve_protected"
	ReasonFanOutLimit                Reason = "fan_out_limit"
	ReasonDepthLimit                 Reason = "depth_limit"
	ReasonDuplicateWork              Reason = "duplicate_work"
	ReasonPolicyForcedInline         Reason = "policy_forced_inline"
	ReasonHistoricalEvidence         Reason = "historical_evidence"
)

// reasonTexts es la ÚNICA fuente del texto de cada razón. Un código nuevo se
// añade aquí y en AllReasons, en ningún otro sitio.
var reasonTexts = map[Reason]string{
	ReasonTrivial:                    "el sobrecosto de delegar supera el beneficio esperado",
	ReasonNoSubagents:                "el runtime no declara soporte de subagentes",
	ReasonUnresolvedDependency:       "quedan dependencias directas sin resolver",
	ReasonContextNearlyFull:          "la tarea requiere casi todo el contexto del agente principal",
	ReasonOverheadExceedsBenefit:     "el costo estimado de delegar iguala o supera el de ejecutar inline",
	ReasonIsolatableInvestigation:    "investigación independiente con contexto fuertemente aislable",
	ReasonBoundedInterface:           "trabajo acotado sobre un contrato estable",
	ReasonParallelEligible:           "independiente de las demás y elegible para ejecución concurrente",
	ReasonBudgetExhausted:            "el presupuesto de delegación restante no cubre el costo estimado",
	ReasonValidationReserveProtected: "los tokens restantes pertenecen a la reserva de validación",
	ReasonFanOutLimit:                "se alcanzó el tope de agentes delegados del plan",
	ReasonDepthLimit:                 "se alcanzó la profundidad máxima de delegación",
	ReasonDuplicateWork:              "trabajo equivalente ya completado, en curso o cubierto por el contexto del padre",
	ReasonPolicyForcedInline:         "la política del llamador exige ejecución inline",
	ReasonHistoricalEvidence:         "la evidencia histórica del patrón favorece la delegación",
}

// Text devuelve el texto de la razón, o cadena vacía si el código no pertenece
// al catálogo. No inventa texto: una razón desconocida se detecta, no se disimula.
func (r Reason) Text() string { return reasonTexts[r] }

// AllReasons enumera el catálogo en orden estable.
func AllReasons() []Reason {
	return []Reason{
		ReasonTrivial, ReasonNoSubagents, ReasonUnresolvedDependency,
		ReasonContextNearlyFull, ReasonOverheadExceedsBenefit,
		ReasonIsolatableInvestigation, ReasonBoundedInterface,
		ReasonParallelEligible, ReasonBudgetExhausted,
		ReasonValidationReserveProtected, ReasonFanOutLimit, ReasonDepthLimit,
		ReasonDuplicateWork, ReasonPolicyForcedInline, ReasonHistoricalEvidence,
	}
}

// RouteDecision es la decisión de enrutamiento de UNA unidad de trabajo.
type RouteDecision struct {
	WorkUnitID    string
	Route         Route
	Reason        Reason
	ContextBudget int
	OutputBudget  int
	ParallelGroup string
	EstimatedCost CostEstimate
	// Estimated marca que las cifras provienen de estimaciones y no de un
	// reporte real del runtime. Nunca se presentan como exactas (FR-033).
	Estimated bool
	// BlockedBy solo se puebla con ruta WAIT: las dependencias que faltan.
	BlockedBy []string
}

// ParallelGroup es un conjunto de unidades que pueden ejecutarse a la vez.
type ParallelGroup struct {
	ID    string
	Tasks []string
}

// RoutingPlan es el conjunto de decisiones de un grafo de tareas. Es ASESOR y
// revisable, no inmutable (FR-020): reevaluarlo preserva el trabajo completado.
type RoutingPlan struct {
	PlanID         string
	Decisions      []RouteDecision
	ParallelGroups []ParallelGroup
	Budget         Budget
	DelegatedCount int
}

// Decision busca la decisión de una unidad. Devuelve nil si no está — nil es
// "no encontrado", nunca un error (convención del proyecto).
func (p RoutingPlan) Decision(workUnitID string) *RouteDecision {
	for i := range p.Decisions {
		if p.Decisions[i].WorkUnitID == workUnitID {
			return &p.Decisions[i]
		}
	}
	return nil
}
