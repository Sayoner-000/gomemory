package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCD057SplashGate(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Splash Gate",
		Why:    "Validar gate de splash antes de aprobar",
		Intent: "Asegurar que splash se complete u omita antes de la aprobacion",
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

	// 1. Approve without splash decision is rejected
	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     view.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "splash") {
		t.Fatalf("expected splash decision required, got %v", err)
	}

	// 2. Accept splash alone: approve still rejected (incomplete)
	view, _, err = env.uc.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-acc-g",
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "acepto splash"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     view.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("expected approve rejected when splash incomplete, got %v", err)
	}

	// 3. Skip splash: now approve succeeds
	view, _, err = env.uc.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-skip-g",
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "omito splash"},
	})
	if err != nil {
		t.Fatal(err)
	}

	view, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     view.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if err != nil {
		t.Fatalf("approve failed after skip: %v", err)
	}
	if !view.Approved {
		t.Fatal("expected view approved after skip")
	}
}

func TestHCD057SplashRevision(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Splash Revision",
		Why:    "Validar invalidacion de splash ante cambio de revision",
		Intent: "Comprobar persistencia de splash y caducidad",
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

	// Skip splash
	view, _, err = env.uc.RecordDefinitionReview(ctx, usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-skip-rev",
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "omito splash"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Normalizing change preserves revision and allows approval
	editDoc(t, env, view, "Excluido: exportación\n", "Excluido: exportación\n\n")
	viewNorm, err := env.uc.Show(ctx, view.Unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if viewNorm.DefinitionRevision != string(rev) {
		t.Fatalf("expected same revision %s, got %s", rev, viewNorm.DefinitionRevision)
	}
	viewNorm, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     viewNorm.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if err != nil {
		t.Fatalf("expected approve to succeed under normalization, got %v", err)
	}

	// Editorial change invalidates review and approval
	editDoc(t, env, viewNorm, "Incluido: guardado", "Incluido: guardado y exportacion")
	viewEdit, err := env.uc.Show(ctx, viewNorm.Unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if viewEdit.DefinitionRevision == string(rev) {
		t.Fatal("expected revision change after editorial edit")
	}

	// Socratic exam is required first for new revision
	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     viewEdit.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("expected approve rejected on new revision without reviews, got %v", err)
	}

	// After completing socratic exam, splash decision is still required for the new revision
	viewEdit = completeSocraticExam(t, env, viewEdit)
	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     viewEdit.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "splash") {
		t.Fatalf("expected splash decision required on new revision, got %v", err)
	}
}
