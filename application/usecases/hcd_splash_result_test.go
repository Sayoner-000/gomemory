package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCD057SplashResult(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Splash Result",
		Why:    "Validar intento y resultado de splash",
		Intent: "Probar ciclo completo de intento y resultado splash",
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

	// 1. Result without attempt is rejected
	completeInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-complete-1",
		Kind:        "splash",
		Action:      "complete",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-1",
			Summary:   "Informe completo",
		},
	}
	if _, _, err := env.uc.RecordDefinitionReview(ctx, completeInput); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on complete without attempt, got %v", err)
	}

	// 2. Accept splash
	acceptInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-accept-1",
		Kind:        "splash",
		Action:      "accept",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "acepto la revisión splash"},
	}
	view, _, err = env.uc.RecordDefinitionReview(ctx, acceptInput)
	if err != nil {
		t.Fatalf("accept failed: %v", err)
	}

	// 3. Complete directly after accept without start is rejected
	if _, _, err := env.uc.RecordDefinitionReview(ctx, completeInput); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error on complete without start, got %v", err)
	}

	// 4. Start attempt att-1
	startInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-start-1",
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-1"},
	}
	view, _, err = env.uc.RecordDefinitionReview(ctx, startInput)
	if err != nil {
		t.Fatalf("start att-1 failed: %v", err)
	}

	// 5. Fail attempt att-1 with reason
	failInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-fail-1",
		Kind:        "splash",
		Action:      "fail",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-1",
			Reason:    "timeout del subproceso reviewer",
		},
	}
	view, failEv, err := env.uc.RecordDefinitionReview(ctx, failInput)
	if err != nil {
		t.Fatalf("fail att-1 failed: %v", err)
	}
	if failEv.Payload.Reason != failInput.Payload.Reason {
		t.Fatalf("unexpected fail reason: %s", failEv.Payload.Reason)
	}

	// 6. Attempting to complete already finished att-1 is rejected
	if _, _, err := env.uc.RecordDefinitionReview(ctx, completeInput); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected error completing finished attempt, got %v", err)
	}

	// 7. Start attempt att-2 and complete with structured findings and privacy protection
	startInput2 := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-start-2",
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-2"},
	}
	view, _, err = env.uc.RecordDefinitionReview(ctx, startInput2)
	if err != nil {
		t.Fatalf("start att-2 failed: %v", err)
	}

	completeInput2 := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-complete-2",
		Kind:        "splash",
		Action:      "complete",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-2",
			Summary:   "Informe splash con <private>secret_token</private> protegido",
			Findings: []domain.HCDReviewFinding{
				{
					ID:       "B1",
					Severity: "Blocker",
					Quote:    "Incluido: guardado",
					Problem:  "Riesgo de bloqueo sin comprobacion",
					Question: "¿Se asegura atomicidad?",
				},
				{
					ID:       "M1",
					Severity: "Major",
					Problem:  "Manejo de errores mejorable",
				},
			},
		},
	}
	view, compEv, err := env.uc.RecordDefinitionReview(ctx, completeInput2)
	if err != nil {
		t.Fatalf("complete att-2 failed: %v", err)
	}
	if strings.Contains(compEv.Payload.Summary, "secret_token") {
		t.Fatal("expected private token to be protected in summary")
	}
	if len(compEv.Payload.Findings) != 2 || compEv.Payload.Findings[0].Severity != "Blocker" {
		t.Fatalf("unexpected findings: %+v", compEv.Payload.Findings)
	}

	// 8. Result posterior a omisión: start attempt att-3, then skip, then try complete
	startInput3 := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-start-3",
		Kind:        "splash",
		Action:      "start",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{AttemptID: "att-3"},
	}
	view, _, err = env.uc.RecordDefinitionReview(ctx, startInput3)
	if err != nil {
		t.Fatalf("start att-3 failed: %v", err)
	}

	skipInput := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-skip-after-start",
		Kind:        "splash",
		Action:      "skip",
		Client:      "antigravity",
		Payload:     domain.HCDDefinitionReviewPayload{Quote: "omito splash tras demora"},
	}
	view, _, err = env.uc.RecordDefinitionReview(ctx, skipInput)
	if err != nil {
		t.Fatalf("skip after start failed: %v", err)
	}

	completeInput3 := usecases.HCDDefinitionReviewInput{
		ID:          view.Unit.ID,
		Revision:    rev,
		OperationID: "op-complete-3",
		Kind:        "splash",
		Action:      "complete",
		Client:      "antigravity",
		Payload: domain.HCDDefinitionReviewPayload{
			AttemptID: "att-3",
			Summary:   "Informe tardio",
		},
	}
	if _, _, err := env.uc.RecordDefinitionReview(ctx, completeInput3); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("expected validation error completing attempt after skip, got %v", err)
	}
}
