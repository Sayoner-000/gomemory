package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func scopeValue(pending ...any) map[string]any {
	if pending == nil {
		pending = []any{}
	}
	return map[string]any{
		"included":    []any{"Exportar el historial propio"},
		"excluded":    map[string]any{"declared": true, "items": []any{"Historiales ajenos"}},
		"constraints": map[string]any{"declared": true, "items": []any{}},
		"assumptions": map[string]any{"declared": true, "items": []any{}},
		"pending":     map[string]any{"declared": true, "items": pending},
	}
}

func storiesValue() map[string]any {
	return map[string]any{
		"stories":    []any{map[string]any{"id": "US1", "actor_id": "A1", "need": "Exportar mi historial", "outcome": "Consultarlo fuera"}},
		"criteria":   []any{map[string]any{"id": "C1", "story_ids": []any{"US1"}, "condition": "Tiene registros", "expected_result": "El archivo los contiene"}},
		"deliveries": []any{map[string]any{"id": "D1", "title": "Exportación", "outcome": "Historial exportable", "story_ids": []any{"US1"}, "criterion_ids": []any{"C1"}}},
	}
}

func validate(id string, rev any, stage string, extra map[string]any) map[string]any {
	prep := map[string]any{"operation": "validate", "stage": stage}
	for k, v := range extra {
		prep[k] = v
	}
	args := map[string]any{"action": "prepare", "flow_id": id, "expected_revision": rev, "preparation": prep}
	return args
}

func fillSpecify(e *socraticaMCPEnv, id string, scope map[string]any) {
	e.t.Helper()
	e.call(prepareSet(id, 0, "intent", intentValue()))
	e.call(prepareSet(id, 1, "scope", scope))
	e.call(prepareSet(id, 2, "stories", storiesValue()))
}

func pendingCodes(out map[string]any) []string {
	var codes []string
	for _, c := range out["pending_checks"].([]any) {
		m := c.(map[string]any)
		codes = append(codes, m["code"].(string)+"@"+m["path"].(string))
	}
	return codes
}

func TestMCPSpecifyValidate_FullPathOpensReviewNotApproval(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := startSpecify(e, "exportar historial")["flow_id"].(string)
	fillSpecify(e, id, scopeValue())
	stages := []string{"intent", "scope", "stories", "validation"}
	var out map[string]any
	for i, s := range stages {
		out = e.call(validate(id, 3, s, nil))
		if i < 3 && (out["phase"] != "captured" || out["stage"] != stages[i+1]) {
			t.Fatalf("after %s: phase %v stage %v", s, out["phase"], out["stage"])
		}
	}
	if out["phase"] != "review" || out["stage"] != "review" || out["revision"] != float64(3) {
		t.Fatalf("final = phase %v stage %v revision %v", out["phase"], out["stage"], out["revision"])
	}
	proposed := out["proposed"].(string)
	sum := sha256.Sum256([]byte(proposed))
	if proposed == "" || out["review_hash"] != hex.EncodeToString(sum[:]) {
		t.Fatalf("review_hash %v does not match the exact bytes of proposed", out["review_hash"])
	}
	if !strings.Contains(proposed, "US1") || !strings.Contains(proposed, "C1") || !strings.Contains(proposed, "D1") {
		t.Fatalf("proposed lacks the traceable ids:\n%s", proposed)
	}
	if out["approved_prompt"] != "" || out["dispatch_id"] != "" || out["execution_granted"] != false {
		t.Fatalf("completing the checks must not approve: %v", out)
	}
	if len(out["pending_checks"].([]any)) != 0 {
		t.Fatalf("pending_checks = %v", out["pending_checks"])
	}
	actions := stringList(out["allowed_actions"])
	// Review can be approved by the person's explicit choice; delivery cannot be
	// reached by completeness alone.
	if hasString(actions, "ack") {
		t.Fatalf("ack opened by completeness alone: %v", actions)
	}
	e.fails(map[string]any{"action": "approve", "flow_id": id}) // no expected_revision
	if msg := out["next"].(string); !strings.Contains(msg, "persona") || !strings.Contains(msg, "no ejecutes") {
		t.Fatalf("next = %q", msg)
	}
}

func TestMCPSpecifyValidate_RejectsValidityAndHashesFromTheAgent(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := startSpecify(e, "x")["flow_id"].(string)
	fillSpecify(e, id, scopeValue())
	for _, extra := range []map[string]any{
		{"passed": true}, {"valid": true}, {"ready": true}, {"input_hash": "abc"}, {"review_hash": "abc"},
		{"checks": []any{map[string]any{"passed": true}}}, {"value": map[string]any{"objective": "x"}},
	} {
		e.fails(validate(id, 3, "intent", extra))
	}
	got := e.call(map[string]any{"action": "get", "flow_id": id})
	if checks, _ := got["preparation"].(map[string]any)["checks"].([]any); len(checks) != 0 {
		t.Fatalf("a rejected request stored checks: %v", checks)
	}
}

func TestMCPSpecifyValidate_StageAndRevisionGuards(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := startSpecify(e, "x")["flow_id"].(string)
	fillSpecify(e, id, scopeValue())
	for _, s := range []string{"scope", "stories", "validation", "review", "approved", "nonsense", ""} {
		if msg := e.fails(validate(id, 3, s, nil)); !strings.Contains(msg, "stage_blocked") {
			t.Errorf("stage %q = %q, want stage_blocked", s, msg)
		}
	}
	if msg := e.fails(validate(id, 1, "intent", nil)); !strings.Contains(msg, "revision_conflict") {
		t.Errorf("stale revision = %q", msg)
	}
	noRev := validate(id, nil, "intent", nil)
	delete(noRev, "expected_revision")
	if msg := e.fails(noRev); !strings.Contains(msg, "expected_revision") {
		t.Errorf("missing revision = %q", msg)
	}
	e.fails(map[string]any{"action": "prepare", "flow_id": id, "expected_revision": 3, "preparation": map[string]any{"operation": "bogus"}})
	// An edit invalidates the check, so the next stage closes again.
	e.call(validate(id, 3, "intent", nil))
	changed := intentValue()
	changed["objective"] = "Otro objetivo"
	e.call(prepareSet(id, 3, "intent", changed))
	if msg := e.fails(validate(id, 4, "scope", nil)); !strings.Contains(msg, "stage_blocked") {
		t.Errorf("scope after editing the intent = %q", msg)
	}
}

func TestMCPSpecifyValidate_FailureReturnsMissingItemsAndKeepsDraft(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := startSpecify(e, "x")["flow_id"].(string)
	e.call(prepareSet(id, 0, "intent", map[string]any{"objective": "Exportar", "actors": []any{map[string]any{"id": "A1", "name": "Titular"}}}))
	out := e.call(validate(id, 1, "intent", nil))
	if out["stage"] != "intent" || out["phase"] != "captured" {
		t.Fatalf("failed check = stage %v phase %v", out["stage"], out["phase"])
	}
	codes := pendingCodes(out)
	if !hasString(codes, "missing@intent.expected_result") {
		t.Fatalf("pending_checks = %v", codes)
	}
	checks := out["preparation"].(map[string]any)["checks"].([]any)
	if len(checks) != 1 || checks[0].(map[string]any)["passed"] != false {
		t.Fatalf("checks = %v", checks)
	}
	if out["preparation"].(map[string]any)["content"].(map[string]any)["intent"] == nil {
		t.Fatal("the draft was not kept")
	}
	e.fails(validate(id, 1, "scope", nil))
	if !strings.Contains(out["next"].(string), "pending_checks") {
		t.Fatalf("next = %q", out["next"])
	}
}

func TestMCPSpecifyValidate_OmissionsNeverResolveABlock(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := startSpecify(e, "x")["flow_id"].(string)
	blocking := map[string]any{"id": "P1", "description": "Formato del archivo", "blocking": true, "resolution": "open"}
	fillSpecify(e, id, scopeValue(blocking))
	e.call(map[string]any{"action": "answer", "flow_id": id, "questions": []any{map[string]any{"text": "¿Formato?", "options": []string{"CSV", "JSON"}}}})
	e.call(map[string]any{"action": "answer", "flow_id": id, "skip_rest": true})
	var out map[string]any
	for _, s := range []string{"intent", "scope", "stories", "validation"} {
		out = e.call(validate(id, 3, s, nil))
	}
	if out["phase"] != "captured" || out["stage"] != "validation" || out["proposed"] != "" {
		t.Fatalf("a blocked preparation reached the review: phase %v stage %v", out["phase"], out["stage"])
	}
	if codes := pendingCodes(out); !hasString(codes, "blocking_pending@scope.pending.items[0]") {
		t.Fatalf("pending_checks = %v", codes)
	}
	if out["questions_used"] != float64(1) {
		t.Fatalf("questions_used = %v", out["questions_used"])
	}
	// The block cannot be dropped silently; resolving it with a note works.
	if msg := e.fails(prepareSet(id, 3, "scope", scopeValue())); !strings.Contains(msg, "blocking_pending") {
		t.Fatalf("silent removal = %q", msg)
	}
	resolved := map[string]any{"id": "P1", "description": "Formato del archivo", "blocking": true, "resolution": "resolved", "resolution_note": "Decidido con la persona: CSV"}
	e.call(prepareSet(id, 3, "scope", scopeValue(resolved)))
	for _, s := range []string{"scope", "stories", "validation"} {
		out = e.call(validate(id, 4, s, nil))
	}
	if out["phase"] != "review" {
		t.Fatalf("after resolving the block: phase %v, pending %v", out["phase"], pendingCodes(out))
	}
}

func TestMCPSpecifyValidate_OnlyForSpecifyFlows(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	refine := e.call(map[string]any{"action": "start", "client": "claude", "mode": "refine", "prompt": "original"})
	e.fails(validate(refine["flow_id"].(string), 0, "intent", nil))
}
