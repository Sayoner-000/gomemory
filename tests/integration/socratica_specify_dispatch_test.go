package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Cases A–C of specs/042-socratica-specify-preparation/quickstart.md against
// the real MCP server. The model and Spec Kit are synthetic: the "agent" is
// this test, and Spec Kit is a counted helper process, so what is proven is the
// control flow (who may execute, how many times, with which exact text), not
// the quality of any real generation. Real agents are accepted in US6.

func clone[T any](t *testing.T, v T) T {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out T
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func (f *specifyFixture) set(id string, rev int, section string, value any) map[string]any {
	f.t.Helper()
	return f.call(map[string]any{"action": "prepare", "flow_id": id, "expected_revision": rev, "preparation": map[string]any{"operation": "set", "section": section, "value": value}})
}

func (f *specifyFixture) check(id string, rev int, stage string) map[string]any {
	f.t.Helper()
	return f.call(map[string]any{"action": "prepare", "flow_id": id, "expected_revision": rev, "preparation": map[string]any{"operation": "validate", "stage": stage}})
}

// toReview starts a flow with the example and checks every stage.
func (f *specifyFixture) toReview(prompt string) (string, map[string]any) {
	f.t.Helper()
	sections := loadExamplePreparation(f.t)
	id := f.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": prompt})["flow_id"].(string)
	for i, name := range []string{"intent", "scope", "stories"} {
		f.set(id, i, name, sections[name])
	}
	var out map[string]any
	for _, stage := range []string{"intent", "scope", "stories", "validation"} {
		out = f.check(id, 3, stage)
	}
	if out["phase"] != "review" {
		f.t.Fatalf("setup: phase %v, pending %v", out["phase"], out["pending_checks"])
	}
	return id, out
}

func pendingPaths(out map[string]any) []string {
	var paths []string
	for _, c := range out["pending_checks"].([]any) {
		m := c.(map[string]any)
		paths = append(paths, m["code"].(string)+"@"+m["path"].(string))
	}
	return paths
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// noSpecsYet: nothing was generated in the agent's project.
func (f *specifyFixture) noSpecsYet() {
	f.t.Helper()
	if _, err := os.Stat(filepath.Join(f.workdir, "specs")); !os.IsNotExist(err) {
		f.t.Fatalf("a specification exists before approval: %v", err)
	}
	if got := f.invocations(); len(got) != 0 {
		f.t.Fatalf("Spec Kit was invoked %d times before approval", len(got))
	}
}

func TestSpecifyCaseA_PreparationGeneratesNothing(t *testing.T) {
	f := newSpecifyFixture(t)
	id, review := f.toReview("Quiero permitir que una persona exporte su historial autorizado")
	proposed := review["proposed"].(string)
	for _, want := range []string{"A1", "US1", "C1", "C2", "D1", "Exportar historiales de otras personas", "Cambiar los permisos de consulta"} {
		if !strings.Contains(proposed, want) {
			t.Errorf("the reviewable text lacks %q", want)
		}
	}
	if review["execution_granted"] != false || review["approved_prompt"] != "" || review["questions_used"] != float64(0) {
		t.Fatalf("review = %v", review)
	}
	f.noSpecsYet()
	// Recoverable: a new read returns the same review.
	if got := f.call(map[string]any{"action": "get", "flow_id": id}); got["review_hash"] != review["review_hash"] || got["proposed"] != proposed {
		t.Fatal("the review is not recoverable")
	}
}

func TestSpecifyCaseB_BlocksEditsAndQuestions(t *testing.T) {
	f := newSpecifyFixture(t)
	sections := loadExamplePreparation(t)
	id := f.call(map[string]any{"action": "start", "client": "codex", "mode": "specify", "prompt": "exportar historial"})["flow_id"].(string)
	f.set(id, 0, "intent", sections["intent"])
	f.set(id, 1, "scope", sections["scope"])

	// 1. A removed criterion and an invalid reference stop the stories stage.
	broken := clone(t, sections["stories"])
	broken["criteria"] = broken["criteria"].([]any)[:1] // C2 removed, D1 still points at it
	broken["criteria"].([]any)[0].(map[string]any)["story_ids"] = []any{"US9"}
	f.set(id, 2, "stories", broken)
	f.check(id, 3, "intent")
	f.check(id, 3, "scope")
	out := f.check(id, 3, "stories")
	paths := pendingPaths(out)
	for _, want := range []string{"invalid_reference@stories.criteria[0].story_ids[0]", "invalid_reference@stories.deliveries[0].criterion_ids[1]", "uncovered_story@stories.stories[0]"} {
		if !contains(paths, want) {
			t.Errorf("pending checks %v lack %s", paths, want)
		}
	}
	if out["phase"] != "captured" || out["stage"] != "stories" {
		t.Fatalf("a broken draft advanced: phase %v stage %v", out["phase"], out["stage"])
	}
	f.fails(map[string]any{"action": "prepare", "flow_id": id, "expected_revision": 3, "preparation": map[string]any{"operation": "validate", "stage": "validation"}})

	// 2. An open blocking pending survives an omitted answer.
	f.set(id, 3, "stories", sections["stories"])
	scope := clone(t, sections["scope"])
	scope["pending"] = map[string]any{"declared": true, "items": []any{map[string]any{"id": "P1", "description": "Formato del archivo", "blocking": true, "resolution": "open"}}}
	f.set(id, 4, "scope", scope)
	f.call(map[string]any{"action": "answer", "flow_id": id, "questions": []any{map[string]any{"text": "¿Formato?", "options": []string{"CSV", "JSON"}}}})
	f.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true})
	for _, stage := range []string{"intent", "scope", "stories", "validation"} {
		out = f.check(id, 5, stage)
	}
	if !contains(pendingPaths(out), "blocking_pending@scope.pending.items[0]") || out["phase"] != "captured" {
		t.Fatalf("an omitted answer resolved the block: %v", pendingPaths(out))
	}
	// Dropping the block silently is refused; resolving it with a note works.
	silent := clone(t, scope)
	silent["pending"] = map[string]any{"declared": true, "items": []any{}}
	if msg := f.fails(map[string]any{"action": "prepare", "flow_id": id, "expected_revision": 5, "preparation": map[string]any{"operation": "set", "section": "scope", "value": silent}}); !strings.Contains(msg, "blocking_pending") {
		t.Fatalf("silent removal = %q", msg)
	}
	resolved := clone(t, scope)
	resolved["pending"] = map[string]any{"declared": true, "items": []any{map[string]any{"id": "P1", "description": "Formato del archivo", "blocking": true, "resolution": "resolved", "resolution_note": "Decidido con la persona: CSV"}}}
	f.set(id, 5, "scope", resolved)

	// 3. Three questions in total, kept across edits and resumes; a fourth is refused.
	q := func() []any { return []any{map[string]any{"text": "¿Cuál?", "options": []string{"A", "B"}}} }
	f.call(map[string]any{"action": "answer", "flow_id": id, "questions": q()})
	f.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true})
	f.call(map[string]any{"action": "answer", "flow_id": id, "questions": q()})
	f.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true})
	f.call(map[string]any{"action": "resume", "flow_id": id})
	f.fails(map[string]any{"action": "answer", "flow_id": id, "questions": q()})
	if got := f.call(map[string]any{"action": "get", "flow_id": id}); got["questions_used"] != float64(3) {
		t.Fatalf("questions_used = %v", got["questions_used"])
	}

	// 4. Review, then change the scope: the old revision cannot be approved.
	var review map[string]any
	for _, stage := range []string{"intent", "scope", "stories", "validation"} {
		review = f.check(id, 6, stage)
	}
	if review["phase"] != "review" {
		t.Fatalf("after resolving the block: %v", pendingPaths(review))
	}
	changed := clone(t, resolved)
	changed["included"] = []any{"Exportar el historial propio en CSV"}
	f.set(id, 6, "scope", changed)
	if msg := f.fails(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 6}); !strings.Contains(msg, "revision_conflict") {
		t.Fatalf("approving the version shown before the edit = %q", msg)
	}

	// 5. No free-text route bypasses the preparation.
	for _, args := range []map[string]any{
		{"action": "rewrite", "flow_id": id, "proposed": "texto libre"},
		{"action": "edit", "flow_id": id, "prompt": "texto libre"},
		{"action": "send_original", "flow_id": id},
	} {
		f.fails(args)
	}
	f.noSpecsYet()
}

func TestSpecifyCaseC_AvailabilityApprovalAndSingleDelivery(t *testing.T) {
	f := newSpecifyFixture(t)
	id, review := f.toReview("Quiero permitir que una persona exporte su historial autorizado")
	reviewed := review["proposed"].(string)

	// 1. Spec Kit cannot be loaded by this agent: the preflight stops before
	// ack. The approval and the prepared text stay; nothing is consumed.
	approved := f.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3})
	dispatch := approved["dispatch_id"].(string)
	if !strings.Contains(approved["next"].(string), "NO llames a ack") {
		t.Fatalf("approve next lacks the preflight: %q", approved["next"])
	}
	// The preflight is the agent's own check (the skill); the backend only has to
	// keep everything intact while no ack is sent.
	held := f.call(map[string]any{"action": "get", "flow_id": id})
	if held["phase"] != "approved" || held["approved_prompt"] != reviewed || held["dispatch_id"] != dispatch || held["execution_granted"] != false {
		t.Fatalf("the approval was not kept while Spec Kit is missing: %v", held)
	}
	f.noSpecsYet()

	// 2. Back in an environment with Spec Kit: the first ack grants, once.
	first := f.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "nonce-1"})
	if first["execution_granted"] != true || first["approved_prompt"] != reviewed {
		t.Fatalf("first ack = %v", first)
	}
	f.deliver(first["approved_prompt"].(string))

	// 3. A lost response retried with the same nonce, another nonce and a repeated
	// approval grant nothing: the agent must not deliver again.
	replay := f.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "nonce-1"})
	other := f.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "nonce-2"})
	again := f.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3})
	for name, out := range map[string]map[string]any{"replay": replay, "other nonce": other, "approve": again} {
		if out["execution_granted"] != false {
			t.Fatalf("%s granted execution: %v", name, out)
		}
	}
	if !strings.Contains(replay["next"].(string), "NO ejecutes") {
		t.Fatalf("replay next = %q", replay["next"])
	}
	got := f.invocations()
	if len(got) != 1 || got[0] != reviewed {
		t.Fatalf("Spec Kit invocations = %d; want exactly one with the exact reviewed text", len(got))
	}
}

// The consumed authorization says nothing about generation: the agent opens the
// synthetic artifacts and reports what it saw, including a failure.
func TestSpecifyCaseC_ResultIsInspectedAndFailuresAreReported(t *testing.T) {
	f := newSpecifyFixture(t)
	id, review := f.toReview("exportar historial")
	dispatch := f.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3})["dispatch_id"].(string)
	f.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n1"})
	f.deliver(review["proposed"].(string))

	// The synthetic generator wrote the files (or not); the agent must look.
	specDir := filepath.Join(f.workdir, "specs", "001-export")
	if err := os.MkdirAll(filepath.Join(specDir, "checklists"), 0o755); err != nil {
		t.Fatal(err)
	}
	spec := "# Spec\n## US1\nAcceptance C1: archivo solo con registros propios.\nAcceptance C2: aviso sin registros.\n"
	if err := os.WriteFile(filepath.Join(specDir, "spec.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "checklists", "requirements.md"), []byte("- [x] ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	read, err := os.ReadFile(filepath.Join(specDir, "spec.md"))
	if err != nil || !strings.Contains(string(read), "C1") || !strings.Contains(string(read), "C2") {
		t.Fatalf("the agent could not trace the criteria in the real file: %v", err)
	}
	if _, err = os.Stat(filepath.Join(specDir, "checklists", "requirements.md")); err != nil {
		t.Fatal(err)
	}
	report := f.call(map[string]any{"action": "report", "flow_id": id, "dispatch_id": dispatch, "outcome": map[string]any{
		"status": "completed", "spec_path": "specs/001-export/spec.md", "checklist_path": "specs/001-export/checklists/requirements.md",
		"traceability": []any{
			map[string]any{"criterion_id": "C1", "story_ids": []any{"US1"}, "spec_reference": "Acceptance C1"},
			map[string]any{"criterion_id": "C2", "story_ids": []any{"US1"}, "spec_reference": "Acceptance C2"},
		},
	}})
	if report["execution_granted"] != false || report["phase"] != "dispatched" || report["outcome"].(map[string]any)["status"] != "completed" {
		t.Fatalf("report = %v", report)
	}
	if n := len(f.invocations()); n != 1 {
		t.Fatalf("reporting must not start another generation: %d invocations", n)
	}
	// An escaping path is refused, and so is claiming success without tracing.
	f.fails(map[string]any{"action": "report", "flow_id": id, "dispatch_id": dispatch, "correct_previous": true, "outcome": map[string]any{"status": "completed", "spec_path": "../x", "checklist_path": "a/b"}})
}

func TestSpecifyCaseC_GenerationFailureIsReportedNotRetriedWithTheSameAuthorization(t *testing.T) {
	f := newSpecifyFixture(t)
	id, review := f.toReview("exportar historial")
	dispatch := f.call(map[string]any{"action": "approve", "flow_id": id, "expected_revision": 3})["dispatch_id"].(string)
	f.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n1"})
	f.deliver(review["proposed"].(string)) // the helper recorded the invocation, then produced nothing

	if _, err := os.Stat(filepath.Join(f.workdir, "specs")); !os.IsNotExist(err) {
		t.Fatal("setup: the synthetic generator must have produced nothing")
	}
	out := f.call(map[string]any{"action": "report", "flow_id": id, "dispatch_id": dispatch, "outcome": map[string]any{"status": "failed", "message": "Spec Kit no generó archivos"}})
	if out["outcome"].(map[string]any)["status"] != "failed" || out["execution_granted"] != false {
		t.Fatalf("failure report = %v", out)
	}
	if retry := f.call(map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": "n9"}); retry["execution_granted"] != false {
		t.Fatalf("a failed delivery re-opened the authorization: %v", retry)
	}
	if n := len(f.invocations()); n != 1 {
		t.Fatalf("invocations = %d, want 1", n)
	}
	// Only an explicit reconciliation creates a new authorization.
	resent := f.call(map[string]any{"action": "force_resend", "flow_id": id, "dispatch_id": dispatch})
	if resent["phase"] != "approved" || resent["dispatch_id"] == dispatch {
		t.Fatalf("resend = %v", resent)
	}
}
