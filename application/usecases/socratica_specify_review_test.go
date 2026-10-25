package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

// reviewedSpecify returns a use case with a specify flow in review (revision 3).
func reviewedSpecify(t *testing.T) (*SocraticaFlow, *socraticaFakeRepo, string) {
	t.Helper()
	u, r, id := startedSpecify(t)
	setAll(t, u, id)
	for _, s := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories, domain.StageValidation} {
		if _, err := u.PrepareValidate(context.Background(), id, 3, s); err != nil {
			t.Fatal(err)
		}
	}
	if r.flow.Phase != domain.PhaseReview {
		t.Fatalf("setup: %s", r.flow.Phase)
	}
	return u, r, id
}

func TestSpecifyApprove_BindsTheDispatchToTheReviewedText(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	reviewed := r.flow.ProposedPrompt
	res, err := u.ApproveSpecify(ctx, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	f := res.Flow
	if f.Phase != domain.PhaseApproved || f.ApprovedPrompt != reviewed || f.DispatchID == "" || f.ExecutionGranted || f.Duplicate {
		t.Fatalf("approve = phase %s dispatch %q granted %v", f.Phase, f.DispatchID, f.ExecutionGranted)
	}
	if r.flow.Phase != domain.PhaseApproved || r.flow.ApprovedPrompt != reviewed || r.flow.Preparation.Stage != domain.StageApproved || r.flow.ApprovedHash != domain.SocraticaPromptHash(reviewed) {
		t.Fatalf("approval was not saved with the transition: %+v", r.flow)
	}
	if !strings.Contains(res.Next, "speckit-specify") || !strings.Contains(res.Next, "NO llames a ack") || !strings.Contains(res.Next, "ack_nonce") {
		t.Fatalf("next must carry the preflight before ack: %q", res.Next)
	}
	// Repeated approval keeps the dispatch and never grants execution.
	again, err := u.ApproveSpecify(ctx, id, 3)
	if err != nil || !again.Flow.Duplicate || again.Flow.DispatchID != f.DispatchID || again.Flow.ExecutionGranted {
		t.Fatalf("repeat = %+v, %v", again.Flow, err)
	}
	got, _ := u.Get(ctx, id)
	if got.Flow.ExecutionGranted {
		t.Fatal("get must never grant execution")
	}
}

func TestSpecifyApprove_StaleRevisionAndConcurrentEditNeverApprove(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	if _, err := u.ApproveSpecify(ctx, id, 2); !errors.Is(err, domain.ErrRevisionConflict) || r.flow.Phase != domain.PhaseReview {
		t.Fatalf("stale approval = %v, phase %s", err, r.flow.Phase)
	}
	// An edit lands between the person's review and the approval.
	edited := strings.Replace(storiesJSON, "Exportar mi historial", "Exportar mi historial completo", 1)
	if _, err := u.PrepareSet(ctx, id, 3, domain.StageStories, []byte(edited)); err != nil {
		t.Fatal(err)
	}
	if _, err := u.ApproveSpecify(ctx, id, 3); !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("approving the version that was shown, after an edit = %v", err)
	}
	if _, err := u.ApproveSpecify(ctx, id, 4); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("approving the edited draft without a new review = %v", err)
	}
	if r.flow.ApprovedPrompt != "" || r.flow.DispatchID != "" {
		t.Fatalf("something got approved: %+v", r.flow)
	}
}

func TestSpecifyApprove_BypassRoutesStayClosed(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	for _, ev := range []domain.Event{
		{Kind: "approve"}, // no expected revision: the revision is 3, not 0
		{Kind: "send_original"},
		{Kind: "edit", Proposed: "texto libre"},
		{Kind: "rewrite_ready", Proposed: "texto libre"},
		{Kind: "ack", DispatchID: "inventado", Nonce: "n"},
	} {
		if _, err := u.Apply(ctx, id, ev, OriginMCP); err == nil {
			t.Errorf("%s was accepted", ev.Kind)
		}
	}
	if r.flow.Phase != domain.PhaseReview || r.flow.ApprovedPrompt != "" || r.flow.ProposedPrompt == "free" {
		t.Fatalf("a bypass changed the flow: %+v", r.flow)
	}
}

func TestSpecifyApprove_SanitisedContentInvalidatesTheReview(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	// Privacy maintenance rewrote a field in storage after the review.
	r.flow.Preparation.Content.Stories.Stories[0].Need = "[REDACTED:key]"
	if _, err := u.ApproveSpecify(ctx, id, 3); !errors.Is(err, domain.ErrReviewStale) {
		t.Fatalf("approve over sanitised content = %v, want ErrReviewStale", err)
	}
	if r.flow.Phase != domain.PhaseReview || r.flow.ApprovedPrompt != "" {
		t.Fatal("a stale review must not approve")
	}
	// After approval, the first ack re-checks too.
	u2, r2, id2 := reviewedSpecify(t)
	approved, err := u2.ApproveSpecify(ctx, id2, 3)
	if err != nil {
		t.Fatal(err)
	}
	r2.flow.Preparation.Content.Stories.Stories[0].Need = "[REDACTED:key]"
	_, err = u2.Apply(ctx, id2, domain.Event{Kind: "ack", DispatchID: approved.Flow.DispatchID, Nonce: "n1"}, OriginMCP)
	if !errors.Is(err, domain.ErrReviewStale) || !errors.Is(err, ports.ErrAckRejected) || r2.flow.Phase != domain.PhaseApproved {
		t.Fatalf("ack over sanitised content = %v, phase %s", err, r2.flow.Phase)
	}
}

func TestSpecifyApprove_AckGrantsOnceAndNeverFromReads(t *testing.T) {
	ctx := context.Background()
	u, _, id := reviewedSpecify(t)
	approved, err := u.ApproveSpecify(ctx, id, 3)
	if err != nil {
		t.Fatal(err)
	}
	ack := domain.Event{Kind: "ack", DispatchID: approved.Flow.DispatchID, Nonce: "n1"}
	first, err := u.Apply(ctx, id, ack, OriginMCP)
	if err != nil || !first.Flow.ExecutionGranted || first.Flow.Phase != domain.PhaseDispatched {
		t.Fatalf("first ack = %+v, %v", first.Flow, err)
	}
	for _, read := range []func() (SocraticaResult, error){
		func() (SocraticaResult, error) { return u.Apply(ctx, id, ack, OriginMCP) },
		func() (SocraticaResult, error) { return u.Get(ctx, id) },
		func() (SocraticaResult, error) { return u.ApproveSpecify(ctx, id, 3) },
	} {
		res, err := read()
		if err != nil || res.Flow.ExecutionGranted {
			t.Fatalf("a replay or read granted execution again: %+v, %v", res.Flow.ExecutionGranted, err)
		}
	}
}

func TestSpecifyCancel_ReturnsTheOriginalAndWithdrawsEverything(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	if _, err := u.ApproveSpecify(ctx, id, 3); err != nil {
		t.Fatal(err)
	}
	res, err := u.Apply(ctx, id, domain.Event{Kind: "cancel"}, OriginMCP)
	if err != nil {
		t.Fatal(err)
	}
	f := res.Flow
	if f.Phase != domain.PhaseCancelled || f.OriginalPrompt != "exportar historial" || f.ApprovedPrompt != "" || f.Preparation != nil || f.DispatchID != "" {
		t.Fatalf("cancel = %+v", f)
	}
	if r.flow.Preparation != nil || r.flow.ApprovedPrompt != "" || r.flow.Phase != domain.PhaseCancelled {
		t.Fatalf("cancel was not saved: %+v", r.flow)
	}
}

func TestSpecifyApprove_RecordsHistoryWithThePreparationSnapshot(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	u.Settings.HistoryLimit = nil
	if _, err := u.ApproveSpecify(ctx, id, 3); err != nil {
		t.Fatal(err)
	}
	h, ok := r.history[id]
	if !ok || h.Mode != domain.ModeSpecify || h.Preparation == nil || h.Preparation.Review == nil || h.ApprovedPrompt != r.flow.ApprovedPrompt {
		t.Fatalf("history = %+v (present %v)", h, ok)
	}
}

func TestSpecifyView_ActionsAfterApprovalAndDispatch(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*domain.RefinementFlow)
		actions string
	}{
		{"review", func(f *domain.RefinementFlow) { f.Phase = domain.PhaseReview }, "get,resume,prepare,approve,cancel"},
		{"approved", func(f *domain.RefinementFlow) { f.Phase = domain.PhaseApproved }, "get,resume,prepare,ack,cancel"},
		{"claimed", func(f *domain.RefinementFlow) { f.Phase, f.DispatchClaimOwner = domain.PhaseApproved, "o" }, "get,force_resend,mark_dispatched,cancel"},
		{"dispatched", func(f *domain.RefinementFlow) { f.Phase, f.DispatchAck = domain.PhaseDispatched, true }, "get,report,force_resend"},
	}
	for _, c := range cases {
		f := domain.RefinementFlow{Mode: domain.ModeSpecify, Preparation: domain.NewSpecifyPreparation()}
		c.mutate(&f)
		if got := strings.Join(SpecifyView(f).AllowedActions, ","); got != c.actions {
			t.Errorf("%s: actions = %s, want %s", c.name, got, c.actions)
		}
	}
}
