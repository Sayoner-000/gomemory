package domain

import (
	"errors"
	"strings"
	"testing"
)

// dispatchedFlow is an approved specify flow whose first ack was granted.
func dispatchedFlow(t *testing.T) RefinementFlow {
	t.Helper()
	f := approvedFlow(t)
	return step(t, f, Event{Kind: "ack", DispatchID: f.DispatchID, Nonce: "n1"})
}

func goodOutcome() SpecifyOutcome {
	return SpecifyOutcome{
		Status: OutcomeCompleted, SpecPath: "specs/001/spec.md", ChecklistPath: "specs/001/checklists/requirements.md",
		Traceability: []SpecifyTrace{
			{CriterionID: "C1", StoryIDs: []string{"US1"}, SpecReference: "Escenario 1"},
			{CriterionID: "C2", StoryIDs: []string{"US1"}, SpecReference: "Escenario 2"},
		},
	}
}

func reportEvent(f RefinementFlow, o SpecifyOutcome, correct bool) Event {
	return Event{Kind: "report", DispatchID: f.DispatchID, Outcome: o, CorrectPrevious: correct}
}

func TestValidRelativePath(t *testing.T) {
	for _, ok := range []string{"specs/001/spec.md", "a", "docs/guia v2/a.md", "specs/001-exportar/checklists/requirements.md"} {
		if !validRelativePath(ok) {
			t.Errorf("%q rejected", ok)
		}
	}
	for _, bad := range []string{
		"", " specs/a.md", "specs/a.md ", "/etc/passwd", "~/a.md", `C:\a.md`, `specs\a.md`, "https://x.test/a.md", "a:b",
		"../a.md", "a/../b", "a/./b", "./a", "a//b", "a/", "a\nb", "a\x00b", "a\x7fb",
		"specs/clave AKIAIOSFODNN7EXAMPLE/spec.md",
	} {
		if validRelativePath(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestReportOutcome_CompletedIsRecordedAndNeverGrants(t *testing.T) {
	f := dispatchedFlow(t)
	got, effect, err := specifyReduce(t, f, reportEvent(f, goodOutcome(), false))
	if err != nil || effect != EffectRecordHistory || got.Duplicate {
		t.Fatalf("report = effect %s duplicate %v, %v", effect, got.Duplicate, err)
	}
	o := got.Preparation.Outcome
	if o == nil || o.DispatchID != f.DispatchID || o.Status != OutcomeCompleted || len(o.Traceability) != 2 || got.Preparation.Stage != StageDelivery {
		t.Fatalf("outcome = %+v stage %s", o, got.Preparation.Stage)
	}
	if got.Phase != PhaseDispatched || !got.DispatchAck || got.DispatchID != f.DispatchID || got.ExecutionGranted || got.Acked || got.DispatchConsumed {
		t.Fatalf("reporting changed the authorization: %+v", got)
	}
	if f.Preparation.Outcome != nil {
		t.Fatal("Reduce mutated the flow it received")
	}
}

func TestReportOutcome_DuplicatesCorrectionsAndConflicts(t *testing.T) {
	f := step(t, dispatchedFlow(t), reportEvent(dispatchedFlow(t), goodOutcome(), false))
	same, effect, err := specifyReduce(t, f, reportEvent(f, goodOutcome(), false))
	if err != nil || !same.Duplicate || effect != EffectNone {
		t.Fatalf("identical repeat = duplicate %v effect %s, %v", same.Duplicate, effect, err)
	}
	changed := goodOutcome()
	changed.SpecPath = "specs/001/otro.md"
	if _, _, err = specifyReduce(t, f, reportEvent(f, changed, false)); !errors.Is(err, ErrOutcomeConflict) {
		t.Fatalf("different outcome = %v", err)
	}
	fixed, _, err := specifyReduce(t, f, reportEvent(f, changed, true))
	if err != nil || fixed.Preparation.Outcome.SpecPath != "specs/001/otro.md" || fixed.DispatchID != f.DispatchID || !fixed.DispatchAck {
		t.Fatalf("correction = %+v, %v", fixed.Preparation.Outcome, err)
	}
}

func TestReportOutcome_ValidatesStatusPathsAndTraceability(t *testing.T) {
	f := dispatchedFlow(t)
	cases := map[string]func(*SpecifyOutcome){
		"unknown status":         func(o *SpecifyOutcome) { o.Status = "done" },
		"no spec path":           func(o *SpecifyOutcome) { o.SpecPath = "" },
		"no checklist path":      func(o *SpecifyOutcome) { o.ChecklistPath = "" },
		"escaping spec path":     func(o *SpecifyOutcome) { o.SpecPath = "../spec.md" },
		"absolute checklist":     func(o *SpecifyOutcome) { o.ChecklistPath = "/tmp/c.md" },
		"unknown criterion":      func(o *SpecifyOutcome) { o.Traceability[0].CriterionID = "C9" },
		"repeated criterion":     func(o *SpecifyOutcome) { o.Traceability[1].CriterionID = "C1" },
		"criterion left out":     func(o *SpecifyOutcome) { o.Traceability = o.Traceability[:1] },
		"no stories":             func(o *SpecifyOutcome) { o.Traceability[0].StoryIDs = nil },
		"unknown story":          func(o *SpecifyOutcome) { o.Traceability[0].StoryIDs = []string{"US9"} },
		"blank reference":        func(o *SpecifyOutcome) { o.Traceability[0].SpecReference = " " },
		"other dispatch in body": func(o *SpecifyOutcome) { o.DispatchID = "otro" },
	}
	for name, mutate := range cases {
		o := goodOutcome()
		o.Traceability = append([]SpecifyTrace(nil), o.Traceability...)
		mutate(&o)
		if _, _, err := specifyReduce(t, f, reportEvent(f, o, false)); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	for _, status := range []string{OutcomeFailed, OutcomeUncertain} {
		if _, _, err := specifyReduce(t, f, reportEvent(f, SpecifyOutcome{Status: status}, false)); err == nil {
			t.Errorf("%s without a message accepted", status)
		}
		got, _, err := specifyReduce(t, f, reportEvent(f, SpecifyOutcome{Status: status, Message: "falló \x1b[31mrojo\x1b[0m AKIAIOSFODNN7EXAMPLE"}, false))
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if m := got.Preparation.Outcome.Message; strings.Contains(m, "\x1b") || strings.Contains(m, "AKIAIOSFODNN7EXAMPLE") {
			t.Fatalf("%s message = %q", status, m)
		}
	}
	// Optional artifacts of a failure are still validated when given.
	if _, _, err := specifyReduce(t, f, reportEvent(f, SpecifyOutcome{Status: OutcomeFailed, Message: "x", SpecPath: "../x"}, false)); err == nil {
		t.Error("an escaping optional path was accepted")
	}
}

func TestReportOutcome_OnlyForTheConsumedDispatchOfASpecifyFlow(t *testing.T) {
	approved := approvedFlow(t)
	sent := dispatchedFlow(t)
	noPrep := sent
	noPrep.Preparation = nil
	future := sent
	future.Preparation = sent.Preparation.Clone()
	future.Preparation.SchemaVersion = 99
	staleID := reportEvent(sent, goodOutcome(), false)
	staleID.DispatchID = "viejo"
	noID := reportEvent(sent, goodOutcome(), false)
	noID.DispatchID = ""
	refine := RefinementFlow{Mode: ModeRefine, Phase: PhaseDispatched, DispatchAck: true, DispatchID: "d"}
	cases := []struct {
		name string
		f    RefinementFlow
		ev   Event
		want error
	}{
		{"not consumed", approved, reportEvent(approved, goodOutcome(), false), ErrInvalidTransition},
		{"other dispatch", sent, staleID, ErrInvalidTransition},
		{"no dispatch id", sent, noID, ErrInvalidTransition},
		{"refine flow", refine, reportEvent(refine, goodOutcome(), false), ErrInvalidTransition},
		{"redacted preparation", noPrep, reportEvent(noPrep, goodOutcome(), false), ErrPrivateStateUnavailable},
		{"newer preparation", future, reportEvent(future, goodOutcome(), false), ErrUnsupportedPreparation},
	}
	for _, c := range cases {
		if _, _, err := specifyReduce(t, c.f, c.ev); !errors.Is(err, c.want) {
			t.Errorf("%s = %v, want %v", c.name, err, c.want)
		}
	}
}

func TestForceResendSpecify_NewAuthorizationOnlyForMatchingContent(t *testing.T) {
	f := step(t, dispatchedFlow(t), reportEvent(dispatchedFlow(t), goodOutcome(), false))
	got, _, err := Reduce(f, Event{Kind: "force_resend", DispatchID: f.DispatchID}, specifyNow, func() string { return "dispatch-2" })
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != PhaseApproved || got.DispatchID == f.DispatchID || got.DispatchAck || got.AckNonce != "" || got.Preparation.Outcome != nil || got.ApprovedPrompt != f.ApprovedPrompt || got.ExecutionGranted {
		t.Fatalf("resend = %+v", got)
	}
	stale := f
	stale.Preparation = f.Preparation.Clone()
	stale.Preparation.Content.Intent.Objective = "[REDACTED:key]"
	if _, _, err = specifyReduce(t, stale, Event{Kind: "force_resend", DispatchID: f.DispatchID}); !errors.Is(err, ErrReviewStale) {
		t.Fatalf("resend over changed content = %v", err)
	}
	blank := f
	blank.ApprovedPrompt = ""
	if _, _, err = specifyReduce(t, blank, Event{Kind: "force_resend", DispatchID: f.DispatchID}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("resend without an approved text = %v", err)
	}
}

func TestPrivateSpecify_AckAndMarkDispatchedKeepTheEphemeralPayload(t *testing.T) {
	ephemeral := approvedFlow(t)
	ephemeral.PayloadPolicy = PayloadEphemeral
	got, effect, err := specifyReduce(t, ephemeral, Event{Kind: "ack", DispatchID: ephemeral.DispatchID, Nonce: "n", Private: true})
	if err != nil || effect != EffectNone {
		t.Fatalf("ack of an ephemeral flow = effect %s, %v; the payload must stay for the report", effect, err)
	}
	if got.Preparation == nil || got.ApprovedPrompt == "" {
		t.Fatal("the payload was dropped")
	}
	// The policy is fixed at creation: a persistent flow toggled to private
	// keeps its payload, so the outcome can still be reported (C-001, ACR 042).
	persistent := approvedFlow(t)
	acked, effect, err := specifyReduce(t, persistent, Event{Kind: "ack", DispatchID: persistent.DispatchID, Nonce: "n", Private: true})
	if err != nil || effect != EffectNone || acked.Preparation == nil {
		t.Fatalf("ack of a persistent flow toggled to private = effect %s, %v; want its payload kept", effect, err)
	}
	if reported := step(t, acked, reportEvent(acked, goodOutcome(), false)); reported.Preparation.Outcome == nil {
		t.Fatal("the outcome of a persistent flow toggled to private was not recorded")
	}
	claimed := approvedFlow(t)
	claimed.DispatchClaimOwner = "o"
	claimed.PayloadPolicy = PayloadEphemeral
	if _, effect, err = specifyReduce(t, claimed, Event{Kind: "mark_dispatched", Private: true}); err != nil || effect != EffectNone {
		t.Fatalf("mark_dispatched of an ephemeral flow = %s, %v", effect, err)
	}
}

func TestReduce_ReservationOnlyCancels(t *testing.T) {
	f := approvedFlow(t)
	f.PrivateStateUnavailable = true
	for _, kind := range []string{"resume", "approve", "ack", "report", "force_resend", "prepare_set", "prepare_validate", "skip", "cancelx"} {
		if _, _, err := specifyReduce(t, f, Event{Kind: kind}); !errors.Is(err, ErrPrivateStateUnavailable) {
			t.Errorf("%s = %v, want private_state_unavailable", kind, err)
		}
	}
	got, effect, err := specifyReduce(t, f, Event{Kind: "cancel"})
	if err != nil || got.Phase != PhaseCancelled || effect != EffectRestoreOriginal || got.PrivateStateUnavailable {
		t.Fatalf("cancel of a reservation = %s %s, %v", got.Phase, effect, err)
	}
}

// With both paths missing the error always names spec_path first (C-002, ACR 042).
func TestReportOutcome_ErrorFieldIsDeterministic(t *testing.T) {
	f := dispatchedFlow(t)
	o := goodOutcome()
	o.SpecPath, o.ChecklistPath = "", ""
	for range 50 {
		if _, _, err := specifyReduce(t, f, reportEvent(f, o, false)); err == nil || !strings.Contains(err.Error(), "spec_path") {
			t.Fatalf("error = %v, want spec_path", err)
		}
	}
}

// A dispatched private reservation without its payload only admits an explicit
// cancellation: report and resend need the private text (C-001, ACR 042 r2).
func TestDispatchedReservationWithoutPayload_OnlyCancels(t *testing.T) {
	f := dispatchedFlow(t)
	f.PayloadPolicy = PayloadEphemeral
	f.OriginalPrompt, f.ApprovedPrompt, f.Preparation = "", "", nil
	f.PrivateStateUnavailable = true
	for _, ev := range []Event{reportEvent(f, goodOutcome(), false), {Kind: "force_resend", DispatchID: f.DispatchID}} {
		if _, _, err := specifyReduce(t, f, ev); !errors.Is(err, ErrPrivateStateUnavailable) {
			t.Errorf("%s = %v, want private_state_unavailable", ev.Kind, err)
		}
	}
	if got := step(t, f, Event{Kind: "cancel"}); got.Phase != PhaseCancelled {
		t.Fatalf("cancel = %s, want cancelled", got.Phase)
	}
	f.PrivateStateUnavailable = false
	if _, _, err := specifyReduce(t, f, Event{Kind: "cancel"}); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("cancel of a dispatched flow with payload = %v, want invalid", err)
	}
}
