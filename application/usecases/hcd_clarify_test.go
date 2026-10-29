package usecases_test

import (
	"context"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestClarifyOpenBlocksDefinitionAndAnswerRecordsDecision(t *testing.T) {
	uc, _, db, _, _ := newHCDFlow(t)
	unit, err := uc.Start(context.Background(), HCDStartInputForTest())
	if err != nil {
		t.Fatal(err)
	}
	clarification, err := uc.Clarify(context.Background(), usecases.HCDClarifyInput{UnitID: unit.ID, ExpectedRevision: unit.StateRevision, OperationID: "clarify-open", Action: "open", TargetRef: "definition", Question: "¿Cuál comportamiento se espera?", Options: []string{"A", "B"}, Recommendation: "A", Blocking: true})
	if err != nil {
		t.Fatal(err)
	}
	if clarification.ID != "CL-01" || clarification.Status != "open" || !clarification.Blocking {
		t.Fatalf("clarification=%+v", clarification)
	}
	definition := domain.Definition{ID: unit.ID, Kind: unit.Kind, Title: unit.Title, Implementer: unit.Implementer, Playbook: unit.PlaybookRev, Problem: "problema", Actor: "usuario", ExpectedResult: "resultado", Included: "x", Excluded: "y", Restrictions: "z", Stories: []domain.Story{{ID: "US1", Title: "Historia", Actor: "usuario", Text: "texto"}}, Criteria: []domain.Criterion{{ID: "C-01", Title: "criterio", Stories: []string{"US1"}, Condition: "condición", Verification: domain.VerificationAutomated}}}
	checks, err := uc.ValidateUnit(context.Background(), unit.ID, definition)
	if err != nil {
		t.Fatal(err)
	}
	if !hasHCDCheck(checks, "CLARIFICATION-OPEN") {
		t.Fatalf("open blocking clarification missing: %+v", checks)
	}
	updated, err := uc.Clarify(context.Background(), usecases.HCDClarifyInput{UnitID: unit.ID, ExpectedRevision: unit.StateRevision + 1, OperationID: "clarify-answer", Action: "answer", ClarificationID: clarification.ID, Choice: "B"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != "answered" || updated.Answer != "B" {
		t.Fatalf("answered=%+v", updated)
	}
	var decisions int
	if err := db.QueryRow("SELECT count(*) FROM hcd_decisions WHERE project=? AND unit_id=? AND kind='clarification_answer'", "project", unit.ID).Scan(&decisions); err != nil || decisions != 1 {
		t.Fatalf("clarification decisions=%d err=%v", decisions, err)
	}
}

func hasHCDCheck(checks []domain.PendingCheck, code string) bool {
	for _, check := range checks {
		if check.Code == code {
			return true
		}
	}
	return false
}
