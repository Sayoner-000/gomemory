package usecases

import (
	"context"
	"errors"
	"testing"

	"mem/application/ports"
	"mem/domain"
)

// reopened builds a fresh use case over the same stored state, as a new process
// would after the previous one closed.
func reopened(u *SocraticaFlow) *SocraticaFlow {
	n := 0
	return &SocraticaFlow{Repo: u.Repo, UOW: u.UOW, Clock: u.Clock, Lock: &socraticaFakeLock{}, NewID: func() string { n++; return "reopened-" + string(rune('a'+n)) }}
}

func TestSpecifyRecovery_PersistentFlowKeepsDecisionsRelationsAndBudget(t *testing.T) {
	ctx := context.Background()
	u, r, id := startedSpecify(t)
	setAll(t, u, id)
	q := []domain.Question{{Text: "¿Formato?", Options: []string{"CSV", "JSON"}}}
	if _, err := u.Apply(ctx, id, domain.Event{Kind: "questions_generated", Questions: q}, OriginMCP); err != nil {
		t.Fatal(err)
	}
	if _, err := u.Apply(ctx, id, domain.Event{Kind: "answer", Answer: domain.Answer{QuestionIndex: 0, Choice: "CSV"}}, OriginMCP); err != nil {
		t.Fatal(err)
	}
	for _, s := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope} {
		if _, err := u.PrepareValidate(ctx, id, 3, s); err != nil {
			t.Fatal(err)
		}
	}
	before := r.flow

	next := reopened(u)
	got, err := next.Get(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	f := got.Flow
	if f.Revision != before.Revision || f.Preparation.Stage != before.Preparation.Stage || len(f.Questions) != 1 || len(f.Answers) != 1 || f.Answers[0].Choice != "CSV" {
		t.Fatalf("recovered = rev %d stage %s questions %d answers %d", f.Revision, f.Preparation.Stage, len(f.Questions), len(f.Answers))
	}
	if f.Preparation.Content.Stories == nil || len(f.Preparation.Checks) != 2 || SpecifyView(f).QuestionsUsed != 1 {
		t.Fatalf("relations or checks lost: %+v", f.Preparation)
	}
	// Resuming and continuing neither resets the budget nor repeats work.
	resumed, err := next.Resume(ctx, id, OriginMCP)
	if err != nil || len(resumed.Flow.Questions) != 1 {
		t.Fatalf("resume = %+v, %v", resumed.Flow.Questions, err)
	}
	for _, s := range []domain.SpecifyStage{domain.StageStories, domain.StageValidation} {
		if _, err = next.PrepareValidate(ctx, id, 3, s); err != nil {
			t.Fatal(err)
		}
	}
	if r.flow.Phase != domain.PhaseReview {
		t.Fatalf("phase = %s", r.flow.Phase)
	}
}

func TestSpecifyRecovery_ValidApprovalSurvivesAndStaleOneDoesNot(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := approvedSpecify(t)
	res, err := reopened(u).Resume(ctx, id, OriginMCP)
	if err != nil || res.Flow.Phase != domain.PhaseApproved || res.Flow.DispatchID != dispatch || !res.Flow.Recovered || res.Flow.ExecutionGranted {
		t.Fatalf("resume of a valid approval = %+v, %v", res.Flow, err)
	}
	r.flow.Preparation.Content.Stories.Stories[0].Need = "[REDACTED:key]"
	res, err = reopened(u).Resume(ctx, id, OriginMCP)
	if err != nil || res.Flow.Phase != domain.PhaseCaptured || res.Flow.DispatchID != "" || res.Flow.ApprovedPrompt != "" {
		t.Fatalf("resume of a stale approval = %+v, %v", res.Flow, err)
	}
}

func TestSpecifyRecovery_UnsupportedPreparationIsKeptAndOnlyCancellable(t *testing.T) {
	ctx := context.Background()
	u, r, id := startedSpecify(t)
	setAll(t, u, id)
	r.flow.Preparation.SchemaVersion = 99
	got, err := u.Get(ctx, id)
	if err != nil || got.Flow.Preparation.SchemaVersion != 99 {
		t.Fatalf("read of a newer preparation = %v", err)
	}
	for name, run := range map[string]func() error{
		"set":      func() error { _, e := u.PrepareSet(ctx, id, 3, domain.StageIntent, []byte(intentJSON)); return e },
		"validate": func() error { _, e := u.PrepareValidate(ctx, id, 3, domain.StageIntent); return e },
	} {
		if err := run(); !errors.Is(err, domain.ErrUnsupportedPreparation) {
			t.Errorf("%s = %v, want ErrUnsupportedPreparation", name, err)
		}
	}
	if _, err = u.ApproveSpecify(ctx, id, 3); err == nil {
		t.Error("approve over a newer preparation succeeded")
	}
	if r.flow.Preparation.SchemaVersion != 99 || r.flow.Revision != 3 {
		t.Fatalf("a refused mutation changed the stored preparation: %+v", r.flow.Preparation)
	}
	res, err := u.Apply(ctx, id, domain.Event{Kind: "cancel"}, OriginMCP)
	if err != nil || res.Flow.Phase != domain.PhaseCancelled || res.Flow.Preparation == nil || res.Flow.Preparation.SchemaVersion != 99 {
		t.Fatalf("cancel = %+v, %v; the slot must be freed without destroying the newer data", res.Flow.Preparation, err)
	}
}

func TestSpecifyRecovery_ProjectsDoNotSeeEachOther(t *testing.T) {
	ctx := context.Background()
	u, _, _ := startedSpecify(t)
	other, err := u.Pending(ctx, "otro-proyecto")
	if err != nil || len(other) != 0 {
		t.Fatalf("pending of another project = %v, %v", other, err)
	}
	mine, err := u.Pending(ctx, "p")
	if err != nil || len(mine) != 1 || mine[0].Flow.Mode != domain.ModeSpecify {
		t.Fatalf("pending = %v, %v", mine, err)
	}
}

// A reservation without payload (another instance, or this one after a restart)
// only reads and cancels; it never rebuilds content nor an authorization.
func TestSpecifyRecovery_ReservationWithoutPayloadOnlyReadsAndCancels(t *testing.T) {
	ctx := context.Background()
	u, r, id, dispatch := approvedSpecify(t)
	r.flow.PrivateStateUnavailable = true
	r.flow.PayloadPolicy = domain.PayloadEphemeral
	got, err := u.Get(ctx, id)
	if err != nil || !got.Flow.PrivateStateUnavailable || got.Flow.ExecutionGranted {
		t.Fatalf("get = %+v, %v", got.Flow, err)
	}
	if v := SpecifyView(got.Flow); len(v.AllowedActions) != 2 || v.AllowedActions[0] != "get" || v.AllowedActions[1] != "cancel" {
		t.Fatalf("actions = %v", v.AllowedActions)
	}
	for name, ev := range map[string]domain.Event{
		"set":          {Kind: "prepare_set", Section: domain.StageIntent, Content: domain.SpecifyContent{Intent: &domain.SpecifyIntent{Objective: "x"}}},
		"validate":     {Kind: "prepare_validate", Section: domain.StageIntent},
		"approve":      {Kind: "approve", ExpectedRevision: 3},
		"ack":          {Kind: "ack", DispatchID: dispatch, Nonce: "n"},
		"resume":       {Kind: "resume"},
		"force_resend": {Kind: "force_resend", DispatchID: dispatch},
		"report":       {Kind: "report", DispatchID: dispatch, Outcome: domain.SpecifyOutcome{Status: domain.OutcomeFailed, Message: "x"}},
		"answer":       {Kind: "skip"},
		"questions":    {Kind: "questions_generated"},
	} {
		if _, err := u.Apply(ctx, id, ev, OriginMCP); !errors.Is(err, domain.ErrPrivateStateUnavailable) {
			t.Errorf("%s = %v, want private_state_unavailable", name, err)
		}
	}
	if r.flow.DispatchAck || r.flow.Phase != domain.PhaseApproved {
		t.Fatalf("a refused operation changed the reservation: %+v", r.flow)
	}
	res, err := u.Apply(ctx, id, domain.Event{Kind: "cancel"}, OriginMCP)
	if err != nil || res.Flow.Phase != domain.PhaseCancelled {
		t.Fatalf("cancel of the reservation = %+v, %v", res.Flow, err)
	}
}

// The generic local route never runs a specify flow, whatever its phase.
func TestSpecifyRecovery_LocalExecuteNeverDispatchesSpecify(t *testing.T) {
	ctx := context.Background()
	for _, setup := range []func(t *testing.T) (*SocraticaFlow, string){
		func(t *testing.T) (*SocraticaFlow, string) { u, _, id := startedSpecify(t); return u, id },
		func(t *testing.T) (*SocraticaFlow, string) { u, _, id := reviewedSpecify(t); return u, id },
		func(t *testing.T) (*SocraticaFlow, string) { u, _, id, _ := approvedSpecify(t); return u, id },
	} {
		u, id := setup(t)
		launcher := &socraticaFakeLauncher{repo: u.Repo.(*socraticaFakeRepo)}
		_, err := u.Execute(ctx, id, "codex", launcher)
		if !errors.Is(err, ErrSpecifyAgentOnly) || launcher.calls != 0 {
			t.Fatalf("Execute = %v (launches %d); a specify flow is delivered by the agent", err, launcher.calls)
		}
	}
	var _ ports.AgentLauncher = (*socraticaFakeLauncher)(nil)
}

// The policy fixed at creation, not today's setting, decides about history.
func TestSpecifyRecovery_EphemeralFlowRecordsNoHistoryWhateverTheSettingSays(t *testing.T) {
	ctx := context.Background()
	u, r, id := reviewedSpecify(t)
	r.flow.PayloadPolicy = domain.PayloadEphemeral
	u.Settings.Private = false
	if _, err := u.ApproveSpecify(ctx, id, 3); err != nil {
		t.Fatal(err)
	}
	if r.historyCalls != 0 {
		t.Fatalf("history was recorded %d times for an ephemeral flow", r.historyCalls)
	}
}
