package persistence

import (
	"testing"

	"mem/domain"
)

func repoOctopus(t *testing.T) *OctopusRepository {
	t.Helper()
	db, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewOctopusRepository(db)
}

func decision(id string, ruta domain.Route, costo int) domain.RouteDecision {
	return domain.RouteDecision{
		WorkUnitID: id, Route: ruta, Reason: domain.ReasonIsolatableInvestigation,
		ContextBudget: 2000, OutputBudget: 900,
		EstimatedCost: domain.CostEstimate{ContextTokens: costo},
		Estimated:     true,
	}
}

// T073: ciclo completo — se registra la decisión, llega el reporte, y el
// historial refleja ambos.
func TestOctopusRepository_DecisionYReporte(t *testing.T) {
	r := repoOctopus(t)

	r.RecordDecision("proj", "plan-1", domain.ClassInvestigation, decision("T004", domain.RouteDelegate, 3000))

	hist := r.History("proj", "", 10)
	if len(hist) != 1 {
		t.Fatalf("esperaba 1 fila en el historial, hay %d", len(hist))
	}
	if hist[0].Reported {
		t.Error("una decisión sin reporte no debe aparecer como reportada")
	}
	if hist[0].EstimatedCost != 3000 {
		t.Errorf("EstimatedCost = %d, esperaba 3000", hist[0].EstimatedCost)
	}

	r.RecordReport("proj", domain.ExecutionReport{
		TaskID: "T004", Route: domain.RouteDelegate, Status: domain.StatusCompleted,
		ContextTokens: 2410, OutputTokens: 742, DurationMS: 4200, Quality: domain.QualityAccepted,
	})

	hist = r.History("proj", "", 10)
	if !hist[0].Reported {
		t.Fatal("tras el reporte la fila debe aparecer como reportada")
	}
	if hist[0].ContextTokens != 2410 || hist[0].OutputTokens != 742 {
		t.Errorf("consumo real = %d/%d, esperaba 2410/742", hist[0].ContextTokens, hist[0].OutputTokens)
	}
	if hist[0].Quality != domain.QualityAccepted {
		t.Errorf("calidad = %q", hist[0].Quality)
	}
}

// T074: un reporte huérfano se ignora SIN error. El runtime nunca debe romperse
// por informarnos de algo que no le pedimos.
func TestOctopusRepository_ReporteHuerfanoSeIgnora(t *testing.T) {
	r := repoOctopus(t)

	r.RecordReport("proj", domain.ExecutionReport{TaskID: "T999", Status: domain.StatusCompleted})

	if n := len(r.History("proj", "", 10)); n != 0 {
		t.Errorf("un reporte huérfano no debe crear filas, hay %d", n)
	}
}

// Un repositorio sin base no revienta: fire-and-forget de verdad.
func TestOctopusRepository_NilEsSeguro(t *testing.T) {
	var r *OctopusRepository
	r.RecordDecision("proj", "", domain.ClassInvestigation, decision("T001", domain.RouteInline, 10))
	r.RecordReport("proj", domain.ExecutionReport{TaskID: "T001"})
	if len(r.Evidence("proj")) != 0 || len(r.History("proj", "", 5)) != 0 {
		t.Error("un repositorio nil debe devolver vacío, no reventar")
	}
	if r.Stats("proj").Decisiones != 0 {
		t.Error("un repositorio nil no tiene estadísticas")
	}
}

// El historial está aislado por proyecto, como toda la memoria de gomemory.
func TestOctopusRepository_AisladoPorProyecto(t *testing.T) {
	r := repoOctopus(t)
	r.RecordDecision("proj-a", "", domain.ClassInvestigation, decision("T001", domain.RouteDelegate, 100))
	r.RecordDecision("proj-b", "", domain.ClassInvestigation, decision("T002", domain.RouteDelegate, 100))

	if n := len(r.History("proj-a", "", 10)); n != 1 {
		t.Errorf("proj-a debería ver 1 fila, ve %d", n)
	}
}

// T076: los agregados distinguen por ruta, y separan estimado de real.
func TestOctopusRepository_Stats(t *testing.T) {
	r := repoOctopus(t)

	r.RecordDecision("proj", "p1", domain.ClassInvestigation, decision("T001", domain.RouteInline, 500))
	r.RecordDecision("proj", "p1", domain.ClassInvestigation, decision("T002", domain.RouteDelegate, 3000))
	r.RecordDecision("proj", "p1", domain.ClassInvestigation, decision("T003", domain.RouteDelegate, 3000))
	r.RecordReport("proj", domain.ExecutionReport{TaskID: "T002", Status: domain.StatusCompleted, ContextTokens: 1500, OutputTokens: 500})
	r.RecordReport("proj", domain.ExecutionReport{TaskID: "T003", Status: domain.StatusFailed, ContextTokens: 900, OutputTokens: 100})

	s := r.Stats("proj")

	if s.Decisiones != 3 {
		t.Errorf("Decisiones = %d, esperaba 3", s.Decisiones)
	}
	if s.PorRuta[domain.RouteDelegate] != 2 || s.PorRuta[domain.RouteInline] != 1 {
		t.Errorf("PorRuta = %v", s.PorRuta)
	}
	if s.ConReporte != 2 {
		t.Errorf("ConReporte = %d, esperaba 2", s.ConReporte)
	}
	if s.Exitos != 1 || s.Fallos != 1 {
		t.Errorf("éxitos/fallos = %d/%d, esperaba 1/1", s.Exitos, s.Fallos)
	}
	if s.TokensReales != 3000 {
		t.Errorf("TokensReales = %d, esperaba 3000", s.TokensReales)
	}
	ahorro, ok := s.AhorroEstimado()
	if !ok {
		t.Fatal("con reportes debe poder calcularse el ahorro")
	}
	if ahorro != s.TokensEstimados-s.TokensReales {
		t.Errorf("ahorro = %d", ahorro)
	}
}

// Sin ningún reporte, el ahorro NO se declara: presentar la estimación como
// resultado medido es exactamente lo que prohíbe FR-033.
func TestOctopusRepository_SinReportesNoHayAhorroQueDeclarar(t *testing.T) {
	r := repoOctopus(t)
	r.RecordDecision("proj", "", domain.ClassInvestigation, decision("T001", domain.RouteDelegate, 3000))

	if _, ok := r.Stats("proj").AhorroEstimado(); ok {
		t.Error("sin reportes no puede declararse ahorro")
	}
}

// T096: la evidencia agregada por clase solo cuenta lo REPORTADO.
func TestOctopusRepository_Evidence(t *testing.T) {
	r := repoOctopus(t)

	for i, caso := range []struct {
		id    string
		ruta  domain.Route
		ctx   int
		out   int
		exito bool
	}{
		{"T001", domain.RouteInline, 5000, 1200, true},
		{"T002", domain.RouteInline, 5400, 1000, true},
		{"T003", domain.RouteDelegate, 1800, 700, true},
		{"T004", domain.RouteDelegate, 1900, 600, true},
		{"T005", domain.RouteDelegate, 1700, 800, true},
		{"T006", domain.RouteDelegate, 1600, 700, false},
	} {
		_ = i
		r.RecordDecision("proj", "p1", domain.ClassRepositoryExploration, decision(caso.id, caso.ruta, 3000))
		estado := domain.StatusCompleted
		if !caso.exito {
			estado = domain.StatusFailed
		}
		r.RecordReport("proj", domain.ExecutionReport{
			TaskID: caso.id, Route: caso.ruta, Status: estado,
			ContextTokens: caso.ctx, OutputTokens: caso.out,
		})
	}

	ev := r.Evidence("proj")[domain.ClassRepositoryExploration]
	if ev == nil {
		t.Fatal("debería haber evidencia para repository-exploration")
	}
	if ev.Executions != 6 {
		t.Errorf("Executions = %d, esperaba 6", ev.Executions)
	}
	if ev.DelegatedAvgTokens >= ev.InlineAvgTokens {
		t.Errorf("delegado (%d) debería salir más barato que inline (%d)", ev.DelegatedAvgTokens, ev.InlineAvgTokens)
	}
	if ev.Favorece() {
		t.Error("tres de cuatro delegaciones exitosas no debe alcanzar el umbral, aunque los inline hayan funcionado")
	}
}

// Una decisión sin reporte no alimenta la evidencia: estimar no es medir.
func TestOctopusRepository_EvidenciaIgnoraLoNoReportado(t *testing.T) {
	r := repoOctopus(t)
	r.RecordDecision("proj", "", domain.ClassInvestigation, decision("T001", domain.RouteDelegate, 3000))

	if len(r.Evidence("proj")) != 0 {
		t.Error("sin reporte no hay evidencia que agregar")
	}
}
