package domain

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestReduce(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	base := RefinementFlow{ID: "flow", OriginalPrompt: " original\n", ProposedPrompt: "proposed", Questions: []Question{{Text: "Q", Options: []string{"A", "B"}}}, LastStablePhase: PhaseCaptured}
	for _, tc := range []struct {
		name   string
		phase  SocraticaPhase
		ev     Event
		want   SocraticaPhase
		effect Effect
		bad    bool
	}{
		{"questions", PhaseCaptured, Event{Kind: "questions_generated", Questions: base.Questions}, PhaseQuestions, EffectNone, false},
		{"no questions", PhaseCaptured, Event{Kind: "questions_generated"}, PhaseRewriting, EffectGenerateRewrite, false},
		{"answer last", PhaseQuestions, Event{Kind: "answer", Answer: Answer{QuestionIndex: 0, Choice: "A"}}, PhaseRewriting, EffectGenerateRewrite, false},
		{"skip last", PhaseQuestions, Event{Kind: "skip"}, PhaseRewriting, EffectGenerateRewrite, false},
		{"skip rest", PhaseQuestions, Event{Kind: "skip_rest"}, PhaseRewriting, EffectGenerateRewrite, false},
		{"rewrite", PhaseRewriting, Event{Kind: "rewrite_ready", Proposed: "new"}, PhaseReview, EffectRecordHistory, false},
		{"empty rewrite", PhaseRewriting, Event{Kind: "rewrite_ready"}, "", EffectNone, true},
		{"edit", PhaseReview, Event{Kind: "edit", Proposed: "edited"}, PhaseReview, EffectRecordHistory, false},
		{"approve", PhaseReview, Event{Kind: "approve"}, PhaseApproved, EffectRecordHistory, false},
		{"duplicate", PhaseApproved, Event{Kind: "approve"}, PhaseApproved, EffectNone, false},
		{"dispatched duplicate", PhaseDispatched, Event{Kind: "approve"}, PhaseDispatched, EffectNone, false},
		{"model error", PhaseCaptured, Event{Kind: "model_error", Error: "failed"}, PhaseFailed, EffectNone, false},
		{"model error rewriting", PhaseRewriting, Event{Kind: "model_error", Error: "failed"}, PhaseFailed, EffectNone, false},
		{"retry captured", PhaseFailed, Event{Kind: "retry"}, PhaseCaptured, EffectGenerateQuestions, false},
		{"original", PhaseFailed, Event{Kind: "send_original"}, PhaseApproved, EffectRecordHistory, false},
		{"resume captured", PhaseCaptured, Event{Kind: "resume"}, PhaseCaptured, EffectGenerateQuestions, false},
		{"resume rewriting", PhaseRewriting, Event{Kind: "resume"}, PhaseRewriting, EffectGenerateRewrite, false},
		{"resume questions", PhaseQuestions, Event{Kind: "resume"}, PhaseQuestions, EffectNone, false},
		{"resume review", PhaseReview, Event{Kind: "resume"}, PhaseReview, EffectNone, false},
		{"resume approved", PhaseApproved, Event{Kind: "resume"}, PhaseReview, EffectNone, false},
		{"invalid", PhaseCaptured, Event{Kind: "approve"}, "", EffectNone, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := base
			f.Phase = tc.phase
			before := f
			got, e, err := Reduce(f, tc.ev, now, func() string { return "dispatch" })
			if !reflect.DeepEqual(f, before) {
				t.Fatal("mutated input")
			}
			if tc.bad {
				if !errors.Is(err, ErrInvalidTransition) {
					t.Fatalf("%v", err)
				}
				return
			}
			if err != nil || got.Phase != tc.want || e != tc.effect {
				t.Fatalf("got %s/%s/%v", got.Phase, e, err)
			}
			if got.OriginalPrompt != base.OriginalPrompt {
				t.Fatal("original changed")
			}
		})
	}
	for _, phase := range []SocraticaPhase{PhaseCaptured, PhaseQuestions, PhaseRewriting, PhaseReview, PhaseApproved, PhaseFailed} {
		f := base
		f.Phase = phase
		g, e, err := Reduce(f, Event{Kind: "cancel"}, now, func() string { return "id" })
		if err != nil || g.Phase != PhaseCancelled || g.ProposedPrompt != "" || e != EffectRestoreOriginal {
			t.Fatalf("cancel %s: %+v %s %v", phase, g, e, err)
		}
	}
	f := base
	f.Phase = PhaseApproved
	f.DispatchID = "d"
	f.ApprovedPrompt = "approved"
	if _, _, err := Reduce(f, Event{Kind: "ack", DispatchID: "other", Nonce: "n"}, now, func() string { return "id" }); err == nil {
		t.Fatal("foreign ack accepted")
	}
	g, _, err := Reduce(f, Event{Kind: "ack", DispatchID: "d", Nonce: "n"}, now, func() string { return "id" })
	if err != nil || !g.DispatchAck || g.Phase != PhaseDispatched {
		t.Fatal(g, err)
	}
	g, e, err := Reduce(f, Event{Kind: "ack", DispatchID: "d", Nonce: "n", Private: true}, now, func() string { return "id" })
	if err != nil || e != EffectRedactFlow {
		t.Fatal(g, e, err)
	}
	f.DispatchClaimOwner = "owner"
	for _, held := range []bool{true, false} {
		g, _, err = Reduce(f, Event{Kind: "resume", LockHeld: held}, now, func() string { return "id" })
		if err != nil || g.Phase != PhaseApproved || g.InFlight != held || g.Uncertain == held {
			t.Fatal(g, err)
		}
	}
	for _, kind := range []string{"cancel", "mark_dispatched", "force_resend"} {
		g, _, err = Reduce(f, Event{Kind: kind, LockHeld: true}, now, func() string { return "id" })
		if err != nil || !g.InFlight || g.Phase != PhaseApproved {
			t.Fatal(g, err)
		}
	}
	g, _, err = Reduce(f, Event{Kind: "force_resend"}, now, func() string { return "new" })
	if err != nil || g.DispatchID != "new" || g.DispatchClaimOwner != "" {
		t.Fatal(g, err)
	}
	f.Phase = PhaseFailed
	f.LastStablePhase = PhaseQuestions
	g, e, err = Reduce(f, Event{Kind: "retry"}, now, func() string { return "id" })
	if err != nil || g.Phase != PhaseRewriting || e != EffectGenerateRewrite {
		t.Fatal(g, e, err)
	}
	f.LastStablePhase = PhaseReview
	if _, _, err = Reduce(f, Event{Kind: "retry"}, now, func() string { return "id" }); err == nil {
		t.Fatal("invalid retry accepted")
	}

	f = base
	f.Phase = PhaseQuestions
	f.Questions = append(f.Questions, Question{Text: "second", Options: []string{"C", "D"}})
	g, e, err = Reduce(f, Event{Kind: "skip"}, now, func() string { return "id" })
	if err != nil || g.Phase != PhaseQuestions || e != EffectNone || !g.Answers[0].Skipped {
		t.Fatal(g, e, err)
	}
	f = base
	f.Phase = PhaseReview
	g, e, err = Reduce(f, Event{Kind: "approve"}, now, func() string { return "id" })
	if err != nil || g.DispatchID != "id" || g.ApprovedHash != SocraticaPromptHash("proposed") || g.LastStablePhase != PhaseApproved {
		t.Fatal(g, e, err)
	}
	g, e, err = Reduce(f, Event{Kind: "cancel", Private: true}, now, func() string { return "id" })
	if err != nil || e != EffectDeleteFlow {
		t.Fatal(g, e, err)
	}
	f.Phase = PhaseApproved
	f.ApprovedPrompt = "proposed"
	f.DispatchClaimOwner = "owner"
	f.DispatchID = "id"
	g, e, err = Reduce(f, Event{Kind: "mark_dispatched"}, now, func() string { return "id" })
	if err != nil || g.Phase != PhaseDispatched || !g.DispatchAck {
		t.Fatal(g, e, err)
	}
	g, e, err = Reduce(g, Event{Kind: "force_resend"}, now, func() string { return "new" })
	if err != nil || g.DispatchID != "new" || g.DispatchAck || g.AckNonce != "" {
		t.Fatal(g, e, err)
	}
	f.Phase = PhaseDispatched
	f.ApprovedPrompt = ""
	if _, _, err = Reduce(f, Event{Kind: "force_resend"}, now, func() string { return "id" }); err == nil {
		t.Fatal("resend accepted redacted prompt")
	}
}

// TestReduce_FilasDeDespacho cubre las filas de data-model.md que la tabla
// principal no comprueba campo a campo.
func TestReduce_FilasDeDespacho(t *testing.T) {
	now := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	id := func() string { return "new" }
	review := RefinementFlow{ID: "flow", OriginalPrompt: "original", ProposedPrompt: "proposed", Phase: PhaseReview, LastStablePhase: PhaseReview}

	t.Run("questions_generated", func(t *testing.T) {
		f := review
		f.Phase = PhaseCaptured
		qs := []Question{{Text: "a", Options: []string{"1", "2"}}, {Text: "b", Options: []string{"1", "2"}}, {Text: "c", Options: []string{"1", "2"}}}
		g, _, err := Reduce(f, Event{Kind: "questions_generated", Questions: qs, Scrutiny: &ScrutinySynthesis{Restatement: "r"}}, now, id)
		if err != nil || len(g.Questions) != 3 || g.LastStablePhase != PhaseQuestions || g.Scrutiny == nil {
			t.Fatal(g, err)
		}
		if _, _, err = Reduce(f, Event{Kind: "questions_generated", Questions: append(qs, qs[0])}, now, id); !errors.Is(err, ErrInvalidTransition) {
			t.Fatal("accepted 4 questions", err)
		}
	})
	t.Run("rewrite_ready and edit", func(t *testing.T) {
		f := review
		f.Phase = PhaseRewriting
		f.LastStablePhase = PhaseQuestions
		g, e, err := Reduce(f, Event{Kind: "rewrite_ready", Proposed: "p"}, now, id)
		if err != nil || g.LastStablePhase != PhaseReview || e != EffectRecordHistory || !g.UpdatedAt.Equal(now) {
			t.Fatal(g, e, err)
		}
		g, e, err = Reduce(g, Event{Kind: "edit", Proposed: "p2"}, now, id)
		if err != nil || g.Revision != 1 || g.ProposedPrompt != "p2" || e != EffectRecordHistory {
			t.Fatal(g, e, err)
		}
	})
	approved := review
	approved.Phase = PhaseApproved
	approved.ApprovedPrompt = "proposed"
	approved.DispatchID = "d"
	t.Run("approve duplicates", func(t *testing.T) {
		g, e, err := Reduce(approved, Event{Kind: "approve"}, now, id)
		if err != nil || !g.Duplicate || g.Acked || g.AlreadyDispatched || g.DispatchID != "d" || e != EffectNone {
			t.Fatal(g, e, err)
		}
		d := approved
		d.Phase = PhaseDispatched
		g, _, err = Reduce(d, Event{Kind: "approve"}, now, id)
		if err != nil || !g.Duplicate || !g.AlreadyDispatched || g.DispatchID != "d" {
			t.Fatal(g, err)
		}
	})
	t.Run("ack nonce", func(t *testing.T) {
		g, _, err := Reduce(approved, Event{Kind: "ack", DispatchID: "d", Nonce: "n"}, now, id)
		if err != nil || g.AckNonce != "n" || !g.Acked {
			t.Fatal(g, err)
		}
		same, _, err := Reduce(g, Event{Kind: "ack", DispatchID: "d", Nonce: "n"}, now, id)
		if err != nil || !same.Acked || same.AlreadyDispatched {
			t.Fatal(same, err)
		}
		other, _, err := Reduce(g, Event{Kind: "ack", DispatchID: "d", Nonce: "x"}, now, id)
		if err != nil || other.Acked || !other.AlreadyDispatched {
			t.Fatal(other, err)
		}
		private, e, err := Reduce(approved, Event{Kind: "ack", DispatchID: "d", Nonce: "n", Private: true}, now, id)
		if err != nil || e != EffectRedactFlow || private.DispatchID != "d" || private.ApprovedHash != approved.ApprovedHash || private.DispatchConsumed != approved.DispatchConsumed {
			t.Fatal(private, e, err)
		}
	})
	t.Run("model_error keeps the original", func(t *testing.T) {
		f := review
		f.Phase = PhaseRewriting
		f.LastStablePhase = PhaseQuestions
		g, _, err := Reduce(f, Event{Kind: "model_error", Error: "boom"}, now, id)
		if err != nil || g.OriginalPrompt != "original" || g.LastStablePhase != PhaseQuestions || g.LastError != "boom" {
			t.Fatal(g, err)
		}
	})
	t.Run("send_original records history", func(t *testing.T) {
		f := review
		f.Phase = PhaseFailed
		g, e, err := Reduce(f, Event{Kind: "send_original"}, now, id)
		if err != nil || e != EffectRecordHistory || g.ApprovedPrompt != "original" || g.DispatchID != "new" {
			t.Fatal(g, e, err)
		}
	})
	uncertain := approved
	uncertain.DispatchClaimOwner = "A"
	t.Run("uncertain exits", func(t *testing.T) {
		g, _, err := Reduce(uncertain, Event{Kind: "mark_dispatched"}, now, id)
		if err != nil || g.Phase != PhaseDispatched || !g.DispatchAck {
			t.Fatal(g, err)
		}
		g, _, err = Reduce(uncertain, Event{Kind: "force_resend"}, now, id)
		if err != nil || g.Phase != PhaseApproved || g.DispatchID != "new" || g.DispatchClaimOwner != "" || g.DispatchClaimedAt != nil || g.DispatchAck {
			t.Fatal(g, err)
		}
		g, e, err := Reduce(uncertain, Event{Kind: "cancel"}, now, id)
		if err != nil || g.Phase != PhaseCancelled || e != EffectRestoreOriginal {
			t.Fatal(g, e, err)
		}
		for _, kind := range []string{"approve", "edit"} {
			if _, _, err = Reduce(uncertain, Event{Kind: kind, Proposed: "x"}, now, id); !errors.Is(err, ErrInvalidTransition) {
				t.Fatal(kind, err)
			}
		}
	})
	t.Run("in flight changes nothing", func(t *testing.T) {
		for _, kind := range []string{"mark_dispatched", "force_resend", "cancel"} {
			g, e, err := Reduce(uncertain, Event{Kind: kind, LockHeld: true}, now, id)
			g.InFlight = false
			if err != nil || e != EffectNone || !reflect.DeepEqual(g, uncertain) {
				t.Fatal(kind, g, e, err)
			}
		}
	})
	t.Run("force_resend from dispatched", func(t *testing.T) {
		d := approved
		d.Phase = PhaseDispatched
		d.DispatchAck = true
		d.AckNonce = "n"
		d.DispatchConsumed = true
		g, _, err := Reduce(d, Event{Kind: "force_resend"}, now, id)
		if err != nil || g.Phase != PhaseApproved || g.DispatchID != "new" || g.AckNonce != "" || g.DispatchConsumed {
			t.Fatal(g, err)
		}
	})
	t.Run("transitions not listed", func(t *testing.T) {
		for _, tc := range []struct {
			phase SocraticaPhase
			kind  string
		}{{PhaseDispatched, "cancel"}, {PhaseCancelled, "resume"}, {PhaseReview, "ack"}, {PhaseQuestions, "rewrite_ready"}, {PhaseReview, "retry"}, {PhaseReview, "mark_dispatched"}, {PhaseReview, "unknown"}} {
			f := review
			f.Phase = tc.phase
			if _, _, err := Reduce(f, Event{Kind: tc.kind, Proposed: "x"}, now, id); !errors.Is(err, ErrInvalidTransition) {
				t.Fatal(tc, err)
			}
		}
	})
}
