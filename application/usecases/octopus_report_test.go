package usecases_test

import (
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// repoOctopusEspia registra las llamadas sin base de datos: aquí se prueba el
// contrato del caso de uso, no el SQL (que tiene su propia prueba con base real).
type repoOctopusEspia struct {
	decisiones []domain.RouteDecision
	reportes   []domain.ExecutionReport
}

func (r *repoOctopusEspia) RecordDecision(_, _ string, _ domain.TaskClass, d domain.RouteDecision) {
	r.decisiones = append(r.decisiones, d)
}
func (r *repoOctopusEspia) RecordReport(_ string, rep domain.ExecutionReport) {
	r.reportes = append(r.reportes, rep)
}
func (r *repoOctopusEspia) Evidence(string) map[domain.TaskClass]*domain.ClassEvidence { return nil }
func (r *repoOctopusEspia) Stats(string) domain.RoutingStats                           { return domain.RoutingStats{} }
func (r *repoOctopusEspia) History(string, domain.TaskClass, int) []domain.ExecutionRecord {
	return nil
}

// Un plan enrutado registra TODAS sus decisiones, no solo las delegadas: sin las
// inline no hay con qué comparar después.
func TestReportUseCase_RegistraTodoElPlan(t *testing.T) {
	espia := &repoOctopusEspia{}
	uc := usecases.NewReportUseCase(espia)

	plan := domain.RoutingPlan{
		PlanID: "p1",
		Decisions: []domain.RouteDecision{
			{WorkUnitID: "T001", Route: domain.RouteInline, Reason: domain.ReasonTrivial},
			{WorkUnitID: "T002", Route: domain.RouteDelegate, Reason: domain.ReasonBoundedInterface},
		},
	}
	uc.RecordPlan("proj", plan, map[string]domain.TaskClass{"T002": domain.ClassIntegration})

	if len(espia.decisiones) != 2 {
		t.Errorf("registradas %d decisiones, esperaba las 2 del plan", len(espia.decisiones))
	}
}

// T074: sin repositorio, todo sigue funcionando. Fire-and-forget de verdad.
func TestReportUseCase_SinRepositorioNoRevienta(t *testing.T) {
	uc := usecases.NewReportUseCase(nil)

	uc.RecordPlan("proj", domain.RoutingPlan{Decisions: []domain.RouteDecision{{WorkUnitID: "T001"}}}, nil)
	uc.RecordDecision("proj", domain.ClassInvestigation, domain.RouteDecision{WorkUnitID: "T001"})
	uc.Report("proj", domain.ExecutionReport{TaskID: "T001"})

	if s := uc.Stats("proj"); s.Decisiones != 0 {
		t.Error("sin repositorio los agregados están vacíos, no rotos")
	}
	if uc.History("proj", "", 10) != nil {
		t.Error("sin repositorio el historial es nil")
	}
	if uc.Evidence("proj") != nil {
		t.Error("sin repositorio no hay evidencia")
	}
}

// El reporte llega tal cual al repositorio: este caso de uso no interpreta lo
// que el runtime informa, solo lo persiste.
func TestReportUseCase_PasaElReporteSinAlterarlo(t *testing.T) {
	espia := &repoOctopusEspia{}
	uc := usecases.NewReportUseCase(espia)

	rep := domain.ExecutionReport{
		TaskID: "T004", Route: domain.RouteDelegate, Status: domain.StatusCompleted,
		ContextTokens: 2410, OutputTokens: 742, DurationMS: 4200, Quality: domain.QualityAccepted,
	}
	uc.Report("proj", rep)

	if len(espia.reportes) != 1 || espia.reportes[0] != rep {
		t.Errorf("el reporte debe llegar íntegro: %+v", espia.reportes)
	}
}

// --- Historia 7: el caso de uso registra y recomienda, en ese orden ---

// El desenlace se registra SIEMPRE, gane la recomendación que gane: la
// telemetría de un fallo es justo la que más sirve después.
func TestHandleFailure_RegistraAntesDeRecomendar(t *testing.T) {
	espia := &repoOctopusEspia{}
	uc := usecases.NewReportUseCase(espia)

	uc.HandleFailure(usecases.FailureRequest{
		Project: "proj",
		Report:  domain.ExecutionReport{TaskID: "T004", Status: domain.StatusFailed},
	})

	if len(espia.reportes) != 1 {
		t.Fatalf("el desenlace debe registrarse, hay %d reportes", len(espia.reportes))
	}
}

// AC-011 vía el caso de uso: un segundo fallo no reintenta.
func TestHandleFailure_NoReintentaIndefinidamente(t *testing.T) {
	uc := usecases.NewReportUseCase(nil)

	primera := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T004", Status: domain.StatusFailed},
		ParentCanDoIt: true,
	})
	if primera.Policy != domain.PolicyRetry {
		t.Errorf("primer fallo = %q, esperaba RETRY", primera.Policy)
	}

	segunda := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T004", Status: domain.StatusFailed},
		Attempts:      domain.AttemptState{Retries: 1},
		ParentCanDoIt: true,
	})
	if segunda.Policy != domain.PolicyFallbackInline {
		t.Errorf("segundo fallo = %q, esperaba FALLBACK_INLINE", segunda.Policy)
	}
}

// AC-012: la ampliación de contexto es una, acotada, y trae cuánto ampliar.
func TestHandleFailure_AmpliacionUnicaYAcotada(t *testing.T) {
	uc := usecases.NewReportUseCase(nil)

	primera := uc.HandleFailure(usecases.FailureRequest{
		Report: domain.ExecutionReport{TaskID: "T004", Status: domain.StatusInsufficientContext},
	})
	if primera.Policy != domain.PolicyExpandContext {
		t.Fatalf("policy = %q, esperaba EXPAND_CONTEXT", primera.Policy)
	}
	if primera.ExtraContextTokens <= 0 {
		t.Error("una ampliación debe declarar cuánto amplía")
	}

	segunda := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T004", Status: domain.StatusInsufficientContext},
		Attempts:      domain.AttemptState{Expansions: 1},
		ParentCanDoIt: true,
	})
	if segunda.Policy == domain.PolicyExpandContext {
		t.Error("no puede haber una segunda ampliación automática")
	}
	if segunda.ExtraContextTokens != 0 {
		t.Error("sin ampliación no debe declararse cuánto ampliar")
	}
}

// FR-043: al replegar a inline, el trabajo parcial útil se entrega; el vacío no.
func TestHandleFailure_EntregaElParcialUtil(t *testing.T) {
	uc := usecases.NewReportUseCase(nil)

	conParcial := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T004", Status: domain.StatusFailed},
		Result:        domain.DelegatedResult{Evidence: []string{"store.go:88 toma el lock antes"}},
		Attempts:      domain.AttemptState{Retries: 1},
		ParentCanDoIt: true,
	})
	if conParcial.PartialResult == nil {
		t.Error("el trabajo parcial útil debe llegar al padre")
	}

	sinNada := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T005", Status: domain.StatusFailed},
		Result:        domain.DelegatedResult{Summary: "no llegué a nada"},
		Attempts:      domain.AttemptState{Retries: 1},
		ParentCanDoIt: true,
	})
	if sinNada.PartialResult != nil {
		t.Error("un resultado sin contenido útil solo ocuparía contexto")
	}

	// En un reintento el hijo rehará el trabajo: entregar el parcial ahí sería
	// contexto duplicado.
	enReintento := uc.HandleFailure(usecases.FailureRequest{
		Report:        domain.ExecutionReport{TaskID: "T006", Status: domain.StatusFailed},
		Result:        domain.DelegatedResult{Evidence: []string{"algo"}},
		ParentCanDoIt: true,
	})
	if enReintento.PartialResult != nil {
		t.Error("en un reintento no debe entregarse el parcial")
	}
}
