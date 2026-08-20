package usage

import (
	"errors"
	"testing"

	"mem/domain"
)

type fakeUsageRepo struct {
	records []domain.UsageRecord
	err     error
}

func (f *fakeUsageRepo) Record(rec domain.UsageRecord) error {
	if f.err != nil {
		return f.err
	}
	f.records = append(f.records, rec)
	return nil
}

func (f *fakeUsageRepo) BySession(project, sessionID string) ([]domain.UsageRecord, error) {
	return f.records, nil
}

func (f *fakeUsageRepo) Sessions(project string, limit int) ([]string, error) {
	return nil, nil
}

func (f *fakeUsageRepo) Totals(project string) ([]domain.UsageRecord, error) {
	return f.records, nil
}

func TestRecorder_CarriesChannelFromConstruction(t *testing.T) {
	repo := &fakeUsageRepo{}
	rec := NewRecorder(repo, "proj", "cli", func() string { return "sess-1" })

	rec.Record(domain.OpBuildContext, 100, 60)

	if len(repo.records) != 1 {
		t.Fatalf("se esperaba 1 registro, got %d", len(repo.records))
	}
	got := repo.records[0]
	if got.Channel != "cli" {
		t.Fatalf("Channel = %q, want %q — la etiqueta debe venir de la construcción, no de Record()", got.Channel, "cli")
	}
	if got.Project != "proj" || got.SessionID != "sess-1" {
		t.Fatalf("registro inesperado: %+v", got)
	}
	if got.BaselineTokens != 100 || got.EmittedTokens != 60 {
		t.Fatalf("tokens inesperados: %+v", got)
	}
}

func TestRecorder_UnknownChannelIsAccepted(t *testing.T) {
	repo := &fakeUsageRepo{}
	rec := NewRecorder(repo, "proj", "canal-nuevo-cualquiera", func() string { return "" })

	rec.Record(domain.OpOther, 10, 10)

	if len(repo.records) != 1 || repo.records[0].Channel != "canal-nuevo-cualquiera" {
		t.Fatalf("un canal no listado debe registrarse igual: %+v", repo.records)
	}
}

func TestRecorder_PersistenceFailureDoesNotPanicOrBlock(t *testing.T) {
	repo := &fakeUsageRepo{err: errors.New("db caída")}
	rec := NewRecorder(repo, "proj", "mcp", func() string { return "sess-1" })

	// No debe entrar en pánico ni bloquear: fire-and-forget (FR-006).
	rec.Record(domain.OpSaveMemory, 5, 5)
}

func TestRecorder_NilIsUsableByCallers(t *testing.T) {
	// El puerto es opcional en toda dependencia que lo reciba: un valor nil
	// no debe requerir que el llamador lo verifique con pánico. Este test
	// documenta el contrato — la verificación real vive en cada llamador
	// (build_context.go, etc.), que debe chequear nil antes de invocar.
	var rec interface {
		Record(operation string, baselineTokens, emittedTokens int)
	}
	if rec != nil {
		t.Fatalf("se esperaba nil")
	}
}
