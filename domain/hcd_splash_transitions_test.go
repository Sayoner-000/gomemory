package domain_test

import (
	"mem/domain"
	"strings"
	"testing"
	"time"
)

func TestHCD057SplashTransitions(t *testing.T) {
	now := time.Now().UTC()
	rev := domain.HCDReviewRevision("a1fa639ae55a")

	// 1. Validation of individual splash events
	// Accept requires quote
	acceptEvent := domain.HCDDefinitionReviewEvent{
		OperationID: "op-accept",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "acepto la revisión splash"},
	}
	if err := acceptEvent.Validate(); err != nil {
		t.Fatalf("expected acceptEvent valid, got %v", err)
	}
	badAccept := acceptEvent
	badAccept.Payload.Quote = "   "
	if err := badAccept.Validate(); err == nil {
		t.Fatal("expected empty quote rejected in accept")
	}

	// Skip requires quote
	skipEvent := domain.HCDDefinitionReviewEvent{
		OperationID: "op-skip",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "omito splash por alcance acotado"},
	}
	if err := skipEvent.Validate(); err != nil {
		t.Fatalf("expected skipEvent valid, got %v", err)
	}

	// Start requires attempt_id
	startEvent := domain.HCDDefinitionReviewEvent{
		OperationID: "op-start",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-1"},
	}
	if err := startEvent.Validate(); err != nil {
		t.Fatalf("expected startEvent valid, got %v", err)
	}
	badStart := startEvent
	badStart.Payload.AttemptID = ""
	if err := badStart.Validate(); err == nil {
		t.Fatal("expected empty attempt_id rejected in start")
	}

	// Complete requires attempt_id, summary, and valid findings
	completeEvent := domain.HCDDefinitionReviewEvent{
		OperationID: "op-complete",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "complete",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-1",
			Summary:   "Informe splash completado sin observaciones mayores",
			Findings: []domain.HCDReviewFinding{
				{
					ID:       "B1",
					Severity: "Blocker",
					Quote:    "Incluido: guardado",
					Problem:  "Falta considerar transaccion concurrente",
					Question: "¿Se asegura aislamiento inmediato?",
				},
				{
					ID:       "M1",
					Severity: "Major",
					Problem:  "Formato de fecha podria desalinearse",
				},
				{
					ID:       "N1",
					Severity: "Nit",
					Problem:  "Nombre de variable ambiguo",
				},
			},
		},
	}
	if err := completeEvent.Validate(); err != nil {
		t.Fatalf("expected completeEvent valid, got %v", err)
	}
	badFinding := completeEvent
	badFinding.Payload.Findings = []domain.HCDReviewFinding{
		{ID: "X1", Severity: "Unknown", Problem: "prob"},
	}
	if err := badFinding.Validate(); err == nil {
		t.Fatal("expected invalid severity rejected")
	}

	// Dismiss requires finding_id and quote
	dismissEvent := domain.HCDDefinitionReviewEvent{
		OperationID: "op-dismiss",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "dismiss",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload: domain.HCDDefinitionReviewPayload{
			FindingID: "B1",
			Quote:     "descarto B1 porque el DSN ya configura bloqueo inmediato",
		},
	}
	if err := dismissEvent.Validate(); err != nil {
		t.Fatalf("expected dismissEvent valid, got %v", err)
	}
	badDismiss := dismissEvent
	badDismiss.Payload.FindingID = ""
	if err := badDismiss.Validate(); err == nil {
		t.Fatal("expected empty finding_id rejected in dismiss")
	}

	// 2. Out-of-order transition rejections
	// start without accept
	if err := startEvent.ValidateTransition(nil); err == nil {
		t.Fatal("expected start without accept rejected")
	}

	// complete without start
	if err := completeEvent.ValidateTransition([]domain.HCDDefinitionReviewEvent{acceptEvent}); err == nil {
		t.Fatal("expected complete without start rejected")
	}

	// complete after skip
	if err := completeEvent.ValidateTransition([]domain.HCDDefinitionReviewEvent{skipEvent}); err == nil {
		t.Fatal("expected complete after skip rejected")
	}

	// dismiss without complete
	if err := dismissEvent.ValidateTransition([]domain.HCDDefinitionReviewEvent{acceptEvent, startEvent}); err == nil {
		t.Fatal("expected dismiss without complete rejected")
	}

	// dismiss of non-blocker (e.g. M1)
	dismissMajor := dismissEvent
	dismissMajor.Payload.FindingID = "M1"
	if err := dismissMajor.ValidateTransition([]domain.HCDDefinitionReviewEvent{acceptEvent, startEvent, completeEvent}); err == nil {
		t.Fatal("expected dismiss of Major finding rejected (only Blocker can be dismissed)")
	}

	// 3. Valid transition sequence
	history := []domain.HCDDefinitionReviewEvent{}
	if err := acceptEvent.ValidateTransition(history); err != nil {
		t.Fatalf("accept transition: %v", err)
	}
	history = append(history, acceptEvent)

	if err := startEvent.ValidateTransition(history); err != nil {
		t.Fatalf("start transition: %v", err)
	}
	history = append(history, startEvent)

	if err := completeEvent.ValidateTransition(history); err != nil {
		t.Fatalf("complete transition: %v", err)
	}
	history = append(history, completeEvent)

	if err := dismissEvent.ValidateTransition(history); err != nil {
		t.Fatalf("dismiss transition: %v", err)
	}
	history = append(history, dismissEvent)

	// 3b. Sequence skip -> accept -> start -> complete -> dismiss (H-02 / C-002)
	skipFirstHistory := []domain.HCDDefinitionReviewEvent{skipEvent, acceptEvent, startEvent}
	if err := completeEvent.ValidateTransition(skipFirstHistory); err != nil {
		t.Fatalf("expected complete allowed after skip->accept->start, got: %v", err)
	}
	skipFirstHistory = append(skipFirstHistory, completeEvent)
	if err := dismissEvent.ValidateTransition(skipFirstHistory); err != nil {
		t.Fatalf("expected dismiss allowed after skip->accept->start->complete, got: %v", err)
	}

	// However, if skip is after start/complete, new complete/dismiss must be rejected
	postSkipHistory := append(skipFirstHistory, domain.HCDDefinitionReviewEvent{
		OperationID: "op-skip-2",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "re-skip tras intento"},
	})
	if err := completeEvent.ValidateTransition(postSkipHistory); err == nil {
		t.Fatal("expected complete rejected when active decision is skip")
	}
	if err := dismissEvent.ValidateTransition(postSkipHistory); err == nil {
		t.Fatal("expected dismiss rejected when active decision is skip")
	}

	// 4. Privacy protection
	withPriv := domain.HCDDefinitionReviewEvent{
		OperationID: "op-priv",
		UnitID:      "HCD-001",
		Revision:    rev,
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		CreatedAt:   now,
		Payload: domain.HCDDefinitionReviewPayload{
			Quote: "cita con <private>clave_secreta</private> privada",
		},
	}.Protected()
	if strings.Contains(withPriv.Payload.Quote, "clave_secreta") {
		t.Fatal("expected private quote to be protected")
	}
}
