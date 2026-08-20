package usecases_test

import (
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// fakeUsageReportRepo implementa ports.UsageRepository en memoria para probar
// la agregación sin tocar SQLite.
type fakeUsageReportRepo struct {
	bySession map[string][]domain.UsageRecord
	totals    []domain.UsageRecord
}

func (f *fakeUsageReportRepo) Record(rec domain.UsageRecord) error { return nil }

func (f *fakeUsageReportRepo) BySession(project, sessionID string) ([]domain.UsageRecord, error) {
	return f.bySession[sessionID], nil
}

func (f *fakeUsageReportRepo) Sessions(project string, limit int) ([]string, error) {
	var ids []string
	for id := range f.bySession {
		ids = append(ids, id)
	}
	return ids, nil
}

func (f *fakeUsageReportRepo) Totals(project string) ([]domain.UsageRecord, error) {
	return f.totals, nil
}

func TestBuildUsageReport_AggregatesByOperationAndChannel(t *testing.T) {
	repo := &fakeUsageReportRepo{
		bySession: map[string][]domain.UsageRecord{
			"sess-1": {
				{Project: "proj", SessionID: "sess-1", Operation: domain.OpBuildContext, Channel: "mcp", BaselineTokens: 6000, EmittedTokens: 3500},
				{Project: "proj", SessionID: "sess-1", Operation: domain.OpSearchMemories, Channel: "mcp", BaselineTokens: 1800, EmittedTokens: 1500},
				{Project: "proj", SessionID: "sess-1", Operation: domain.OpSaveMemory, Channel: "cli", BaselineTokens: 320, EmittedTokens: 310},
			},
		},
	}

	report, err := usecases.BuildUsageReport(repo, "proj", "sess-1", 0)
	if err != nil {
		t.Fatalf("BuildUsageReport: %v", err)
	}

	if report.Calls != 3 {
		t.Fatalf("Calls = %d, want 3", report.Calls)
	}
	if report.BaselineTokens != 8120 || report.EmittedTokens != 5310 {
		t.Fatalf("totales inesperados: baseline=%d emitted=%d", report.BaselineTokens, report.EmittedTokens)
	}

	// G1: baseline - saved == emitted, exacto.
	if report.BaselineTokens-report.Saved() != report.EmittedTokens {
		t.Fatalf("G1 rota: baseline(%d) - saved(%d) != emitted(%d)", report.BaselineTokens, report.Saved(), report.EmittedTokens)
	}

	// G3: la suma de ByOperation y de ByChannel debe igualar el total.
	var opSum, chSum int
	for _, b := range report.ByOperation {
		opSum += b.BaselineTokens
	}
	for _, b := range report.ByChannel {
		chSum += b.BaselineTokens
	}
	if opSum != report.BaselineTokens {
		t.Fatalf("G3 rota (by_operation): %d != %d", opSum, report.BaselineTokens)
	}
	if chSum != report.BaselineTokens {
		t.Fatalf("G3 rota (by_channel): %d != %d", chSum, report.BaselineTokens)
	}

	// G4: la suma de calls de ambos desgloses debe igualar Calls.
	var opCalls, chCalls int
	for _, b := range report.ByOperation {
		opCalls += b.Calls
	}
	for _, b := range report.ByChannel {
		chCalls += b.Calls
	}
	if opCalls != report.Calls || chCalls != report.Calls {
		t.Fatalf("G4 rota: opCalls=%d chCalls=%d Calls=%d", opCalls, chCalls, report.Calls)
	}

	// G6: ambos desgloses ordenados descendente por BaselineTokens.
	for i := 1; i < len(report.ByOperation); i++ {
		if report.ByOperation[i].BaselineTokens > report.ByOperation[i-1].BaselineTokens {
			t.Fatalf("ByOperation no está ordenado descendente: %+v", report.ByOperation)
		}
	}
}

func TestBuildUsageReport_EmptySession_NoErrorZeroValues(t *testing.T) {
	repo := &fakeUsageReportRepo{bySession: map[string][]domain.UsageRecord{}}

	report, err := usecases.BuildUsageReport(repo, "proj", "sess-vacia", 0)
	if err != nil {
		t.Fatalf("una sesión sin registros no debe producir error: %v", err)
	}
	if report.Calls != 0 || report.BaselineTokens != 0 {
		t.Fatalf("se esperaba reporte en ceros, got %+v", report)
	}
	if report.ReductionRatio() != 0 {
		t.Fatalf("ReductionRatio con baseline 0 debe ser 0, got %v", report.ReductionRatio())
	}
}

func TestBuildUsageReport_AllSessions_UsesTotals(t *testing.T) {
	repo := &fakeUsageReportRepo{
		totals: []domain.UsageRecord{
			{Project: "proj", SessionID: "a", Operation: domain.OpBuildContext, Channel: "mcp", BaselineTokens: 100, EmittedTokens: 80},
			{Project: "proj", SessionID: "b", Operation: domain.OpBuildContext, Channel: "cli", BaselineTokens: 50, EmittedTokens: 50},
		},
	}

	report, err := usecases.BuildUsageReport(repo, "proj", "", 0)
	if err != nil {
		t.Fatalf("BuildUsageReport (todas las sesiones): %v", err)
	}
	if report.Calls != 2 || report.BaselineTokens != 150 {
		t.Fatalf("reporte de todas las sesiones inesperado: %+v", report)
	}
}

func TestBuildUsageReport_WindowTokens_Propagated(t *testing.T) {
	repo := &fakeUsageReportRepo{bySession: map[string][]domain.UsageRecord{
		"s": {{Project: "proj", SessionID: "s", Operation: domain.OpSaveMemory, Channel: "cli", BaselineTokens: 10, EmittedTokens: 10}},
	}}

	report, err := usecases.BuildUsageReport(repo, "proj", "s", 200000)
	if err != nil {
		t.Fatalf("BuildUsageReport: %v", err)
	}
	if report.WindowTokens != 200000 {
		t.Fatalf("WindowTokens = %d, want 200000", report.WindowTokens)
	}
	if _, ok := report.WindowRatio(); !ok {
		t.Fatalf("con WindowTokens>0, WindowRatio debe ser válido")
	}
}
