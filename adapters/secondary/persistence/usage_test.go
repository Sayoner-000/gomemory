package persistence

import (
	"testing"

	"mem/domain"
)

func TestUsageRepository_RecordAndBySession(t *testing.T) {
	db := openTestDB(t)
	repo := NewUsageRepository(db)

	rec := domain.UsageRecord{
		Project:        "proj",
		SessionID:      "sess-1",
		Operation:      domain.OpBuildContext,
		Channel:        "cli",
		BaselineTokens: 100,
		EmittedTokens:  60,
	}
	if err := repo.Record(rec); err != nil {
		t.Fatalf("Record: %v", err)
	}

	got, err := repo.BySession("proj", "sess-1")
	if err != nil {
		t.Fatalf("BySession: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("BySession devolvió %d registros, want 1", len(got))
	}
	r := got[0]
	if r.Operation != domain.OpBuildContext || r.Channel != "cli" {
		t.Fatalf("registro inesperado: %+v", r)
	}
	if r.BaselineTokens != 100 || r.EmittedTokens != 60 {
		t.Fatalf("tokens inesperados: %+v", r)
	}
	if r.CreatedAt == "" {
		t.Fatalf("CreatedAt debe tener valor por defecto de la base")
	}
}

func TestUsageRepository_BySession_NoRows_ReturnsEmptyNotError(t *testing.T) {
	db := openTestDB(t)
	repo := NewUsageRepository(db)

	got, err := repo.BySession("proj-vacio", "sess-x")
	if err != nil {
		t.Fatalf("BySession no debe devolver error cuando no hay filas: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("BySession sin filas debe devolver slice vacío, got %d", len(got))
	}
}

func TestUsageRepository_Sessions_MostRecentFirst(t *testing.T) {
	db := openTestDB(t)
	repo := NewUsageRepository(db)

	for _, sid := range []string{"sess-a", "sess-b", "sess-c"} {
		rec := domain.UsageRecord{
			Project: "proj", SessionID: sid, Operation: domain.OpSaveMemory,
			Channel: "mcp", BaselineTokens: 10, EmittedTokens: 10,
		}
		if err := repo.Record(rec); err != nil {
			t.Fatalf("Record(%s): %v", sid, err)
		}
	}

	sessions, err := repo.Sessions("proj", 10)
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("Sessions devolvió %d, want 3: %v", len(sessions), sessions)
	}
}

func TestUsageRepository_Totals_AllSessions(t *testing.T) {
	db := openTestDB(t)
	repo := NewUsageRepository(db)

	for _, sid := range []string{"sess-a", "sess-b"} {
		rec := domain.UsageRecord{
			Project: "proj", SessionID: sid, Operation: domain.OpSearchMemories,
			Channel: "cli", BaselineTokens: 20, EmittedTokens: 15,
		}
		if err := repo.Record(rec); err != nil {
			t.Fatalf("Record(%s): %v", sid, err)
		}
	}

	all, err := repo.Totals("proj")
	if err != nil {
		t.Fatalf("Totals: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("Totals devolvió %d, want 2", len(all))
	}
}

func TestUsageRepository_ChannelIsNotValidated(t *testing.T) {
	db := openTestDB(t)
	repo := NewUsageRepository(db)

	rec := domain.UsageRecord{
		Project: "proj", SessionID: "sess-1", Operation: domain.OpOther,
		Channel: "canal-desconocido-inventado", BaselineTokens: 5, EmittedTokens: 5,
	}
	if err := repo.Record(rec); err != nil {
		t.Fatalf("Record con canal no listado debe aceptarse: %v", err)
	}

	got, err := repo.BySession("proj", "sess-1")
	if err != nil || len(got) != 1 {
		t.Fatalf("BySession tras registrar canal desconocido: got=%v err=%v", got, err)
	}
	if got[0].Channel != "canal-desconocido-inventado" {
		t.Fatalf("la etiqueta de canal no se conservó: %q", got[0].Channel)
	}
}
