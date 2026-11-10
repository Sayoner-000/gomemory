package domain_test

import (
	"mem/domain"
	"strings"
	"testing"
	"time"
)

func TestHCD057SplashGate(t *testing.T) {
	doc := validFullDoc()
	d, pending := domain.ParseUnitDoc([]byte(doc), domain.KindEvolution, domain.LaneFull)
	s := domain.HCDState{
		Unit:    domain.Unit{ID: "HCD-001", Kind: domain.KindEvolution, Lane: domain.LaneFull, State: domain.UnitActive},
		Doc:     d,
		Pending: pending,
	}

	rev := domain.HCDReviewRevision(s.DefinitionRevision())
	now := time.Now().UTC()

	// Add socratic exam
	socraticEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-soc-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "socratic",
		Action:      "complete",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload:     domain.HCDDefinitionReviewPayload{Summary: "Examen socratico valido"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv}

	// 1. Without splash decision, splash is not satisfied
	satisfied, reason := s.SplashSatisfied()
	if satisfied || !strings.Contains(reason, "splash") {
		t.Fatalf("expected splash decision required, got %v (%s)", satisfied, reason)
	}

	// 2. Skip splash satisfies requirement
	skipEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-skip-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		CreatedAt:   now.Add(time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "omito splash"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, skipEv}
	if sat, r := s.SplashSatisfied(); !sat {
		t.Fatalf("expected skip to satisfy splash requirement, got false: %s", r)
	}

	// 3. Accept alone does not satisfy
	acceptEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-accept-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		CreatedAt:   now.Add(2 * time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "acepto splash"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv}
	if sat, _ := s.SplashSatisfied(); sat {
		t.Fatal("expected accept alone not to satisfy splash")
	}

	// 4. Start attempt does not satisfy
	startEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-start-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		CreatedAt:   now.Add(3 * time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-1"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv, startEv}
	if sat, _ := s.SplashSatisfied(); sat {
		t.Fatal("expected in-progress start not to satisfy splash")
	}

	// 5. Fail attempt does not satisfy
	failEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-fail-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "fail",
		Client:      "antigravity",
		CreatedAt:   now.Add(4 * time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-1", Reason: "error en subproceso"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv, startEv, failEv}
	if sat, _ := s.SplashSatisfied(); sat {
		t.Fatal("expected fail not to satisfy splash")
	}

	// 6. Complete with un-dismissed Blocker does not satisfy
	completeEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-comp-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "complete",
		Client:      "antigravity",
		CreatedAt:   now.Add(5 * time.Second),
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-1",
			Summary:   "Informe con blocker y major",
			Findings: []domain.HCDReviewFinding{
				{ID: "B1", Severity: "Blocker", Problem: "Bloqueo critico"},
				{ID: "M1", Severity: "Major", Problem: "Aviso importante"},
			},
		},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv, startEv, completeEv}
	if sat, r := s.SplashSatisfied(); sat || !strings.Contains(r, "B1") {
		t.Fatalf("expected un-dismissed blocker B1 to block, got sat=%v reason=%s", sat, r)
	}

	// 7. Dismiss Blocker B1 satisfies requirement (Major M1 remains informative)
	dismissEv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-dism-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "dismiss",
		Client:      "antigravity",
		CreatedAt:   now.Add(6 * time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{FindingID: "B1", Quote: "descarto B1"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv, startEv, completeEv, dismissEv}
	if sat, r := s.SplashSatisfied(); !sat {
		t.Fatalf("expected dismissed blocker to satisfy splash, got false: %s", r)
	}

	// 8. Subsequent start (attempt 2) after complete/dismiss: must NOT satisfy splash
	startEv2 := domain.HCDDefinitionReviewEvent{
		OperationID: "op-start-2",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		CreatedAt:   now.Add(7 * time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-2"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv, startEv, completeEv, dismissEv, startEv2}
	if sat, _ := s.SplashSatisfied(); sat {
		t.Fatal("expected subsequent in-progress attempt not to satisfy splash")
	}

	// 9. Subsequent fail (attempt 2) after complete/dismiss: must NOT satisfy splash
	failEv2 := domain.HCDDefinitionReviewEvent{
		OperationID: "op-fail-2",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "splash",
		Action:      "fail",
		Client:      "antigravity",
		CreatedAt:   now.Add(8 * time.Second),
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-2", Reason: "error en reintento"},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{socraticEv, acceptEv, startEv, completeEv, dismissEv, startEv2, failEv2}
	if sat, _ := s.SplashSatisfied(); sat {
		t.Fatal("expected subsequent failed attempt not to satisfy splash")
	}
}

func TestHCD057SplashRevision(t *testing.T) {
	doc := validFullDoc()
	d, pending := domain.ParseUnitDoc([]byte(doc), domain.KindEvolution, domain.LaneFull)
	s := domain.HCDState{
		Unit:    domain.Unit{ID: "HCD-001", Kind: domain.KindEvolution, Lane: domain.LaneFull, State: domain.UnitActive},
		Doc:     d,
		Pending: pending,
	}

	rev := domain.HCDReviewRevision(s.DefinitionRevision())
	now := time.Now().UTC()
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{
		{
			OperationID: "op-soc-1",
			UnitID:      s.Unit.ID,
			Revision:    rev,
			Kind:        "socratic",
			Action:      "complete",
			Client:      "antigravity",
			CreatedAt:   now,
			Payload:     domain.HCDDefinitionReviewPayload{Summary: "Examen valido"},
		},
		{
			OperationID: "op-skip-1",
			UnitID:      s.Unit.ID,
			Revision:    rev,
			Kind:        "splash",
			Action:      "skip",
			Client:      "antigravity",
			CreatedAt:   now.Add(time.Second),
			Payload:     domain.HCDDefinitionReviewPayload{Quote: "omito splash"},
		},
	}

	if sat, _ := s.SplashSatisfied(); !sat {
		t.Fatal("expected splash satisfied for rev")
	}

	// Normalized change: keeps revision and keeps splash satisfied
	docNorm := validFullDoc() + "\n\n"
	dNorm, _ := domain.ParseUnitDoc([]byte(docNorm), domain.KindEvolution, domain.LaneFull)
	s.Doc = dNorm
	if s.DefinitionRevision() != string(rev) {
		t.Fatalf("expected normalized doc to have revision %s, got %s", rev, s.DefinitionRevision())
	}
	if sat, _ := s.SplashSatisfied(); !sat {
		t.Fatal("expected splash still satisfied under normalization")
	}

	// Editorial change: alters revision, invalidates splash
	docEdit := strings.Replace(validFullDoc(), "Valid intent", "Valid intent editorial splash", 1)
	dEdit, _ := domain.ParseUnitDoc([]byte(docEdit), domain.KindEvolution, domain.LaneFull)
	s.Doc = dEdit
	if s.DefinitionRevision() == string(rev) {
		t.Fatal("expected editorial change to alter revision")
	}
	if sat, _ := s.SplashSatisfied(); sat {
		t.Fatal("expected splash invalidated after revision change")
	}
}
