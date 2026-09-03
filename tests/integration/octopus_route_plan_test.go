package main

import (
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// El escenario extremo a extremo de spec.md §66, enrutado por la cadena real
// (caso de uso → dominio), no por la política suelta.
func escenarioExpiracion() usecases.RoutePlanRequest {
	return usecases.RoutePlanRequest{
		PlanID: "expiracion-de-memorias",
		Capabilities: domain.RuntimeCapabilities{
			Subagents: true, Parallel: true, IsolatedContext: true, MaxParallel: 3,
		},
		Budget: domain.NewBudget(50000, domain.DefaultBudgetSplit()),
		Units: []domain.WorkUnit{
			{
				ID: "T001", Objective: "Definir el modelo de expiración", Class: domain.ClassArchitecture,
				Scope: domain.Scope{Files: []string{"expiration.go"}}, Complexity: domain.LevelMedium,
				ContextNeed: domain.ContextNeed{EstimatedTokens: 5200, NearlyFullParent: true},
			},
			{
				ID: "T002", Objective: "Implementar el almacenamiento", Class: domain.ClassImplementation,
				Dependencies: []string{"T001"},
				Scope:        domain.Scope{Files: []string{"store.go"}}, Complexity: domain.LevelHigh,
				ContextNeed: domain.ContextNeed{EstimatedTokens: 4800},
			},
			{
				ID: "T003", Objective: "Añadir la integración MCP", Class: domain.ClassIntegration,
				Scope: domain.Scope{Files: []string{"cmd_mcp.go"}}, Complexity: domain.LevelMedium,
				ContextNeed: domain.ContextNeed{EstimatedTokens: 2800},
			},
			{
				ID: "T004", Objective: "Investigar riesgos de concurrencia", Class: domain.ClassInvestigation,
				Scope:       domain.Scope{Files: []string{"expiration.go", "store.go"}, ReadOnly: true},
				Complexity:  domain.LevelMedium,
				ContextNeed: domain.ContextNeed{EstimatedTokens: 2200},
			},
			{
				ID: "T005", Objective: "Añadir pruebas de integración", Class: domain.ClassTesting,
				Dependencies: []string{"T002", "T003"},
				Scope:        domain.Scope{Files: []string{"expiration_test.go"}}, Complexity: domain.LevelMedium,
				ContextNeed: domain.ContextNeed{EstimatedTokens: 2400},
			},
			{
				ID: "T006", Objective: "Actualizar la documentación", Class: domain.ClassDocumentation,
				Scope:       domain.Scope{Files: []string{"ARQUITECTURA.md"}, ReadOnly: true},
				Complexity:  domain.LevelLow,
				ContextNeed: domain.ContextNeed{EstimatedTokens: 1600},
				Optional:    true,
			},
		},
	}
}

func TestOctopusEscenarioCompleto(t *testing.T) {
	uc := usecases.NewRoutePlanUseCase(nil, nil)

	plan, err := uc.Route(escenarioExpiracion())
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if len(plan.Decisions) != 6 {
		t.Fatalf("las 6 tareas deben recibir decisión, hay %d", len(plan.Decisions))
	}

	// T001 pide casi todo el contexto del padre: delegarla lo duplicaría.
	if d := plan.Decision("T001"); d.Route != domain.RouteInline || d.Reason != domain.ReasonContextNearlyFull {
		t.Errorf("T001 = %q/%q, esperaba INLINE por contexto casi completo", d.Route, d.Reason)
	}
	// T002 y T005 tienen dependencias sin resolver: no se ejecutan todavía.
	for _, id := range []string{"T002", "T005"} {
		d := plan.Decision(id)
		if d.Route != domain.RouteWait {
			t.Errorf("%s = %q, esperaba WAIT", id, d.Route)
		}
		if len(d.BlockedBy) == 0 {
			t.Errorf("%s: WAIT debe enumerar lo que bloquea", id)
		}
	}
	// T003 y T004 son independientes entre sí y aptas para correr a la vez.
	g3, g4 := plan.Decision("T003").ParallelGroup, plan.Decision("T004").ParallelGroup
	if g3 == "" || g3 != g4 {
		t.Errorf("T003 y T004 deberían compartir grupo paralelo: %q / %q", g3, g4)
	}

	// AC-005: ninguna tarea comparte grupo con algo de lo que dependa.
	for _, g := range plan.ParallelGroups {
		for _, id := range g.Tasks {
			d := plan.Decision(id)
			if len(d.BlockedBy) > 0 {
				t.Errorf("la tarea %s está bloqueada y no debería estar en el grupo %s", id, g.ID)
			}
		}
	}

	// INV-AAR-006: la reserva de validación sigue intacta.
	if plan.Budget.ValidationReserve != 7500 {
		t.Errorf("la reserva = %d, esperaba 7500 intactos", plan.Budget.ValidationReserve)
	}
	if plan.Budget.DelegationSpent > plan.Budget.DelegationPoolMax {
		t.Errorf("el fondo de delegación se desbordó: %d de %d",
			plan.Budget.DelegationSpent, plan.Budget.DelegationPoolMax)
	}
}

// El plan es asesor: reevaluarlo con dependencias ya resueltas libera lo que
// esperaba, sin reenrutar el trabajo completado (FR-020).
func TestOctopusReevaluacionLiberaLoQueEsperaba(t *testing.T) {
	uc := usecases.NewRoutePlanUseCase(nil, nil)

	req := escenarioExpiracion()
	req.Resolved = map[string]bool{"T001": true}

	plan, err := uc.Route(req)
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	if d := plan.Decision("T002"); d.Route == domain.RouteWait {
		t.Error("con T001 resuelta, T002 ya no debería esperar")
	}
	if d := plan.Decision("T005"); d.Route != domain.RouteWait {
		t.Error("T005 sigue dependiendo de T002 y T003: debe seguir esperando")
	}
}

// contadorDeCaracteres mide 1 token por carácter, para tener cifras
// predecibles sin depender de la heurística real del adaptador.
type contadorDeCaracteres struct{}

func (contadorDeCaracteres) Count(text string) int { return len([]rune(text)) }

// C-001 (ACR 027, reintento): en el camino de PLAN, ContractTokens quedaba en
// 0 para las 6 tareas del escenario — el mismo hueco que en route_task, pero
// nadie lo había cerrado ahí. Prueba la superficie real (RoutePlanUseCase.Route
// con un contador inyectado), no un ContractTokens puesto a mano en un WorkUnit.
func TestOctopusEscenarioCompleto_ContractTokensSalenDelProxy(t *testing.T) {
	uc := usecases.NewRoutePlanUseCase(contadorDeCaracteres{}, nil)

	plan, err := uc.Route(escenarioExpiracion())
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	for _, id := range []string{"T003", "T004"} {
		d := plan.Decision(id)
		if d.EstimatedCost.ContractTokens == 0 {
			t.Errorf("%s: ContractTokens debe salir del proxy (objetivo+alcance), no quedar en 0", id)
		}
	}
}
