package usecases

import (
	"mem/application/ports"
	"mem/domain"
)

// --- Octopus AAR: telemetría de ejecución (feature 027) ---
//
// Todo en este archivo es fire-and-forget. Ninguna función devuelve error, y no
// es un descuido: medir jamás puede impedir enrutar ni ejecutar. Un repositorio
// nil es válido y significa "sin memoria de lo ocurrido", no "roto"
// (INV-AAR-015).

// ReportUseCase registra decisiones y reportes, y sirve los agregados.
type ReportUseCase struct {
	repo ports.OctopusRepository
}

func NewReportUseCase(repo ports.OctopusRepository) *ReportUseCase {
	return &ReportUseCase{repo: repo}
}

// RecordPlan guarda todas las decisiones de un plan recién enrutado, para poder
// contrastarlas después con lo que costaron de verdad.
func (uc *ReportUseCase) RecordPlan(project string, plan domain.RoutingPlan, clases map[string]domain.TaskClass) {
	if uc == nil || uc.repo == nil {
		return
	}
	for _, d := range plan.Decisions {
		uc.repo.RecordDecision(project, plan.PlanID, clases[d.WorkUnitID], d)
	}
}

// RecordDecision guarda una decisión suelta.
func (uc *ReportUseCase) RecordDecision(project string, class domain.TaskClass, d domain.RouteDecision) {
	if uc == nil || uc.repo == nil {
		return
	}
	uc.repo.RecordDecision(project, "", class, d)
}

// Report ingiere el resultado real informado por el runtime.
func (uc *ReportUseCase) Report(project string, r domain.ExecutionReport) {
	if uc == nil || uc.repo == nil {
		return
	}
	uc.repo.RecordReport(project, r)
}

// Stats devuelve los agregados del proyecto. Sin repositorio, agregados vacíos:
// una respuesta honesta, no un error.
func (uc *ReportUseCase) Stats(project string) domain.RoutingStats {
	if uc == nil || uc.repo == nil {
		return domain.RoutingStats{PorRuta: map[domain.Route]int{}}
	}
	return uc.repo.Stats(project)
}

// History devuelve las últimas decisiones con su resultado.
func (uc *ReportUseCase) History(project string, class domain.TaskClass, limit int) []domain.ExecutionRecord {
	if uc == nil || uc.repo == nil {
		return nil
	}
	return uc.repo.History(project, class, limit)
}

// Evidence devuelve la evidencia histórica por clase, para alimentar el
// desempate de la política. Sin historial, mapa vacío: el arranque en frío es el
// caso normal el primer día, no una anomalía (FR-048, AC-015).
func (uc *ReportUseCase) Evidence(project string) map[domain.TaskClass]*domain.ClassEvidence {
	if uc == nil || uc.repo == nil {
		return nil
	}
	return uc.repo.Evidence(project)
}

// --- Manejo acotado de fallos (Historia 7) ---

// FailureRequest pide qué hacer tras un desenlace adverso de una delegación.
type FailureRequest struct {
	Project       string
	Class         domain.TaskClass
	Report        domain.ExecutionReport
	Result        domain.DelegatedResult
	Attempts      domain.AttemptState
	Policy        domain.PolicyOverrides
	ParentCanDoIt bool
}

// FailureDecision es la recomendación, ya acotada por los topes.
type FailureDecision struct {
	Policy domain.FailurePolicy
	// ExtraContextTokens es cuánto ampliar el paquete, solo con EXPAND_CONTEXT.
	ExtraContextTokens int
	// PartialResult es el resultado parcial que puede entregarse al padre, o
	// nil si no aporta nada.
	PartialResult *domain.DelegatedResult
}

// expansionExtraTokens es cuánto se amplía el contexto en la ÚNICA ampliación
// autorizada. Una cifra fija y acotada: si la tarea necesitara mucho más, el
// problema no es el presupuesto sino que la unidad no estaba bien acotada, y
// para eso está el repliegue.
const expansionExtraTokens = 1500

// HandleFailure registra el desenlace y devuelve la recomendación.
//
// El registro va PRIMERO y es fire-and-forget: un fallo de escritura de
// telemetría no puede impedir que el llamador sepa qué hacer a continuación.
func (uc *ReportUseCase) HandleFailure(req FailureRequest) FailureDecision {
	uc.Report(req.Project, req.Report)

	estado := req.Attempts
	estado.ParentCanDoIt = req.ParentCanDoIt

	politica := domain.NextAfterFailure(req.Report.Status, estado, req.Policy)
	decision := FailureDecision{Policy: politica}

	if politica == domain.PolicyExpandContext {
		decision.ExtraContextTokens = expansionExtraTokens
	}
	// El resultado parcial solo viaja cuando el padre va a asumir el trabajo y
	// hay algo aprovechable: en un reintento el hijo lo rehará de todos modos.
	if politica == domain.PolicyFallbackInline && req.Result.ConservaResultadoParcial() {
		parcial := req.Result
		decision.PartialResult = &parcial
	}
	return decision
}
