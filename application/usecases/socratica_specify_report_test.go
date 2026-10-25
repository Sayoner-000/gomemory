package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/domain"
)

func completedOutcome() domain.SpecifyOutcome {
	return domain.SpecifyOutcome{
		Status:        domain.OutcomeCompleted,
		SpecPath:      "specs/001-export/spec.md",
		ChecklistPath: "specs/001-export/checklists/requirements.md",
		Traceability:  []domain.SpecifyTrace{{CriterionID: "C1", StoryIDs: []string{"US1"}, SpecReference: "Acceptance Scenarios 1"}},
	}
}

// dispatchedSpecify returns a flow whose first ack was granted.
func dispatchedSpecify(t *testing.T) (*SocraticaFlow, *socraticaFakeRepo, string, string) {
	t.Helper()
	u, r, id, dispatch := approvedSpecify(t)
	if res, err := u.Apply(context.Background(), id, ackEvent(dispatch, "n1"), OriginMCP); err != nil || !res.Flow.ExecutionGranted {
		t.Fatalf("setup ack: %+v, %v", res.Flow, err)
	}
	return u, r, id, dispatch
}

func TestSpecifyReport_CompletedRecordsTheOutcomeWithoutGrantingAnything(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := dispatchedSpecify(t)
	res, err := u.ReportOutcome(ctx, id, dispatch, completedOutcome(), false)
	if err != nil {
		t.Fatal(err)
	}
	f := res.Flow
	o := f.Preparation.Outcome
	if o == nil || o.DispatchID != dispatch || o.Status != domain.OutcomeCompleted || o.SpecPath != "specs/001-export/spec.md" || len(o.Traceability) != 1 {
		t.Fatalf("outcome = %+v", o)
	}
	if f.Phase != domain.PhaseDispatched || f.ExecutionGranted || f.DispatchID != dispatch || f.DispatchConsumed || r.flow.Preparation.Outcome == nil {
		t.Fatalf("report changed the authorization: %+v", f)
	}
	if view := SpecifyView(f); view.Stage != domain.StageDelivery || contains(view.AllowedActions, "ack") || contains(view.AllowedActions, "approve") || !contains(view.AllowedActions, "report") {
		t.Fatalf("view = %+v", view)
	}
	if h := r.history[id]; h.Preparation == nil || h.Preparation.Outcome == nil {
		t.Fatalf("history snapshot lacks the outcome: %+v", h.Preparation)
	}
	if !strings.Contains(res.Next, "resultado") || !strings.Contains(res.Next, "no ejecutes") {
		t.Fatalf("next = %q", res.Next)
	}
	// Reporting never re-opens the consumed authorization.
	again, err := u.Apply(ctx, id, ackEvent(dispatch, "n2"), OriginMCP)
	if err != nil || again.Flow.ExecutionGranted || !again.Flow.AlreadyDispatched {
		t.Fatalf("ack after report = %+v, %v", again.Flow, err)
	}
}

func TestSpecifyReport_CompletedNeedsPathsAndTraceabilityForEveryCriterion(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := dispatchedSpecify(t)
	mutations := map[string]func(*domain.SpecifyOutcome){
		"no spec path":         func(o *domain.SpecifyOutcome) { o.SpecPath = "" },
		"no checklist path":    func(o *domain.SpecifyOutcome) { o.ChecklistPath = "  " },
		"no traceability":      func(o *domain.SpecifyOutcome) { o.Traceability = nil },
		"unknown criterion":    func(o *domain.SpecifyOutcome) { o.Traceability[0].CriterionID = "C9" },
		"unknown story":        func(o *domain.SpecifyOutcome) { o.Traceability[0].StoryIDs = []string{"US9"} },
		"no story":             func(o *domain.SpecifyOutcome) { o.Traceability[0].StoryIDs = nil },
		"no reference":         func(o *domain.SpecifyOutcome) { o.Traceability[0].SpecReference = " " },
		"duplicate criterion":  func(o *domain.SpecifyOutcome) { o.Traceability = append(o.Traceability, o.Traceability[0]) },
		"unsupported status":   func(o *domain.SpecifyOutcome) { o.Status = "done" },
		"wrong dispatch in it": func(o *domain.SpecifyOutcome) { o.DispatchID = "otro" },
	}
	for name, mutate := range mutations {
		o := completedOutcome()
		mutate(&o)
		if _, err := u.ReportOutcome(ctx, id, dispatch, o, false); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}
	if r.flow.Preparation.Outcome != nil {
		t.Fatal("a rejected report left an outcome")
	}
}

func TestSpecifyReport_PathsAreRelativeAndNeverEscape(t *testing.T) {
	ctx := context.Background()
	u, _, id, dispatch := dispatchedSpecify(t)
	for _, bad := range []string{"/etc/passwd", `\\servidor\recurso`, `C:\x\spec.md`, "../spec.md", "specs/../../spec.md", `specs\..\spec.md`, "specs//spec.md", "specs/spec.md\n", "specs/\x00spec.md", "https://x.test/spec.md", "~/spec.md", "./"} {
		o := completedOutcome()
		o.SpecPath = bad
		if _, err := u.ReportOutcome(ctx, id, dispatch, o, false); err == nil {
			t.Errorf("spec_path %q accepted", bad)
		}
		o = completedOutcome()
		o.ChecklistPath = bad
		if _, err := u.ReportOutcome(ctx, id, dispatch, o, false); err == nil {
			t.Errorf("checklist_path %q accepted", bad)
		}
	}
}

func TestSpecifyReport_FailedAndUncertainNeedAMessageNotArtifacts(t *testing.T) {
	ctx := context.Background()
	for _, status := range []string{domain.OutcomeFailed, domain.OutcomeUncertain} {
		u, _, id, dispatch := dispatchedSpecify(t)
		if _, err := u.ReportOutcome(ctx, id, dispatch, domain.SpecifyOutcome{Status: status}, false); err == nil {
			t.Errorf("%s without message accepted", status)
		}
		res, err := u.ReportOutcome(ctx, id, dispatch, domain.SpecifyOutcome{Status: status, Message: "falló \x1b[31mrojo\x1b[0m con AKIAIOSFODNN7EXAMPLE"}, false)
		if err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		msg := res.Flow.Preparation.Outcome.Message
		if strings.Contains(msg, "\x1b") || strings.Contains(msg, "AKIAIOSFODNN7EXAMPLE") || res.Flow.Preparation.Outcome.SpecPath != "" {
			t.Fatalf("%s message = %q", status, msg)
		}
	}
}

func TestSpecifyReport_DuplicatesAndExplicitCorrections(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := dispatchedSpecify(t)
	if _, err := u.ReportOutcome(ctx, id, dispatch, completedOutcome(), false); err != nil {
		t.Fatal(err)
	}
	same, err := u.ReportOutcome(ctx, id, dispatch, completedOutcome(), false)
	if err != nil || !same.Flow.Duplicate {
		t.Fatalf("identical report = duplicate %v, %v; want a no-op", same.Flow.Duplicate, err)
	}
	changed := completedOutcome()
	changed.SpecPath = "specs/001-export/other.md"
	if _, err = u.ReportOutcome(ctx, id, dispatch, changed, false); !errors.Is(err, domain.ErrOutcomeConflict) {
		t.Fatalf("different report without correction = %v", err)
	}
	fixed, err := u.ReportOutcome(ctx, id, dispatch, changed, true)
	if err != nil || fixed.Flow.Preparation.Outcome.SpecPath != "specs/001-export/other.md" || r.flow.Preparation.Outcome.SpecPath != "specs/001-export/other.md" {
		t.Fatalf("correction = %+v, %v", fixed.Flow.Preparation.Outcome, err)
	}
	if fixed.Flow.DispatchAck != true || fixed.Flow.ExecutionGranted || fixed.Flow.Phase != domain.PhaseDispatched {
		t.Fatal("a correction must not change the consumption")
	}
}

func TestSpecifyReport_OnlyForTheCurrentConsumedDispatch(t *testing.T) {
	ctx := context.Background()
	// Not consumed yet.
	u, _, id, dispatch := approvedSpecify(t)
	if _, err := u.ReportOutcome(ctx, id, dispatch, completedOutcome(), false); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("report before the ack = %v", err)
	}
	// Obsolete dispatch.
	u, _, id, dispatch = dispatchedSpecify(t)
	if _, err := u.ReportOutcome(ctx, id, "viejo", completedOutcome(), false); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("report of another dispatch = %v", err)
	}
	// A resend invalidates the old dispatch and the report that belonged to it.
	if _, err := u.ReportOutcome(ctx, id, dispatch, completedOutcome(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Apply(ctx, id, domain.Event{Kind: "force_resend", DispatchID: dispatch}, OriginMCP); err != nil {
		t.Fatal(err)
	}
	if _, err := u.ReportOutcome(ctx, id, dispatch, completedOutcome(), false); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("late report of the old dispatch = %v", err)
	}
	// Other modes have no outcome.
	refine, _ := socraticaFixture()
	if _, err := refine.ReportOutcome(ctx, "f", "d", completedOutcome(), false); !errors.Is(err, domain.ErrInvalidTransition) {
		t.Fatalf("report on refine = %v", err)
	}
}
