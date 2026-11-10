package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCD057SplashDecision(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Splash Decision",
		Why:    "Validar registro de decision splash",
		Intent: "Aceptar u omitir splash con cita literal",
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
	rev := domain.HCDReviewRevision(view.DefinitionRevision)

	// 1. Attempt splash decision before socratic exam is rejected
	acceptInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-splash-accept-1",
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "acepto la revisión splash"},
	}
	if _, _, err := env.uc.RecordDefinitionReview(ctx, acceptInput); !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "socratic") {
		t.Fatalf("expected socratic required before splash decision, got %v", err)
	}

	// Complete socratic exam
	view = completeSocraticExam(t, env, view)

	// 2. Reject splash decision with wrong revision
	badRevInput := acceptInput
	badRevInput.Revision = "deadbeef1234"
	if _, _, err := env.uc.RecordDefinitionReview(ctx, badRevInput); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on mismatched revision, got %v", err)
	}

	// 3. Reject splash decision without quote
	noQuoteInput := acceptInput
	noQuoteInput.Payload.Quote = "   "
	if _, _, err := env.uc.RecordDefinitionReview(ctx, noQuoteInput); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on missing quote, got %v", err)
	}

	// 4. Record accept with literal quote succeeds
	view, acceptEv, err := env.uc.RecordDefinitionReview(ctx, acceptInput)
	if err != nil {
		t.Fatalf("record accept failed: %v", err)
	}
	if acceptEv.Kind != "splash" || acceptEv.Action != "accept" || acceptEv.Payload.Quote != acceptInput.Payload.Quote {
		t.Fatalf("unexpected accept event: %+v", acceptEv)
	}

	// 5. Replay of same accept operation is idempotent
	_, replayEv, err := env.uc.RecordDefinitionReview(ctx, acceptInput)
	if err != nil {
		t.Fatalf("replay accept failed: %v", err)
	}
	if replayEv.OperationID != acceptEv.OperationID || !replayEv.CreatedAt.Equal(acceptEv.CreatedAt) {
		t.Fatalf("expected exact replay, got %+v", replayEv)
	}

	// 6. Record skip after accept: succeeds and history retains prior accept
	skipInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-splash-skip-1",
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "cambio de opinión: omito splash"},
	}
	view, skipEv, err := env.uc.RecordDefinitionReview(ctx, skipInput)
	if err != nil {
		t.Fatalf("record skip failed: %v", err)
	}
	if skipEv.Action != "skip" {
		t.Fatalf("expected skip action, got %s", skipEv.Action)
	}

	// Verify history contains both accept and skip events
	var hasAccept, hasSkip bool
	for _, e := range view.State.ReviewEvents {
		if e.Revision == rev && e.Kind == "splash" {
			if e.Action == "accept" {
				hasAccept = true
			}
			if e.Action == "skip" {
				hasSkip = true
			}
		}
	}
	if !hasAccept || !hasSkip {
		t.Fatalf("expected history to retain both accept and skip events: %+v", view.State.ReviewEvents)
	}

	// 7. Replay of skip operation is idempotent
	_, replaySkip, err := env.uc.RecordDefinitionReview(ctx, skipInput)
	if err != nil {
		t.Fatalf("replay skip failed: %v", err)
	}
	if replaySkip.OperationID != skipEv.OperationID || !replaySkip.CreatedAt.Equal(skipEv.CreatedAt) {
		t.Fatalf("expected exact replay of skip, got %+v", replaySkip)
	}
}
