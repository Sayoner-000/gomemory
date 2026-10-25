package cli

import (
	"strings"
	"testing"
)

// reviewedFlow drives a specify flow to review through MCP and returns the
// last response. The scope carries an assumption and a non-blocking pending so
// the review text can be checked for them.
func reviewedFlow(e *socraticaMCPEnv) (string, map[string]any) {
	e.t.Helper()
	id := startSpecify(e, "Quiero exportar mi historial")["flow_id"].(string)
	scope := scopeValue(map[string]any{"id": "P1", "description": "Formato del archivo", "blocking": false, "resolution": "open"})
	scope["assumptions"] = map[string]any{"declared": true, "items": []any{"La aplicación ya permite consultar el historial"}}
	fillSpecify(e, id, scope)
	var out map[string]any
	for _, s := range []string{"intent", "scope", "stories", "validation"} {
		out = e.call(validate(id, 3, s, nil))
	}
	if out["phase"] != "review" {
		e.t.Fatalf("setup: phase %v pending %v", out["phase"], pendingCodes(out))
	}
	return id, out
}

func approve(id string, rev any) map[string]any {
	args := map[string]any{"action": "approve", "flow_id": id}
	if rev != nil {
		args["expected_revision"] = rev
	}
	return args
}

func TestMCPSpecifyReview_ShowsTheCompleteTextWithoutSideEffects(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, out := reviewedFlow(e)
	proposed := out["proposed"].(string)
	for _, want := range []string{"La aplicación ya permite consultar el historial", "Formato del archivo", "no bloqueante", "US1", "C1", "D1"} {
		if !strings.Contains(proposed, want) {
			t.Errorf("the reviewable text lacks %q:\n%s", want, proposed)
		}
	}
	if out["original"] != "Quiero exportar mi historial" {
		t.Fatalf("original = %v", out["original"])
	}
	actions := stringList(out["allowed_actions"])
	if !hasString(actions, "approve") || hasString(actions, "ack") {
		t.Fatalf("actions = %v", actions)
	}
	next := out["next"].(string)
	for _, want := range []string{"Aprobar y generar especificación", "Editar", "Copiar", "Cancelar", "completitud", "expected_revision=3"} {
		if !strings.Contains(next, want) {
			t.Errorf("next lacks %q: %q", want, next)
		}
	}
	// Reading is a read-only copy: nothing moves, nothing is granted.
	for i := 0; i < 2; i++ {
		got := e.call(map[string]any{"action": "get", "flow_id": id})
		if got["phase"] != "review" || got["revision"] != float64(3) || got["proposed"] != proposed || got["review_hash"] != out["review_hash"] || got["execution_granted"] != false || got["approved_prompt"] != "" {
			t.Fatalf("get #%d changed or granted: %v", i, got)
		}
	}
}

func TestMCPSpecifyReview_ApprovalIsExplicitAndBoundToTheShownRevision(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, review := reviewedFlow(e)
	if msg := e.fails(approve(id, nil)); !strings.Contains(msg, "expected_revision") {
		t.Fatalf("approve without revision = %q", msg)
	}
	if msg := e.fails(approve(id, 2)); !strings.Contains(msg, "revision_conflict") {
		t.Fatalf("approve of another revision = %q", msg)
	}
	if got := e.call(map[string]any{"action": "get", "flow_id": id}); got["phase"] != "review" {
		t.Fatalf("a refused approval moved the flow: %v", got["phase"])
	}
	out := e.call(approve(id, 3))
	if out["phase"] != "approved" || out["approved_prompt"] != review["proposed"] || out["dispatch_id"] == "" || out["dispatch_id"] == nil {
		t.Fatalf("approve = %v", out)
	}
	if out["execution_granted"] != false || out["acked"] != false || out["stage"] != "approved" {
		t.Fatalf("approving must not execute: %v", out)
	}
	if actions := stringList(out["allowed_actions"]); !hasString(actions, "ack") || hasString(actions, "approve") {
		t.Fatalf("actions after approval = %v", actions)
	}
	// Idempotent: the same dispatch, never a grant.
	again := e.call(approve(id, 3))
	if again["dispatch_id"] != out["dispatch_id"] || again["duplicate"] != true || again["execution_granted"] != false {
		t.Fatalf("repeat approve = %v", again)
	}
	for _, args := range []map[string]any{{"action": "get", "flow_id": id}, {"action": "resume", "flow_id": id}} {
		got := e.call(args)
		if got["execution_granted"] != false || got["phase"] != "approved" || got["dispatch_id"] != out["dispatch_id"] {
			t.Fatalf("%v after approval = %v", args["action"], got)
		}
	}
}

func TestMCPSpecifyReview_EditingForcesANewCheckAndReview(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, _ := reviewedFlow(e)
	changed := storiesValue()
	changed["stories"].([]any)[0].(map[string]any)["need"] = "Exportar mi historial completo"
	edited := e.call(prepareSet(id, 3, "stories", changed))
	if edited["phase"] != "captured" || edited["revision"] != float64(4) || edited["proposed"] != "" || edited["review_hash"] != nil {
		t.Fatalf("after editing the reviewed draft = phase %v revision %v", edited["phase"], edited["revision"])
	}
	if msg := e.fails(approve(id, 3)); !strings.Contains(msg, "revision_conflict") {
		t.Fatalf("approving the version shown before the edit = %q", msg)
	}
	e.fails(approve(id, 4)) // nothing reviewed at revision 4 yet
	for _, s := range []string{"stories", "validation"} {
		edited = e.call(validate(id, 4, s, nil))
	}
	if edited["phase"] != "review" || !strings.Contains(edited["proposed"].(string), "completo") {
		t.Fatalf("second review = %v", edited["phase"])
	}
	if out := e.call(approve(id, 4)); out["phase"] != "approved" {
		t.Fatalf("approving the new version = %v", out["phase"])
	}
	// Editing an approved, unclaimed flow removes the authorization too.
	back := e.call(prepareSet(id, 4, "stories", storiesValue()))
	if back["phase"] != "captured" || back["approved_prompt"] != "" || back["dispatch_id"] != "" {
		t.Fatalf("edit after approval = %v", back)
	}
}

func TestMCPSpecifyReview_FreeTextRoutesStayClosedAtReview(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, review := reviewedFlow(e)
	for _, args := range []map[string]any{
		{"action": "rewrite", "flow_id": id, "proposed": "texto libre"},
		{"action": "edit", "flow_id": id, "prompt": "texto libre"},
		{"action": "send_original", "flow_id": id},
	} {
		if msg := e.fails(args); !strings.Contains(msg, "unsupported_specify_action") {
			t.Errorf("%v = %q", args["action"], msg)
		}
	}
	got := e.call(map[string]any{"action": "get", "flow_id": id})
	if got["proposed"] != review["proposed"] || got["phase"] != "review" {
		t.Fatalf("a closed route changed the reviewed text: %v", got["phase"])
	}
}

func TestMCPSpecifyReview_CancelReturnsTheOriginalAndWithdrawsTheDraft(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, _ := reviewedFlow(e)
	e.call(approve(id, 3))
	out := e.call(map[string]any{"action": "cancel", "flow_id": id})
	if out["phase"] != "cancelled" || out["original"] != "Quiero exportar mi historial" || out["approved_prompt"] != "" || out["proposed"] != "" || out["preparation"] != nil || out["dispatch_id"] != "" {
		t.Fatalf("cancel = %v", out)
	}
	if strings.Contains(out["next"].(string), "Ejecutar") || !strings.Contains(out["next"].(string), "original") {
		t.Fatalf("next = %q", out["next"])
	}
	// The pending slot is free again.
	if again := startSpecify(e, "otra necesidad"); again["recovered"] == true {
		t.Fatalf("a cancelled flow still holds the slot: %v", again)
	}
}

func TestMCPSpecifyReview_PreviousModesKeepApprove(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := e.review("claude")
	out := e.call(map[string]any{"action": "approve", "flow_id": id})
	if out["phase"] != "approved" || out["dispatch_id"] == "" {
		t.Fatalf("refine approve = %v", out)
	}
	if _, present := out["execution_granted"]; present {
		t.Fatal("refine output gained execution_granted")
	}
}
