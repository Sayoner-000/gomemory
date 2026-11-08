package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/adapters/primary/setup"
)

func TestExpandStatusAgy(t *testing.T) {
	h := newExpandHarness(t)
	if _, err := setup.InstallAgyGlobal(h.home, setup.AgentRef{HookCommand: "mem", MCPCommand: "mem"}, time.Now()); err != nil {
		t.Fatal(err)
	}
	for _, installed := range []bool{true, false} {
		if !installed {
			if err := os.Remove(filepath.Join(h.home, ".gemini", "config", "hooks.json")); err != nil {
				t.Fatal(err)
			}
		}
		out, stderr, code := h.run(t, "", "status")
		want := "agy ✗\n"
		if installed {
			want = "agy ✓\n"
		}
		if code != 0 || !strings.Contains(out, want) {
			t.Errorf("status installed=%t: code=%d stderr=%s output=%s", installed, code, stderr, out)
		}
		out, stderr, code = h.run(t, "", "status", "--json")
		var result struct {
			Wiring map[string]bool `json:"wiring"`
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatal(err)
		}
		got, exists := result.Wiring["agy"]
		if code != 0 || !exists || got != installed || len(result.Wiring) != 4 {
			t.Errorf("status JSON installed=%t: code=%d stderr=%s wiring=%v", installed, code, stderr, result.Wiring)
		}
	}
}
