package domain

import (
	"fmt"
	"sort"
)

// --- Enrutamiento de un grafo de tareas (feature 027) ---
//
// Función pura, como RouteTask. La diferencia es que aquí hay colecciones, y con
// colecciones aparece el riesgo de no-determinismo: el orden de iteración de un
// mapa en Go es aleatorio A PROPÓSITO. Un plan construido recorriendo un mapa
// saldría con los grupos en orden distinto en cada corrida — no rompería la
// corrección, pero sí SC-006 y la confianza en la simulación, y lo haría de
// forma intermitente, que es la peor manera de fallar.
//
// Por eso TODO recorrido que produzca salida va sobre slices ordenados por
// identificador. Los mapas solo se usan para consultas, nunca para iterar.

// PlanInput es la entrada de RoutePlan.
type PlanInput struct {
	PlanID        string
	Units         []WorkUnit
	Resolved      map[string]bool
	Capabilities  RuntimeCapabilities
	Budget        Budget
	Policy        PolicyOverrides
	Evidence      map[TaskClass]*ClassEvidence
	Depth         int
	DuplicateWork map[string]bool
}

// RoutePlan enruta un grafo de tareas completo.
//
// Devuelve error SOLO por entrada inválida: ciclo, dependencia inexistente,
// identificador duplicado o unidad mal formada. La falta de presupuesto, de
// capacidades o de historial NO es error — son decisiones, y decidir con
// restricciones es justamente el trabajo de este paquete.
func RoutePlan(in PlanInput) (RoutingPlan, error) {
	unidades, err := validarGrafo(in.Units)
	if err != nil {
		return RoutingPlan{}, err
	}

	caps := in.Capabilities.Normalize()
	presupuesto := in.Budget
	plan := RoutingPlan{PlanID: in.PlanID, Budget: presupuesto}

	// Primera pasada: decidir cada unidad en orden de identificador. El orden
	// importa porque el tope de agentes se consume secuencialmente: con orden
	// aleatorio, qué tarea se queda sin cupo cambiaría en cada corrida.
	for _, u := range unidades {
		decision := RouteTask(RouteInput{
			Unit:             u,
			Resolved:         in.Resolved,
			Capabilities:     caps,
			Budget:           presupuesto,
			Policy:           in.Policy,
			Evidence:         in.Evidence[u.Class],
			Depth:            in.Depth,
			DuplicateWork:    in.DuplicateWork[u.ID],
			DelegatedSoFar:   plan.DelegatedCount,
			InlineCostTokens: u.InlineCostTokens,
			ContractTokens:   u.ContractTokens,
		})

		if decision.Route.Delegada() {
			plan.DelegatedCount++
			presupuesto = presupuesto.Gastar(decision.EstimatedCost.Total())
		}
		plan.Decisions = append(plan.Decisions, decision)
	}

	plan.Budget = presupuesto
	plan.ParallelGroups = formarGruposParalelos(&plan, unidades, caps, in.Policy)
	return plan, nil
}

// validarGrafo comprueba en el borde y devuelve las unidades ORDENADAS por
// identificador. Ordenar aquí, una sola vez, es lo que hace determinista todo lo
// que viene después.
func validarGrafo(units []WorkUnit) ([]WorkUnit, error) {
	if len(units) == 0 {
		return nil, fmt.Errorf("%w: el plan no tiene unidades de trabajo", ErrValidation)
	}

	porID := make(map[string]WorkUnit, len(units))
	for _, u := range units {
		if err := u.Validate(); err != nil {
			return nil, err
		}
		if _, repetida := porID[u.ID]; repetida {
			return nil, fmt.Errorf("%w: identificador de tarea duplicado: %s", ErrValidation, u.ID)
		}
		porID[u.ID] = u
	}

	ordenadas := append([]WorkUnit(nil), units...)
	sort.Slice(ordenadas, func(i, j int) bool { return ordenadas[i].ID < ordenadas[j].ID })

	for _, u := range ordenadas {
		for _, dep := range u.Dependencies {
			if _, existe := porID[dep]; !existe {
				return nil, fmt.Errorf("%w: la tarea %s depende de %s, que no está en el plan", ErrValidation, u.ID, dep)
			}
		}
	}

	if ciclo := buscarCiclo(ordenadas, porID); ciclo != "" {
		return nil, fmt.Errorf("%w: el grafo de tareas tiene un ciclo en %s", ErrValidation, ciclo)
	}
	return ordenadas, nil
}

// buscarCiclo recorre en profundidad y devuelve el identificador donde se cierra
// el ciclo, o cadena vacía. Se detecta ANTES de decidir: un ciclo enrutado sin
// detectar produciría un plan donde todo espera para siempre y nada lo explica.
func buscarCiclo(ordenadas []WorkUnit, porID map[string]WorkUnit) string {
	const (
		sinVisitar = 0
		enCurso    = 1
		terminado  = 2
	)
	estado := make(map[string]int, len(ordenadas))

	var visitar func(id string) string
	visitar = func(id string) string {
		switch estado[id] {
		case enCurso:
			return id
		case terminado:
			return ""
		}
		estado[id] = enCurso
		for _, dep := range porID[id].Dependencies {
			if ciclo := visitar(dep); ciclo != "" {
				return ciclo
			}
		}
		estado[id] = terminado
		return ""
	}

	for _, u := range ordenadas {
		if ciclo := visitar(u.ID); ciclo != "" {
			return ciclo
		}
	}
	return ""
}

// formarGruposParalelos agrupa las unidades delegadas que pueden ejecutarse a la
// vez, y promueve su ruta a PARALLEL. Recorre `unidades`, que ya viene ordenado.
func formarGruposParalelos(plan *RoutingPlan, unidades []WorkUnit, caps RuntimeCapabilities, policy PolicyOverrides) []ParallelGroup {
	if !caps.Parallel {
		return nil
	}
	tope := caps.ConcurrenciaEfectiva(policy.MaxParallelEfectivo())
	if tope < 2 {
		return nil
	}

	porID := make(map[string]WorkUnit, len(unidades))
	for _, u := range unidades {
		porID[u.ID] = u
	}

	var candidatas []WorkUnit
	for _, u := range unidades {
		if d := plan.Decision(u.ID); d != nil && d.Route == RouteDelegate {
			candidatas = append(candidatas, u)
		}
	}
	if len(candidatas) < 2 {
		return nil
	}

	var grupos []ParallelGroup
	for _, u := range candidatas {
		colocada := false
		for i := range grupos {
			if len(grupos[i].Tasks) >= tope {
				continue
			}
			if compatibleConGrupo(u, grupos[i], porID) {
				grupos[i].Tasks = append(grupos[i].Tasks, u.ID)
				colocada = true
				break
			}
		}
		if !colocada {
			grupos = append(grupos, ParallelGroup{
				ID:    fmt.Sprintf("P%d", len(grupos)+1),
				Tasks: []string{u.ID},
			})
		}
	}

	// Un grupo de uno no es paralelismo: esa unidad conserva DELEGATE. Promover
	// a PARALLEL algo que corre solo mentiría al runtime sobre la forma del plan.
	var reales []ParallelGroup
	for _, g := range grupos {
		if len(g.Tasks) < 2 {
			continue
		}
		reales = append(reales, g)
		for _, id := range g.Tasks {
			if d := plan.Decision(id); d != nil {
				d.Route = RouteParallel
				d.Reason = ReasonParallelEligible
				d.ParallelGroup = g.ID
			}
		}
	}
	return reales
}

// compatibleConGrupo responde si una unidad puede convivir con todos los
// miembros de un grupo: sin dependencia en ninguna dirección (directa o
// transitiva) y sin escrituras en conflicto.
func compatibleConGrupo(u WorkUnit, g ParallelGroup, porID map[string]WorkUnit) bool {
	for _, id := range g.Tasks {
		otra := porID[id]
		if dependeDe(u, otra.ID, porID) || dependeDe(otra, u.ID, porID) {
			return false
		}
		if u.Scope.WritesIntersect(otra.Scope) {
			return false
		}
	}
	return true
}

// dependeDe responde si `u` depende de `objetivo`, directa o transitivamente.
func dependeDe(u WorkUnit, objetivo string, porID map[string]WorkUnit) bool {
	visitados := map[string]bool{}

	var buscar func(id string) bool
	buscar = func(id string) bool {
		if visitados[id] {
			return false
		}
		visitados[id] = true
		for _, dep := range porID[id].Dependencies {
			if dep == objetivo || buscar(dep) {
				return true
			}
		}
		return false
	}
	return buscar(u.ID)
}
