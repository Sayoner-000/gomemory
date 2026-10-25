package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func approvedMCP(e *socraticaMCPEnv) (string, string) {
	e.t.Helper()
	id, _ := reviewedFlow(e)
	out := e.call(approve(id, 3))
	return id, out["dispatch_id"].(string)
}

func ack(id, dispatch, nonce string) map[string]any {
	return map[string]any{"action": "ack", "flow_id": id, "dispatch_id": dispatch, "ack_nonce": nonce}
}

func outcomeArgs(id, dispatch string, outcome map[string]any) map[string]any {
	return map[string]any{"action": "report", "flow_id": id, "dispatch_id": dispatch, "outcome": outcome}
}

func completed() map[string]any {
	return map[string]any{
		"status":         "completed",
		"spec_path":      "specs/001-export/spec.md",
		"checklist_path": "specs/001-export/checklists/requirements.md",
		"traceability":   []any{map[string]any{"criterion_id": "C1", "story_ids": []any{"US1"}, "spec_reference": "Acceptance Scenarios 1"}},
	}
}

func TestMCPSpecifyDispatch_OnlyTheFirstCommittedAckGrantsExecution(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, dispatch := approvedMCP(e)
	noNonce := ack(id, dispatch, "")
	delete(noNonce, "ack_nonce")
	e.fails(noNonce)
	first := e.call(ack(id, dispatch, "n1"))
	if first["acked"] != true || first["execution_granted"] != true || first["already_dispatched"] != false || first["phase"] != "dispatched" || first["stage"] != "delivery" {
		t.Fatalf("first ack = %v", first)
	}
	if next := first["next"].(string); !strings.Contains(next, "speckit-specify") || !strings.Contains(next, "exactamente approved_prompt") || !strings.Contains(next, "report") {
		t.Fatalf("grant next = %q", next)
	}
	if first["approved_prompt"] == "" {
		t.Fatal("the grant response must carry the exact text to deliver")
	}
	replay := e.call(ack(id, dispatch, "n1"))
	if replay["acked"] != true || replay["execution_granted"] != false || !strings.Contains(replay["next"].(string), "NO ejecutes") {
		t.Fatalf("same-nonce replay = %v", replay)
	}
	other := e.call(ack(id, dispatch, "n2"))
	if other["acked"] != false || other["already_dispatched"] != true || other["execution_granted"] != false {
		t.Fatalf("other nonce = %v", other)
	}
	e.fails(ack(id, "despacho-viejo", "n1"))
	for _, args := range []map[string]any{{"action": "get", "flow_id": id}, approve(id, 3)} {
		if got := e.call(args); got["execution_granted"] != false {
			t.Fatalf("%v granted execution: %v", args["action"], got["execution_granted"])
		}
	}
}

func TestMCPSpecifyDispatch_ConcurrentAcksGrantExactlyOnce(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, dispatch := approvedMCP(e)
	var wg sync.WaitGroup
	var mu sync.Mutex
	granted, failures := 0, 0
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(nonce string) {
			defer wg.Done()
			res, err := e.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "socratica_flow", Arguments: ack(id, dispatch, nonce)})
			mu.Lock()
			defer mu.Unlock()
			if err != nil || res.IsError {
				failures++
				return
			}
			var out map[string]any
			if json.Unmarshal([]byte(toolText(res)), &out) == nil && out["execution_granted"] == true {
				granted++
			}
		}(fmt.Sprintf("n%d", i))
	}
	wg.Wait()
	if granted != 1 {
		t.Fatalf("grants = %d (tool failures %d), want exactly 1", granted, failures)
	}
}

func TestMCPSpecifyDispatch_ReportCommunicatesAnOutcomeNeverAPermission(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, dispatch := approvedMCP(e)
	e.fails(outcomeArgs(id, dispatch, completed())) // not consumed yet
	e.call(ack(id, dispatch, "n1"))
	missingOutcome := map[string]any{"action": "report", "flow_id": id, "dispatch_id": dispatch}
	e.fails(missingOutcome)
	noDispatch := outcomeArgs(id, dispatch, completed())
	delete(noDispatch, "dispatch_id")
	e.fails(noDispatch)
	e.fails(outcomeArgs(id, "despacho-viejo", completed()))
	bad := completed()
	bad["spec_path"] = "../fuera/spec.md"
	e.fails(outcomeArgs(id, dispatch, bad))
	bad = completed()
	bad["traceability"] = []any{}
	e.fails(outcomeArgs(id, dispatch, bad))
	forged := completed()
	forged["execution_granted"] = true
	e.fails(outcomeArgs(id, dispatch, forged))

	out := e.call(outcomeArgs(id, dispatch, completed()))
	o := out["outcome"].(map[string]any)
	if o["status"] != "completed" || o["dispatch_id"] != dispatch || o["spec_path"] != "specs/001-export/spec.md" {
		t.Fatalf("outcome = %v", o)
	}
	if out["phase"] != "dispatched" || out["execution_granted"] != false || out["dispatch_id"] != dispatch {
		t.Fatalf("report changed the authorization: %v", out)
	}
	if got := e.call(map[string]any{"action": "get", "flow_id": id}); got["outcome"] == nil {
		t.Fatal("the outcome is not recoverable with get")
	}
	// Reporting does not re-open the consumed authorization.
	if again := e.call(ack(id, dispatch, "n2")); again["execution_granted"] != false || again["already_dispatched"] != true {
		t.Fatalf("ack after report = %v", again)
	}
	// Repeating is a no-op; changing needs an explicit correction.
	same := e.call(outcomeArgs(id, dispatch, completed()))
	if same["duplicate"] != true {
		t.Fatalf("identical report = %v", same["duplicate"])
	}
	changed := completed()
	changed["spec_path"] = "specs/001-export/otro.md"
	if msg := e.fails(outcomeArgs(id, dispatch, changed)); !strings.Contains(msg, "outcome_conflict") {
		t.Fatalf("changed report = %q", msg)
	}
	corrected := outcomeArgs(id, dispatch, changed)
	corrected["correct_previous"] = true
	if fixed := e.call(corrected); fixed["outcome"].(map[string]any)["spec_path"] != "specs/001-export/otro.md" || fixed["execution_granted"] != false {
		t.Fatalf("correction = %v", fixed["outcome"])
	}
}

func TestMCPSpecifyDispatch_FailedAndUncertainOutcomesNeedNoArtifacts(t *testing.T) {
	for _, status := range []string{"failed", "uncertain"} {
		e := newSocraticaMCPEnv(t, nil)
		id, dispatch := approvedMCP(e)
		e.call(ack(id, dispatch, "n1"))
		e.fails(outcomeArgs(id, dispatch, map[string]any{"status": status}))
		out := e.call(outcomeArgs(id, dispatch, map[string]any{"status": status, "message": "Spec Kit no generó archivos"}))
		o := out["outcome"].(map[string]any)
		if o["status"] != status || o["spec_path"] != nil && o["spec_path"] != "" {
			t.Fatalf("%s outcome = %v", status, o)
		}
		if !strings.Contains(out["next"].(string), "comunicado por el agente") {
			t.Fatalf("next = %q", out["next"])
		}
	}
}

func TestMCPSpecifyDispatch_ReconciliationIsExplicitAndCreatesOneNewAuthorization(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id, dispatch := approvedMCP(e)
	e.call(ack(id, dispatch, "n1"))
	e.call(outcomeArgs(id, dispatch, map[string]any{"status": "uncertain", "message": "no se pudo comprobar"}))
	e.fails(map[string]any{"action": "force_resend", "flow_id": id, "dispatch_id": "otro"})
	resent := e.call(map[string]any{"action": "force_resend", "flow_id": id, "dispatch_id": dispatch})
	fresh := resent["dispatch_id"].(string)
	if fresh == dispatch || resent["phase"] != "approved" || resent["outcome"] != nil || resent["execution_granted"] != false {
		t.Fatalf("resend = phase %v dispatch %v outcome %v", resent["phase"], resent["dispatch_id"], resent["outcome"])
	}
	e.fails(ack(id, dispatch, "n1")) // the old dispatch is dead
	e.fails(outcomeArgs(id, dispatch, completed()))
	if granted := e.call(ack(id, fresh, "n9")); granted["execution_granted"] != true {
		t.Fatalf("new authorization = %v", granted)
	}
	if again := e.call(ack(id, fresh, "n9")); again["execution_granted"] != false {
		t.Fatalf("replay of the new authorization = %v", again["execution_granted"])
	}
}

func TestMCPSpecifyDispatch_PreviousModesKeepTheirAckAndHaveNoReport(t *testing.T) {
	e := newSocraticaMCPEnv(t, nil)
	id := e.review("claude")
	approved := e.call(map[string]any{"action": "approve", "flow_id": id})
	out := e.call(ack(id, approved["dispatch_id"].(string), "n1"))
	if out["acked"] != true || out["already_dispatched"] != false {
		t.Fatalf("refine ack = %v", out)
	}
	for _, key := range []string{"execution_granted", "outcome", "stage"} {
		if _, present := out[key]; present {
			t.Errorf("refine ack gained %q", key)
		}
	}
	e.fails(outcomeArgs(id, approved["dispatch_id"].(string), completed()))
}
