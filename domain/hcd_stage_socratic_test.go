package domain_test

import (
	"mem/domain"
	"strings"
	"testing"
	"time"
)

func validFullDoc() string {
	return "## Intención\nValid intent\n## Alcance\nIncluido: guardado\nExcluido: exportación\n## ¿Qué pasa si no lo hacemos?\nnada\n## Alternativa más barata\nninguna\n## Historias\n### US1. Historia\nComo usuario, quiero funcionalidad, para avanzar\n- C-01: Dado x, cuando y, entonces z\n"
}

func TestHCD057SocraticGate(t *testing.T) {
	// A state with an active full-lane unit where definition has no pending fields
	// and no open clarifications. Without socratic review, Stage must be StageDefine
	// with NextAction directing to hcd-define to complete socratic examination.
	doc := validFullDoc()
	d, pending := domain.ParseUnitDoc([]byte(doc), domain.KindEvolution, domain.LaneFull)
	t.Logf("pending length: %d, pending: %+v", len(pending), pending)
	s := domain.HCDState{
		Unit:    domain.Unit{ID: "HCD-001", Kind: domain.KindEvolution, Lane: domain.LaneFull, State: domain.UnitActive},
		Doc:     d,
		Pending: pending,
	}

	if s.HasSocraticExam() {
		t.Fatal("expected no socratic exam initially")
	}

	if stage := s.Stage(); stage != domain.StageDefine {
		t.Fatalf("expected stage %s, got %s", domain.StageDefine, stage)
	}

	next := s.NextAction()
	if next.Skill != "hcd-define" {
		t.Fatalf("expected next skill hcd-define, got %s", next.Skill)
	}

	// Now add a valid socratic review event for the current definition revision
	rev := domain.HCDReviewRevision(s.DefinitionRevision())
	event := domain.HCDDefinitionReviewEvent{
		OperationID: "op-1",
		UnitID:      s.Unit.ID,
		Revision:    rev,
		Kind:        "socratic",
		Action:      "complete",
		Client:      "antigravity",
		CreatedAt:   time.Now().UTC(),
		Payload: domain.HCDDefinitionReviewPayload{
			Summary: "Valid exam summary",
		},
	}
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{event}

	if !s.HasSocraticExam() {
		t.Fatal("expected socratic exam to be recognized")
	}

	if stage := s.Stage(); stage != domain.StageApprove {
		t.Fatalf("expected stage %s after socratic exam, got %s", domain.StageApprove, stage)
	}
}

func TestHCD057SocraticRevision(t *testing.T) {
	// Test that changing definition revision (even editorial) invalidates the socratic exam,
	// but changes that normalize to the same revision keep it valid.
	doc1 := validFullDoc()
	d1, _ := domain.ParseUnitDoc([]byte(doc1), domain.KindEvolution, domain.LaneFull)
	s := domain.HCDState{
		Unit: domain.Unit{ID: "HCD-001", Kind: domain.KindEvolution, Lane: domain.LaneFull, State: domain.UnitActive},
		Doc:  d1,
	}

	rev1 := domain.HCDReviewRevision(s.DefinitionRevision())
	s.ReviewEvents = []domain.HCDDefinitionReviewEvent{
		{
			OperationID: "op-1",
			UnitID:      s.Unit.ID,
			Revision:    rev1,
			Kind:        "socratic",
			Action:      "complete",
			Client:      "antigravity",
			CreatedAt:   time.Now().UTC(),
			Payload: domain.HCDDefinitionReviewPayload{
				Summary: "Valid exam summary",
			},
		},
	}

	if !s.HasSocraticExam() {
		t.Fatal("expected exam valid for rev1")
	}

	// Normalized change: trailing whitespace or empty lines that do not affect DefinitionRevision
	docNormalized := validFullDoc() + "\n\n"
	dNorm, _ := domain.ParseUnitDoc([]byte(docNormalized), domain.KindEvolution, domain.LaneFull)
	s.Doc = dNorm
	if s.DefinitionRevision() != string(rev1) {
		t.Fatalf("expected normalized doc to have revision %s, got %s", rev1, s.DefinitionRevision())
	}
	if !s.HasSocraticExam() {
		t.Fatal("expected exam still valid when revision unchanged under normalization")
	}

	// Editorial change that alters definition revision
	docEditorial := strings.Replace(validFullDoc(), "Valid intent", "Valid intent editorial change", 1)
	dEdit, _ := domain.ParseUnitDoc([]byte(docEditorial), domain.KindEvolution, domain.LaneFull)
	s.Doc = dEdit
	if s.DefinitionRevision() == string(rev1) {
		t.Fatal("expected editorial change to change definition revision")
	}
	if s.HasSocraticExam() {
		t.Fatal("expected exam invalidated after revision change")
	}
}
