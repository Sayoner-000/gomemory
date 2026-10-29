package usecases_test

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestRecordDecisionPersistsControlStateWithoutMemoryWrite(t *testing.T) {
	uc, _, db, _, _ := newHCDFlow(t)
	unit, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	decision, err := uc.RecordDecision(context.Background(), unit.ID, "scope", "proposed", "investigate compatibility", unit.DefinitionRev, []string{"US1"}, "conversation")
	if err != nil {
		t.Fatal(err)
	}
	if decision.ID != "D-02" || decision.Status != "proposed" || decision.MemoryID != 0 || decision.MemorySync != "pending" {
		t.Fatalf("decision=%+v", decision)
	}
	var count int
	if err := db.QueryRow("SELECT count(*) FROM hcd_decisions WHERE project=? AND unit_id=?", "project", unit.ID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("decision rows=%d, want start + explicit decision; err=%v", count, err)
	}
	var memories int
	if err := db.QueryRow("SELECT count(*) FROM memories WHERE project=? AND topic_key LIKE ?", "project", "hcd:%").Scan(&memories); err != nil || memories != 0 {
		t.Fatalf("phase 2 must not mirror decisions to memory: count=%d err=%v", memories, err)
	}
}

func HCDStartInputForTest() usecases.HCDStartInput {
	return usecases.HCDStartInput{Kind: domain.KindFix, Title: "Guardar datos", Implementer: "dev", Client: "claude", SessionID: "session-1"}
}
