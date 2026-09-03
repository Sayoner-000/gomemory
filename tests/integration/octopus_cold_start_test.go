package main

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// T095 — AC-015: sin historial, toda unidad recibe una decisión válida. El
// arranque en frío es el caso NORMAL el primer día, no un modo degradado: si la
// política necesitara historial para funcionar, nadie podría empezar a usarla.
func TestOctopusArranqueEnFrio(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	repo := persistence.NewOctopusRepository(db)

	if len(repo.Evidence("proj")) != 0 {
		t.Fatal("preparación: la base debe estar vacía")
	}

	uc := usecases.NewRoutePlanUseCase(nil, nil).WithEvidence(repo)
	plan, err := uc.Route(usecases.RoutePlanRequest{
		PlanID:  "frio",
		Project: "proj",
		Units: []domain.WorkUnit{
			{ID: "T001", Objective: "investigar la expiración", Class: domain.ClassInvestigation,
				Scope:      domain.Scope{Files: []string{"a.go"}, ReadOnly: true},
				Complexity: domain.LevelMedium, ContextNeed: domain.ContextNeed{EstimatedTokens: 2200}},
			{ID: "T002", Objective: "corregir una errata", Class: domain.ClassTrivial,
				Complexity: domain.LevelTrivial, ContextNeed: domain.ContextNeed{EstimatedTokens: 50}},
		},
		Capabilities: domain.RuntimeCapabilities{Subagents: true, Parallel: true, IsolatedContext: true, MaxParallel: 2},
		Budget:       domain.NewBudget(50000, domain.DefaultBudgetSplit()),
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}

	for _, d := range plan.Decisions {
		if d.Route == "" {
			t.Errorf("%s: sin historial la ruta no puede quedar vacía", d.WorkUnitID)
		}
		if d.Reason.Text() == "" {
			t.Errorf("%s: sin historial la decisión debe seguir explicándose", d.WorkUnitID)
		}
		if d.Reason == domain.ReasonHistoricalEvidence {
			t.Errorf("%s: sin historial no puede alegarse evidencia histórica", d.WorkUnitID)
		}
	}
}

// La evidencia acumulada llega a la política a través del repositorio real, y
// cambia la preferencia — pero solo dentro de lo que las reglas duras permiten.
func TestOctopusEvidenciaAcumuladaLlegaALaPolitica(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	repo := persistence.NewOctopusRepository(db)

	const proyecto = "proj"
	// Historial: la migración sale sistemáticamente más barata delegada.
	for i, caso := range []struct {
		id   string
		ruta domain.Route
		ctx  int
	}{
		{"H001", domain.RouteInline, 6000}, {"H002", domain.RouteInline, 6400},
		{"H003", domain.RouteDelegate, 1800}, {"H004", domain.RouteDelegate, 1900},
		{"H005", domain.RouteDelegate, 1700}, {"H006", domain.RouteDelegate, 1750},
	} {
		_ = i
		repo.RecordDecision(proyecto, "hist", domain.ClassMigration, domain.RouteDecision{
			WorkUnitID: caso.id, Route: caso.ruta, Reason: domain.ReasonBoundedInterface,
		})
		repo.RecordReport(proyecto, domain.ExecutionReport{
			TaskID: caso.id, Route: caso.ruta, Status: domain.StatusCompleted,
			ContextTokens: caso.ctx, OutputTokens: 500,
		})
	}

	// Unidad sosa: sin evidencia se quedaría inline.
	unidad := domain.WorkUnit{
		ID: "T010", Objective: "migrar el esquema de la tabla",
		Class: domain.ClassMigration, Complexity: domain.LevelMedium,
		ContextNeed: domain.ContextNeed{EstimatedTokens: 3000},
	}
	caps := domain.RuntimeCapabilities{Subagents: true, IsolatedContext: true}
	presupuesto := domain.NewBudget(60000, domain.DefaultBudgetSplit())

	sinEvidencia, err := usecases.NewRouteTaskUseCase(nil).Route(usecases.RouteTaskRequest{
		Unit: unidad, Capabilities: caps, Budget: presupuesto,
	})
	if err != nil {
		t.Fatalf("sin evidencia: %v", err)
	}
	if sinEvidencia.Route.Delegada() {
		t.Fatalf("preparación: sin evidencia no debería delegarse (%q)", sinEvidencia.Reason)
	}

	conEvidencia, err := usecases.NewRouteTaskUseCase(nil).WithEvidence(repo).
		Route(usecases.RouteTaskRequest{
			Unit: unidad, Project: proyecto, Capabilities: caps, Budget: presupuesto,
		})
	if err != nil {
		t.Fatalf("con evidencia: %v", err)
	}
	if !conEvidencia.Route.Delegada() {
		t.Errorf("con historial favorable debería delegarse: %q / %q", conEvidencia.Route, conEvidencia.Reason)
	}
	if conEvidencia.Reason != domain.ReasonHistoricalEvidence {
		t.Errorf("razón = %q, esperaba %q", conEvidencia.Reason, domain.ReasonHistoricalEvidence)
	}

	// Y una restricción dura sigue mandando sobre la evidencia.
	sinCapacidades, err := usecases.NewRouteTaskUseCase(nil).WithEvidence(repo).
		Route(usecases.RouteTaskRequest{
			Unit: unidad, Project: proyecto, Capabilities: domain.RuntimeCapabilities{}, Budget: presupuesto,
		})
	if err != nil {
		t.Fatalf("sin capacidades: %v", err)
	}
	if sinCapacidades.Route.Delegada() {
		t.Error("la evidencia no puede saltarse la falta de subagentes")
	}
}
