package usecases_test

import (
	"context"
	"errors"
	"mem/application/usecases"
	"mem/domain"
	"strings"
	"testing"
)

func TestHCD057SocraticGate(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Socratic Gate",
		Why:    "Validar gate socratico antes de aprobar",
		Intent: "Asegurar que toda definicion pase por examen socratico",
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

	// Without socratic review, unit stays in define stage
	if view.Stage != domain.StageDefine {
		t.Fatalf("expected stage %s, got %s", domain.StageDefine, view.Stage)
	}
	if view.Next.Skill != "hcd-define" {
		t.Fatalf("expected next skill hcd-define, got %s", view.Next.Skill)
	}
	if view.State.HasSocraticExam() {
		t.Fatal("expected HasSocraticExam to be false")
	}

	// Attempting to approve without socratic exam is rejected
	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     view.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "examen socrático") {
		t.Fatalf("expected socratic examination error, got %v", err)
	}

	// Record socratic review
	view = completeSocraticExam(t, env, view)

	// Now stage must be approve
	if view.Stage != domain.StageApprove {
		t.Fatalf("expected stage %s after socratic review, got %s", domain.StageApprove, view.Stage)
	}
	if view.Next.Skill != "hcd-approve" {
		t.Fatalf("expected next skill hcd-approve, got %s", view.Next.Skill)
	}
	if !view.State.HasSocraticExam() {
		t.Fatal("expected HasSocraticExam to be true")
	}

	// Skip splash to satisfy approval requirement
	view = skipSplash(t, env, view)

	// Approve succeeds
	view, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     view.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if err != nil {
		t.Fatalf("approve failed after socratic review: %v", err)
	}
	if !view.Approved {
		t.Fatal("expected view to be approved")
	}

	// Second approve is rejected
	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     view.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) {
		t.Fatalf("expected already approved error, got %v", err)
	}
}

func TestHCD057SocraticRevision(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, err := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind:   "feature",
		Lane:   "full",
		Title:  "Socratic Revision",
		Why:    "Validar invalidacion editorial y vigencia normalizada",
		Intent: "Comprobar vigencia de examen ante cambios de revision",
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
	rev1 := view.DefinitionRevision

	// Normalizing change (trailing newline): does not change definition revision
	editDoc(t, env, view, "Excluido: exportación\n", "Excluido: exportación\n\n")
	viewNorm, err := env.uc.Show(ctx, view.Unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if viewNorm.DefinitionRevision != rev1 {
		t.Fatalf("expected same revision %s, got %s", rev1, viewNorm.DefinitionRevision)
	}
	if !viewNorm.State.HasSocraticExam() {
		t.Fatal("expected exam to stay valid when revision unchanged under normalization")
	}
	if viewNorm.Stage != domain.StageApprove {
		t.Fatalf("expected stage approve, got %s", viewNorm.Stage)
	}

	// Editorial change (changes definition revision): invalidates socratic exam
	editDoc(t, env, viewNorm, "Incluido: guardado", "Incluido: guardado editorial")
	viewEdit, err := env.uc.Show(ctx, view.Unit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if viewEdit.DefinitionRevision == rev1 {
		t.Fatal("expected revision to change after editorial edit")
	}
	if viewEdit.State.HasSocraticExam() {
		t.Fatal("expected socratic exam to be invalidated by editorial change")
	}
	if viewEdit.Stage != domain.StageDefine {
		t.Fatalf("expected stage to return to define, got %s", viewEdit.Stage)
	}

	// Attempting to approve the new revision without a new exam fails
	_, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     viewEdit.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if !errors.Is(err, domain.ErrNotAllowed) || !strings.Contains(err.Error(), "examen socrático") {
		t.Fatalf("expected socratic examination error on new revision, got %v", err)
	}

	// Recording exam for the new revision allows approval
	viewEdit = completeSocraticExam(t, env, viewEdit)
	viewEdit = skipSplash(t, env, viewEdit)
	if viewEdit.Stage != domain.StageApprove {
		t.Fatalf("expected stage approve after exam on new revision, got %s", viewEdit.Stage)
	}
	viewEdit, err = env.uc.Approve(ctx, usecases.HCDApproveInput{
		ID:     viewEdit.Unit.ID,
		Quote:  "apruebo",
		Client: "antigravity",
	})
	if err != nil {
		t.Fatalf("approve failed on new revision: %v", err)
	}
	if !viewEdit.Approved {
		t.Fatal("expected new revision to be approved")
	}
}
