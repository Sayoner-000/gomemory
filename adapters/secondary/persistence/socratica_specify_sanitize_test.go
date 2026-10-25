package persistence

import (
	"context"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

func syntheticContent() domain.SpecifyContent {
	return domain.SpecifyContent{
		Intent: &domain.SpecifyIntent{Objective: "Exportar el historial propio", ExpectedResult: "Un archivo", Actors: []domain.SpecifyActor{{ID: "A1", Name: "Titular"}}},
		Scope: &domain.SpecifyScope{
			Included:    []string{"Exportar el historial"},
			Excluded:    domain.DeclaredList{Declared: true, Items: []string{"Historiales ajenos"}},
			Constraints: domain.DeclaredList{Declared: true, Items: []string{}},
			Assumptions: domain.DeclaredList{Declared: true, Items: []string{}},
			Pending:     domain.PendingList{Declared: true, Items: []domain.SpecifyPending{}},
		},
		Stories: &domain.SpecifyStories{
			Stories:    []domain.SpecifyStory{{ID: "US1", ActorID: "A1", Need: "Exportar mi historial", Outcome: "Consultarlo fuera"}},
			Criteria:   []domain.SpecifyCriterion{{ID: "C1", StoryIDs: []string{"US1"}, Condition: "Tiene registros", ExpectedResult: "El archivo los contiene"}},
			Deliveries: []domain.SpecifyDelivery{{ID: "D1", Title: "Exportación", Outcome: "Historial exportable", StoryIDs: []string{"US1"}, CriterionIDs: []string{"C1"}}},
		},
	}
}

// storedSpecifyFlow stores a specify flow driven by the real reducer up to the
// given point: "review", "approved" or "dispatched".
func storedSpecifyFlow(t *testing.T, r *SocraticaStore, id, upTo string) domain.RefinementFlow {
	t.Helper()
	now := time.Unix(1000, 0).UTC()
	newID := func() string { return "dispatch-" + id }
	f := domain.RefinementFlow{ID: id, SchemaVersion: 1, Project: "p", Client: "codex", Mode: domain.ModeSpecify, OriginalPrompt: "necesidad original",
		Phase: domain.PhaseCaptured, LastStablePhase: domain.PhaseCaptured, PayloadPolicy: domain.PayloadPersistent, Preparation: domain.NewSpecifyPreparation(), CreatedAt: now, UpdatedAt: now}
	f.Preparation.Content = syntheticContent()
	events := []domain.Event{}
	for _, s := range []domain.SpecifyStage{domain.StageIntent, domain.StageScope, domain.StageStories, domain.StageValidation} {
		events = append(events, domain.Event{Kind: "prepare_validate", Section: s})
	}
	if upTo == "approved" || upTo == "dispatched" {
		events = append(events, domain.Event{Kind: "approve"})
	}
	if upTo == "dispatched" {
		events = append(events, domain.Event{Kind: "ack", DispatchID: "dispatch-" + id, Nonce: "n1"})
	}
	var err error
	for _, ev := range events {
		ev.ExpectedRevision = f.Revision
		if f, _, err = domain.Reduce(f, ev, now, newID); err != nil {
			t.Fatalf("%s: %v", ev.Kind, err)
		}
	}
	if err = r.CreateFlow(context.Background(), f); err != nil {
		t.Fatal(err)
	}
	return f
}

// leak injects an unprotected private region into the stored preparation, as
// a row written before the uniform protection would have.
func leak(t *testing.T, r *SocraticaStore, table, id string) {
	t.Helper()
	col := "id"
	if table == "socratica_history" {
		col = "flow_id"
	}
	res, err := r.db.Exec("UPDATE "+table+" SET preparation_json=replace(preparation_json,'Exportar el historial propio','Exportar <private>valor-oculto-042</private> el historial propio') WHERE "+col+"=?", id)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("leak touched %d rows", n)
	}
}

func sanitizeAll(t *testing.T, r *SocraticaStore) int {
	t.Helper()
	ctx := context.Background()
	got, err := readSanitizeCandidates(ctx, r.db, "p")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()
	p := &privacyTx{tx: tx, project: "p"}
	for _, c := range got {
		if err := c.Validate("p"); err != nil {
			t.Fatal(err)
		}
		if _, err := p.Apply(ctx, c, domain.ProtectText(c.Text)); err != nil {
			t.Fatal(c.Category, err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return len(got)
}

func TestSanitizeSpecify_CoversThePreparationOfFlowsAndHistory(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := storedSpecifyFlow(t, r, "flow", "review")
	if err := r.UpsertHistory(ctx, domain.RefinementHistory{ID: "flow", FlowID: "flow", Project: "p", Client: "codex", Mode: domain.ModeSpecify, OriginalPrompt: "o", Preparation: f.Preparation, CompletedAt: time.Unix(1, 0)}, 5); err != nil {
		t.Fatal(err)
	}
	leak(t, r, "socratica_flows", "flow")
	leak(t, r, "socratica_history", "flow")
	got, err := readSanitizeCandidates(ctx, r.db, "p")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[domain.SanitizeCategory]bool{}
	for _, c := range got {
		seen[c.Category] = true
	}
	if !seen["socratica_flow_preparation"] || !seen["socratica_history_preparation"] {
		t.Fatalf("candidates lack the preparation categories: %v", seen)
	}
	sanitizeAll(t, r)
	if left, _ := readSanitizeCandidates(ctx, r.db, "p"); len(left) != 0 {
		t.Fatalf("residues remain: %d", len(left))
	}
	for _, q := range []string{"SELECT preparation_json FROM socratica_flows WHERE id='flow'", "SELECT preparation_json FROM socratica_history WHERE flow_id='flow'"} {
		var raw string
		if err = r.db.QueryRow(q).Scan(&raw); err != nil || strings.Contains(raw, "valor-oculto-042") {
			t.Fatalf("%s: %q, %v", q, raw, err)
		}
	}
}

func TestSanitizeSpecify_UnconsumedReviewAndApprovalAreInvalidated(t *testing.T) {
	ctx := context.Background()
	for _, upTo := range []string{"review", "approved"} {
		r := socraticaTestRepo(t)
		before := storedSpecifyFlow(t, r, "flow", upTo)
		leak(t, r, "socratica_flows", "flow")
		sanitizeAll(t, r)
		got, err := r.GetFlow(ctx, "flow")
		if err != nil {
			t.Fatal(err)
		}
		if got.Phase != domain.PhaseCaptured || got.LastStablePhase != domain.PhaseCaptured || got.ProposedPrompt != "" || got.ApprovedPrompt != "" || got.ApprovedHash != "" || got.DispatchID != "" {
			t.Fatalf("%s: the old review/authorization survived: %+v", upTo, got)
		}
		if got.Preparation.Review != nil || len(got.Preparation.Checks) != 0 || got.Revision != before.Revision+1 {
			t.Fatalf("%s: review %v checks %d revision %d (was %d)", upTo, got.Preparation.Review, len(got.Preparation.Checks), got.Revision, before.Revision)
		}
		if strings.Contains(got.Preparation.Content.Intent.Objective, "valor-oculto-042") || !strings.Contains(got.Preparation.Content.Intent.Objective, "Exportar") {
			t.Fatalf("%s: objective = %q", upTo, got.Preparation.Content.Intent.Objective)
		}
		// The draft stays; the old approval can no longer be used.
		if _, _, err = domain.Reduce(got, domain.Event{Kind: "approve", ExpectedRevision: got.Revision}, time.Unix(2000, 0), func() string { return "x" }); err == nil {
			t.Fatalf("%s: approving after the sanitisation succeeded", upTo)
		}
		if _, err = r.Ack(ctx, "flow", before.DispatchID, "", "n"); err == nil {
			t.Fatalf("%s: the old dispatch id still acks", upTo)
		}
	}
}

func TestSanitizeSpecify_ConsumedDispatchKeepsOnlyItsMarks(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	before := storedSpecifyFlow(t, r, "flow", "dispatched")
	if _, err := r.db.Exec("UPDATE socratica_flows SET ack_nonce='n1',dispatch_ack=1 WHERE id='flow'"); err != nil {
		t.Fatal(err)
	}
	leak(t, r, "socratica_flows", "flow")
	sanitizeAll(t, r)
	got, err := r.GetFlow(ctx, "flow")
	if err != nil {
		t.Fatal(err)
	}
	if got.Phase != domain.PhaseDispatched || !got.DispatchAck || got.DispatchID != before.DispatchID || got.AckNonce != "n1" || got.Revision != before.Revision {
		t.Fatalf("a consumed dispatch lost its marks: %+v", got)
	}
	if strings.Contains(got.Preparation.Content.Intent.Objective, "valor-oculto-042") {
		t.Fatal("the sanitised preparation still holds the private region")
	}
	// A consumed dispatch is never re-opened: the nonce replay still works and nothing grants again.
	if res, err := r.Ack(ctx, "flow", before.DispatchID, "", "n1"); err != nil || !res.Acked || res.Granted {
		t.Fatalf("replay after sanitisation = %+v, %v", res, err)
	}
}

func TestSanitizeSpecify_CapturedDraftBumpsRevisionAndClaimedDispatchIsLeftAlone(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	captured := storedSpecifyFlow(t, r, "draft", "review")
	if _, err := r.db.Exec("UPDATE socratica_flows SET phase='captured',last_stable_phase='captured' WHERE id='draft'"); err != nil {
		t.Fatal(err)
	}
	leak(t, r, "socratica_flows", "draft")
	sanitizeAll(t, r)
	got, _ := r.GetFlow(ctx, "draft")
	if got.Revision != captured.Revision+1 || got.Phase != domain.PhaseCaptured {
		t.Fatalf("draft revision %d phase %s", got.Revision, got.Phase)
	}
	claimed := socraticaTestRepo(t)
	c := storedSpecifyFlow(t, claimed, "claimed", "approved")
	if _, err := claimed.db.Exec("UPDATE socratica_flows SET dispatch_claim_owner='o' WHERE id='claimed'"); err != nil {
		t.Fatal(err)
	}
	leak(t, claimed, "socratica_flows", "claimed")
	sanitizeAll(t, claimed)
	got, _ = claimed.GetFlow(ctx, "claimed")
	if got.Phase != domain.PhaseApproved || got.DispatchID != c.DispatchID || got.DispatchClaimOwner != "o" || got.Revision != c.Revision {
		t.Fatalf("a claimed dispatch must not be rewritten under its owner: %+v", got)
	}
}

func TestSanitizeSpecify_NewerPreparationKeepsItsShape(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	storedSpecifyFlow(t, r, "future", "review")
	future := `{"schema_version":99,"stage":"x","content":{"note":"un <private>secreto-042</private> aquí"},"extra":{"keep":true}}`
	if _, err := r.db.Exec("UPDATE socratica_flows SET preparation_json=? WHERE id='future'", future); err != nil {
		t.Fatal(err)
	}
	sanitizeAll(t, r)
	var raw string
	if err := r.db.QueryRow("SELECT preparation_json FROM socratica_flows WHERE id='future'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "secreto-042") || !strings.Contains(raw, `"extra":{"keep":true}`) || !strings.Contains(raw, `"schema_version":99`) {
		t.Fatalf("a newer preparation must be protected without being reinterpreted: %s", raw)
	}
	got, _ := r.GetFlow(ctx, "future")
	if got.Phase != domain.PhaseReview {
		t.Fatalf("a newer preparation was invalidated by guesswork: %s", got.Phase)
	}
}

func TestRedactFlow_AlsoWithdrawsThePreparation(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	storedSpecifyFlow(t, r, "flow", "dispatched")
	if err := r.RedactFlow(ctx, "flow"); err != nil {
		t.Fatal(err)
	}
	var prep *string
	if err := r.db.QueryRow("SELECT preparation_json FROM socratica_flows WHERE id='flow'").Scan(&prep); err != nil || prep != nil {
		t.Fatalf("preparation after redaction = %v, %v", prep, err)
	}
	got, _ := r.GetFlow(ctx, "flow")
	if got.Preparation != nil || got.OriginalPrompt != "" || got.ApprovedPrompt != "" || got.Phase != domain.PhaseDispatched {
		t.Fatalf("redacted flow = %+v", got)
	}
}
