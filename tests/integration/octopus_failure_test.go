package main

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// T089 — el ciclo completo: delegar → fallo → reintento → fallo → repliegue
// inline, conservando el trabajo parcial útil. Con repositorio REAL, para que la
// telemetría de cada paso quede registrada como en producción.
func TestOctopusCicloDeFalloCompleto(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	repo := persistence.NewOctopusRepository(db)
	uc := usecases.NewReportUseCase(repo)

	const proyecto = "proj"
	decision := domain.RouteDecision{
		WorkUnitID: "T004", Route: domain.RouteDelegate,
		Reason: domain.ReasonIsolatableInvestigation, ContextBudget: 2200, OutputBudget: 900,
		EstimatedCost: domain.CostEstimate{ContextTokens: 2200, OutputTokens: 900},
	}
	repo.RecordDecision(proyecto, "p1", domain.ClassInvestigation, decision)

	parcial := domain.DelegatedResult{
		TaskID: "T004", Status: domain.StatusFailed,
		Evidence: []string{"store.go:88 toma el lock antes de leer"},
	}

	// Primer fallo → reintento.
	paso1 := uc.HandleFailure(usecases.FailureRequest{
		Project:       proyecto,
		Report:        domain.ExecutionReport{TaskID: "T004", Route: domain.RouteDelegate, Status: domain.StatusFailed, ContextTokens: 2100},
		Result:        parcial,
		ParentCanDoIt: true,
	})
	if paso1.Policy != domain.PolicyRetry {
		t.Fatalf("paso 1 = %q, esperaba RETRY", paso1.Policy)
	}
	if paso1.PartialResult != nil {
		t.Error("en un reintento el hijo rehará el trabajo: el parcial no debe viajar")
	}

	// Segundo fallo → repliegue inline, con el trabajo útil conservado.
	repo.RecordDecision(proyecto, "p1", domain.ClassInvestigation, decision)
	paso2 := uc.HandleFailure(usecases.FailureRequest{
		Project:       proyecto,
		Report:        domain.ExecutionReport{TaskID: "T004", Route: domain.RouteDelegate, Status: domain.StatusFailed, ContextTokens: 2050},
		Result:        parcial,
		Attempts:      domain.AttemptState{Retries: 1},
		ParentCanDoIt: true,
	})
	if paso2.Policy != domain.PolicyFallbackInline {
		t.Fatalf("paso 2 = %q, esperaba FALLBACK_INLINE", paso2.Policy)
	}
	if paso2.PartialResult == nil || len(paso2.PartialResult.Evidence) == 0 {
		t.Error("al replegar, la evidencia recogida por el hijo debe llegar al padre")
	}

	// Y todo quedó medido: dos intentos fallidos, ninguno perdido.
	s := uc.Stats(proyecto)
	if s.Fallos != 2 {
		t.Errorf("Fallos = %d, esperaba los 2 intentos", s.Fallos)
	}
	if _, ok := s.RatioDeExito(); !ok {
		t.Error("con desenlaces registrados debe poder calcularse la tasa de éxito")
	}
}

// AC-012 extremo a extremo: contexto insuficiente amplía UNA vez y luego repliega.
func TestOctopusContextoInsuficienteAmpliaUnaVez(t *testing.T) {
	uc := usecases.NewReportUseCase(nil)

	primera := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T004", Status: domain.StatusInsufficientContext},
		ParentCanDoIt: true,
	})
	if primera.Policy != domain.PolicyExpandContext || primera.ExtraContextTokens <= 0 {
		t.Fatalf("primera = %q (+%d)", primera.Policy, primera.ExtraContextTokens)
	}

	segunda := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T004", Status: domain.StatusInsufficientContext},
		Attempts:      domain.AttemptState{Expansions: 1},
		ParentCanDoIt: true,
	})
	if segunda.Policy != domain.PolicyFallbackInline {
		t.Errorf("segunda = %q, esperaba repliegue tras agotar la ampliación", segunda.Policy)
	}
}

// La ampliación se traduce en un paquete de contexto EFECTIVAMENTE mayor, y una
// sola vez: FR-042 promete una ampliación acotada, no un presupuesto elástico.
func TestOctopusAmpliacionProduceUnPaqueteMayor(t *testing.T) {
	uc := usecases.NewPackContractUseCase(nil, nil, nil, nil)

	base := usecases.PackContractRequest{
		Unit: domain.WorkUnit{
			ID: "T004", Objective: "investigar la expiración",
			Scope: domain.Scope{Files: []string{"a.go"}, ReadOnly: true},
		},
		Decision: domain.RouteDecision{
			WorkUnitID: "T004", Route: domain.RouteDelegate,
			ContextBudget: 2000, OutputBudget: 800,
		},
		ParentPermissions: domain.Permissions{Filesystem: domain.FSReadWrite},
	}

	sinAmpliar, err := uc.Build(base)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	ampliado := base
	ampliado.ExtraContextTokens = 1500
	conAmpliacion, err := uc.Build(ampliado)
	if err != nil {
		t.Fatalf("Build ampliado: %v", err)
	}

	if conAmpliacion.Contract.ContextBudget != sinAmpliar.Contract.ContextBudget+1500 {
		t.Errorf("presupuesto ampliado = %d, esperaba %d",
			conAmpliacion.Contract.ContextBudget, sinAmpliar.Contract.ContextBudget+1500)
	}
}
