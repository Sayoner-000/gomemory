package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"testing"
)

func TestHCD057SplashDismissal(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Splash Dismissal",
		Why:    "Validar descarte explicito de Blocker",
		Intent: "Probar descarte de Blocker y naturaleza informativa de Major/Nit",
		Client: "antigravity",
	})
	if err != nil {
		t.Fatal(err)
	}

	editDoc(t, env, view, append(commonFill, storyFill...)...)
	view, err = env.uc.Show(ctx, view.Unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	view = completeSocraticExam(t, env, view)
	rev := domain.HCDReviewRevision(view.DefinitionRevision)

	// Accept and start attempt att-1
	view, _, err = env.uc.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-accept-d",
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "acepto splash"},
	})
	if err != nil {
		t.Fatal(err)
	}

	view, _, err = env.uc.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-start-d",
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-d"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Complete splash with 1 Blocker, 1 Major, 1 Nit
	view, _, err = env.uc.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-complete-d",
		Kind:        "splash",
		Action:      "complete",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-d",
			Summary:   "Informe con blocker, major y nit",
			Findings: []domain.HCDReviewFinding{
				{
					ID:       "B1",
					Severity: "Blocker",
					Quote:    "Incluido: guardado",
					Problem:  "Riesgo critico de inconsistencia",
				},
				{
					ID:       "M1",
					Severity: "Major",
					Problem:  "Advertencia de rendimiento",
				},
				{
					ID:       "N1",
					Severity: "Nit",
					Problem:  "Detalle de estilo",
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// 1. Dismiss of nonexistent finding rejected
	badIDInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-dismiss-bad-id",
		Kind:        "splash",
		Action:      "dismiss",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			FindingID: "B99",
			Quote:     "descarto inexistente",
		},
	}
	if _, _, err := env.uc.RecordDefinitionReview(ctx, badIDInput); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on nonexistent finding, got %v", err)
	}

	// 2. Dismiss of Major or Nit finding rejected (only Blocker can be dismissed; Major/Nit are informative)
	dismissMajor := badIDInput
	dismissMajor.OperationID = "op-dismiss-major"
	dismissMajor.Payload.FindingID = "M1"
	if _, _, err := env.uc.RecordDefinitionReview(ctx, dismissMajor); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error dismissing Major finding, got %v", err)
	}

	dismissNit := badIDInput
	dismissNit.OperationID = "op-dismiss-nit"
	dismissNit.Payload.FindingID = "N1"
	if _, _, err := env.uc.RecordDefinitionReview(ctx, dismissNit); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error dismissing Nit finding, got %v", err)
	}

	// 3. Dismiss without quote rejected
	noQuoteDismiss := badIDInput
	noQuoteDismiss.OperationID = "op-dismiss-noquote"
	noQuoteDismiss.Payload.FindingID = "B1"
	noQuoteDismiss.Payload.Quote = "   "
	if _, _, err := env.uc.RecordDefinitionReview(ctx, noQuoteDismiss); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on empty quote in dismiss, got %v", err)
	}

	// 4. Valid dismiss of Blocker B1 succeeds
	validDismiss := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-dismiss-b1",
		Kind:        "splash",
		Action:      "dismiss",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			FindingID: "B1",
			Quote:     "descarto B1 porque la base usa bloqueo inmediato",
		},
	}
	view, dismissEv, err := env.uc.RecordDefinitionReview(ctx, validDismiss)
	if err != nil {
		t.Fatalf("valid dismiss failed: %v", err)
	}
	if dismissEv.Payload.FindingID != "B1" || dismissEv.Payload.Quote != validDismiss.Payload.Quote {
		t.Fatalf("unexpected dismiss event: %+v", dismissEv)
	}

	// 5. Replay of same dismiss is idempotent
	_, replayDismiss, err := env.uc.RecordDefinitionReview(ctx, validDismiss)
	if err != nil {
		t.Fatalf("replay dismiss failed: %v", err)
	}
	if replayDismiss.OperationID != dismissEv.OperationID || !replayDismiss.CreatedAt.Equal(dismissEv.CreatedAt) {
		t.Fatalf("expected exact replay, got %+v", replayDismiss)
	}

	// 6. Second dismiss of B1 with different operation is rejected (already dismissed)
	secondDismiss := validDismiss
	secondDismiss.OperationID = "op-dismiss-b1-second"
	if _, _, err := env.uc.RecordDefinitionReview(ctx, secondDismiss); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on already dismissed blocker, got %v", err)
	}
}
