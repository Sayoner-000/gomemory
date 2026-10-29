package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"mem/adapters/primary/cli/clikit"
	hcdcli "mem/adapters/primary/cli/hcd"
)

func TestHDCEngineUsageWithoutArgumentsIsExitTwoWithoutTTY(t *testing.T) {
	oldOut, oldErr := os.Stdout, os.Stderr
	rOut, wOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	rErr, wErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = wOut, wErr
	code := hcdcli.Run(&clikit.Deps{}, nil)
	_ = wOut.Close()
	_ = wErr.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	out, _ := io.ReadAll(rOut)
	errOut, _ := io.ReadAll(rErr)
	if code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
	if len(out) != 0 || !strings.Contains(string(errOut), "mem hcd") {
		t.Fatalf("usage output stdout=%q stderr=%q", out, errOut)
	}
}

func TestHCDJSONResponseContractFields(t *testing.T) {
	response := hcdcli.Response{ProtocolVersion: 1, OK: true, UnitID: "HCD-001", Approach: "hcd", State: "draft", Stage: "definition", AllowedActions: []string{"show"}, PendingChecks: []hcdcli.PendingCheck{}, Blockers: []string{}, NextAction: hcdcli.NextAction{Skill: "hcd-define"}, ExecutionID: "", MemorySyncStatus: "synced", OperationID: "op-1", Data: map[string]any{}}
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"protocol_version", "ok", "unit_id", "approach", "state", "stage", "revision", "playbook_revision", "allowed_actions", "pending_checks", "blockers", "next_action", "execution_id", "memory_sync_status", "operation_id", "data"} {
		if _, ok := got[field]; !ok {
			t.Errorf("response missing %q: %s", field, data)
		}
	}
}

func TestHCDExitCodeContract(t *testing.T) {
	cases := map[string]int{"usage": 2, "not_found": 3, "revision_conflict": 4, "operation_conflict": 4, "not_allowed": 5, "blocked": 5, "atomicity": 5, "no_executor": 5, "already_approved": 5, "invalid_quote": 5, "approach_mismatch": 6, "uncertain": 7, "internal": 1}
	for name, want := range cases {
		if got := hcdcli.ExitCode(name); got != want {
			t.Errorf("ExitCode(%q)=%d, want %d", name, got, want)
		}
	}
}

func TestHCDJSONResponseCanBeDecoded(t *testing.T) {
	data := []byte(`{"protocol_version":1,"ok":false,"error":{"code":"revision_conflict","message":"stale"},"unit_id":"HCD-001","allowed_actions":["show","resume"]}`)
	var got hcdcli.Response
	if err := json.NewDecoder(bytes.NewReader(data)).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.ProtocolVersion != 1 || got.OK || got.UnitID != "HCD-001" || got.Error == nil || got.Error.Code != "revision_conflict" {
		t.Fatalf("decoded response = %+v", got)
	}
}
