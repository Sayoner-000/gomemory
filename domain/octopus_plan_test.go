package domain

import (
	"encoding/json"
	"errors"
	"testing"
)

func planDePrueba(unidades ...WorkUnit) PlanInput {
	return PlanInput{
		PlanID:       "plan-001",
		Units:        unidades,
		Capabilities: capacidadesPlenas(),
		Budget:       presupuestoAmplio(),
	}
}

func unidadAislable(id string, deps ...string) WorkUnit {
	return WorkUnit{
		ID: id, Objective: "objetivo de " + id,
		Class: ClassInvestigation, Dependencies: deps,
		Scope:       Scope{Files: []string{id + ".go"}, ReadOnly: true},
		Complexity:  LevelMedium,
		ContextNeed: ContextNeed{EstimatedTokens: 2000},
	}
}

// T038: la entrada inválida se rechaza en el BORDE, antes de decidir nada. Un
// ciclo enrutado sin detectar produciría un plan donde todo espera para siempre
// y nada lo explica.
func TestRoutePlan_EntradaInvalida(t *testing.T) {
	casos := []struct {
		nombre string
		in     PlanInput
	}{
		{
			"ciclo directo",
			planDePrueba(unidadAislable("T001", "T002"), unidadAislable("T002", "T001")),
		},
		{
			"ciclo indirecto",
			planDePrueba(unidadAislable("T001", "T003"), unidadAislable("T002", "T001"), unidadAislable("T003", "T002")),
		},
		{
			"dependencia inexistente",
			planDePrueba(unidadAislable("T001", "T999")),
		},
		{
			"identificador duplicado",
			planDePrueba(unidadAislable("T001"), unidadAislable("T001")),
		},
		{
			"unidad sin objetivo",
			planDePrueba(WorkUnit{ID: "T001"}),
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := RoutePlan(c.in)
			if err == nil {
				t.Fatal("se esperaba error de entrada inválida")
			}
			if !errors.Is(err, ErrValidation) {
				t.Errorf("el error debería envolver ErrValidation: %v", err)
			}
		})
	}
}

// La falta de presupuesto o de capacidades NO es un error: es una decisión.
func TestRoutePlan_SinPresupuestoNiCapacidadesNoEsError(t *testing.T) {
	in := planDePrueba(unidadAislable("T001"), unidadAislable("T002"))
	in.Budget = Budget{}
	in.Capabilities = RuntimeCapabilities{}

	plan, err := RoutePlan(in)
	if err != nil {
		t.Fatalf("no debería ser error: %v", err)
	}
	for _, d := range plan.Decisions {
		if d.Route.Delegada() {
			t.Errorf("%s: sin capacidades no puede delegarse", d.WorkUnitID)
		}
	}
}

// T039 — AC-005: una tarea nunca comparte grupo paralelo con su dependencia.
func TestRoutePlan_AC005_DependenciaNoComparteGrupo(t *testing.T) {
	plan, err := RoutePlan(planDePrueba(
		unidadAislable("T001"),
		unidadAislable("T002", "T001"),
	))
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}

	d2 := plan.Decision("T002")
	if d2 == nil {
		t.Fatal("falta la decisión de T002")
	}
	if d2.Route != RouteWait {
		t.Errorf("T002 depende de T001 sin resolver: ruta = %q, esperaba WAIT", d2.Route)
	}
	for _, g := range plan.ParallelGroups {
		var tieneT001, tieneT002 bool
		for _, id := range g.Tasks {
			tieneT001 = tieneT001 || id == "T001"
			tieneT002 = tieneT002 || id == "T002"
		}
		if tieneT001 && tieneT002 {
			t.Errorf("el grupo %s junta una tarea con su dependencia: %v", g.ID, g.Tasks)
		}
	}
}

// T040 — AC-004: dos independientes pueden compartir grupo.
func TestRoutePlan_AC004_IndependientesEnParalelo(t *testing.T) {
	plan, err := RoutePlan(planDePrueba(unidadAislable("T003"), unidadAislable("T004")))
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}

	if len(plan.ParallelGroups) != 1 {
		t.Fatalf("esperaba 1 grupo paralelo, hay %d", len(plan.ParallelGroups))
	}
	if len(plan.ParallelGroups[0].Tasks) != 2 {
		t.Errorf("el grupo debería tener las 2 tareas: %v", plan.ParallelGroups[0].Tasks)
	}
	for _, id := range []string{"T003", "T004"} {
		d := plan.Decision(id)
		if d.Route != RouteParallel {
			t.Errorf("%s: ruta = %q, esperaba PARALLEL", id, d.Route)
		}
		if d.ParallelGroup == "" {
			t.Errorf("%s: falta el identificador de grupo", id)
		}
	}
}

// El tope de concurrencia se respeta y gana el más restrictivo (INV-AAR-008).
func TestRoutePlan_TopeDeConcurrencia(t *testing.T) {
	casos := []struct {
		nombre      string
		maxRuntime  int
		maxPolitica int
		wantMax     int
	}{
		{"runtime 2", 2, 0, 2},
		{"política 1", 8, 1, 1},
		{"gana el más restrictivo", 2, 3, 2},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			in := planDePrueba(
				unidadAislable("T001"), unidadAislable("T002"),
				unidadAislable("T003"), unidadAislable("T004"),
			)
			in.Capabilities.MaxParallel = c.maxRuntime
			in.Policy.MaxParallel = c.maxPolitica
			in.Policy.MaxSubagents = 4

			plan, err := RoutePlan(in)
			if err != nil {
				t.Fatalf("RoutePlan: %v", err)
			}
			for _, g := range plan.ParallelGroups {
				if len(g.Tasks) > c.wantMax {
					t.Errorf("el grupo %s tiene %d tareas, el tope efectivo es %d", g.ID, len(g.Tasks), c.wantMax)
				}
			}
		})
	}
}

// Sin paralelismo declarado no se forma ningún grupo, pero sí puede delegarse.
func TestRoutePlan_SinParalelismoDelegaSinAgrupar(t *testing.T) {
	in := planDePrueba(unidadAislable("T001"), unidadAislable("T002"))
	in.Capabilities = RuntimeCapabilities{Subagents: true, IsolatedContext: true}

	plan, err := RoutePlan(in)
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}
	if len(plan.ParallelGroups) != 0 {
		t.Errorf("sin paralelismo no debe formarse ningún grupo: %v", plan.ParallelGroups)
	}
	if plan.DelegatedCount == 0 {
		t.Error("sin paralelismo la delegación sigue siendo posible")
	}
}

// T041 — AC-009: 20 tareas independientes con tope de 4 no producen 20 agentes.
func TestRoutePlan_AC009_TopeDeAgentes(t *testing.T) {
	var unidades []WorkUnit
	for i := 1; i <= 20; i++ {
		unidades = append(unidades, unidadAislable(idTarea(i)))
	}
	in := planDePrueba(unidades...)
	in.Policy.MaxSubagents = 4
	// Presupuesto deliberadamente holgado: el tope de agentes debe ser la ÚNICA
	// restricción que ate. Con el presupuesto de referencia, la regla 9
	// (fondo agotado) dispararía antes que la 11 y la prueba mediría otra cosa.
	in.Budget = NewBudget(600000, DefaultBudgetSplit())

	plan, err := RoutePlan(in)
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}

	if plan.DelegatedCount > 4 {
		t.Errorf("DelegatedCount = %d, el tope es 4", plan.DelegatedCount)
	}
	var delegadas, conFanOut int
	for _, d := range plan.Decisions {
		if d.Route.Delegada() {
			delegadas++
		}
		if d.Reason == ReasonFanOutLimit {
			conFanOut++
		}
	}
	if delegadas > 4 {
		t.Errorf("%d rutas delegadas, el tope es 4", delegadas)
	}
	if conFanOut == 0 {
		t.Error("las tareas que no caben deberían explicarse con ReasonFanOutLimit")
	}
	if len(plan.Decisions) != 20 {
		t.Errorf("las 20 tareas deben recibir decisión, hay %d", len(plan.Decisions))
	}
}

// T042 — AC-010: con profundidad máxima 1, el hijo no queda autorizado a delegar.
func TestRoutePlan_AC010_ProfundidadMaxima(t *testing.T) {
	in := planDePrueba(unidadAislable("T001"))
	in.Depth = 1

	plan, err := RoutePlan(in)
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}
	d := plan.Decision("T001")
	if d.Route.Delegada() {
		t.Errorf("a profundidad 1 con tope 1 no puede delegarse: %q", d.Route)
	}
	if d.Reason != ReasonDepthLimit {
		t.Errorf("razón = %q, esperaba %q", d.Reason, ReasonDepthLimit)
	}
}

// T043: dos unidades que escriben el mismo artefacto no comparten grupo aunque
// no exista dependencia declarada. El conflicto es de estado, no de orden.
func TestRoutePlan_EscriturasEnConflictoNoComparteGrupo(t *testing.T) {
	a := unidadAislable("T001")
	a.Scope = Scope{Files: []string{"store.go"}}
	a.Class = ClassImplementation
	b := unidadAislable("T002")
	b.Scope = Scope{Files: []string{"store.go"}}
	b.Class = ClassImplementation

	plan, err := RoutePlan(planDePrueba(a, b))
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}

	for _, g := range plan.ParallelGroups {
		if len(g.Tasks) > 1 {
			t.Errorf("el grupo %s junta dos escrituras del mismo artefacto: %v", g.ID, g.Tasks)
		}
	}
}

// T044 — SC-006: el plan es reproducible byte a byte, y sus colecciones salen
// ordenadas por identificador, nunca por recorrido de mapa.
func TestRoutePlan_OrdenDeterministaYReproducible(t *testing.T) {
	var unidades []WorkUnit
	for i := 10; i >= 1; i-- { // deliberadamente en orden inverso
		unidades = append(unidades, unidadAislable(idTarea(i)))
	}
	in := planDePrueba(unidades...)

	primero, err := RoutePlan(in)
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}
	for i := 1; i < len(primero.Decisions); i++ {
		if primero.Decisions[i-1].WorkUnitID >= primero.Decisions[i].WorkUnitID {
			t.Fatalf("las decisiones deben salir ordenadas por identificador: %v > %v",
				primero.Decisions[i-1].WorkUnitID, primero.Decisions[i].WorkUnitID)
		}
	}

	ref, _ := json.Marshal(primero)
	for i := 0; i < 100; i++ {
		otro, err := RoutePlan(in)
		if err != nil {
			t.Fatalf("corrida %d: %v", i, err)
		}
		data, _ := json.Marshal(otro)
		if string(data) != string(ref) {
			t.Fatalf("la corrida %d difiere del plan de referencia", i)
		}
	}
}

// T106 — SC-004: un plan de 50 tareas se enruta en mucho menos de 1 segundo.
func TestRoutePlan_Rendimiento50Tareas(t *testing.T) {
	var unidades []WorkUnit
	for i := 1; i <= 50; i++ {
		u := unidadAislable(idTarea(i))
		if i > 1 {
			u.Dependencies = []string{idTarea(i - 1)}
		}
		unidades = append(unidades, u)
	}

	if testing.Short() {
		t.Skip("medición de tiempo omitida en modo corto")
	}
	inicio := testingNow()
	if _, err := RoutePlan(planDePrueba(unidades...)); err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}
	if d := testingSince(inicio); d.Seconds() >= 1 {
		t.Errorf("enrutar 50 tareas tardó %v, el presupuesto es < 1s", d)
	}
}

func idTarea(n int) string {
	const digitos = "0123456789"
	return "T" + string([]byte{digitos[(n/100)%10], digitos[(n/10)%10], digitos[n%10]})
}

// Una dependencia TRANSITIVA también impide compartir grupo: T003 depende de
// T002, que depende de T001, así que T003 y T001 no pueden correr a la vez
// aunque no haya arista directa entre ellas.
func TestRoutePlan_DependenciaTransitivaNoComparteGrupo(t *testing.T) {
	plan, err := RoutePlan(planDePrueba(
		unidadAislable("T001"),
		unidadAislable("T002", "T001"),
		unidadAislable("T003", "T002"),
	))
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}

	for _, g := range plan.ParallelGroups {
		if len(g.Tasks) > 1 {
			t.Errorf("una cadena de dependencias no admite paralelismo: grupo %s = %v", g.ID, g.Tasks)
		}
	}
}

// Un plan de una sola unidad delegada no forma grupo: un grupo de uno no es
// paralelismo, y prometerlo mentiría al runtime sobre la forma del plan.
func TestRoutePlan_UnaSolaDelegadaNoFormaGrupo(t *testing.T) {
	plan, err := RoutePlan(planDePrueba(unidadAislable("T001")))
	if err != nil {
		t.Fatalf("RoutePlan: %v", err)
	}
	if len(plan.ParallelGroups) != 0 {
		t.Errorf("no debería formarse grupo: %v", plan.ParallelGroups)
	}
	if d := plan.Decision("T001"); d.Route == RouteParallel {
		t.Error("una unidad sola conserva DELEGATE, no se promueve a PARALLEL")
	}
}
