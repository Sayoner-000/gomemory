package cli

import (
	"encoding/json"
	"mem/adapters/primary/setup"
	"strings"
	"testing"
)

func TestHCD014CodexCapabilityDelivery(t *testing.T) {
	payload := func(name string, response any, command string) []byte {
		b, err := json.Marshal(map[string]any{"tool_name": name, "tool_response": response, "tool_input": map[string]string{"command": command}})
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	big := strings.Repeat("synthetic log output\n", 200)
	in := payload("Bash", big, "synthetic-build")
	out := rewriteToolOutput("codex-shell", in, fakeCompress)
	var got struct {
		Continue *bool  `json:"continue"`
		Reason   string `json:"stopReason"`
	}
	if err := json.Unmarshal(out, &got); err != nil || got.Continue == nil || *got.Continue || !strings.Contains(got.Reason, "ref=abcdefabcdef") {
		t.Fatalf("not the observed feedback dialect: %s (%v)", out, err)
	}
	for _, c := range []struct {
		name string
		data []byte
	}{
		{"mcp", payload("mcp__other__tool", big, "")},
		{"unknown_shape", payload("Bash", map[string]string{"output": big}, "build")},
		{"exact_read", payload("Bash", big, "cat source.go")},
		{"small", payload("Bash", "small", "build")},
		{"invalid", []byte("not JSON")},
	} {
		t.Run(c.name, func(t *testing.T) {
			if out := rewriteToolOutput("codex-shell", c.data, fakeCompress); len(out) != 0 {
				t.Fatalf("unverified or excluded path rewritten: %s", out)
			}
		})
	}
	if out := rewriteToolOutput("codex", in, fakeCompress); len(out) != 0 {
		t.Fatalf("legacy dialect must remain silent: %s", out)
	}
}

func TestHCD014CodexShellInstallation(t *testing.T) {
	if !setup.CodexShellFeedbackVersionSupported("codex-cli 0.166.0") {
		t.Fatal("observed version not recognized")
	}
	for _, version := range []string{"", "codex-cli 0.159.0", "codex-cli 0.167.0", "unexpected"} {
		if setup.CodexShellFeedbackVersionSupported(version) {
			t.Fatalf("unverified version promoted: %s", version)
		}
	}
	base := []byte("[[hooks.PostToolUse]]\nmatcher = \"Bash\"\n[[hooks.PostToolUse.hooks]]\ntype = \"command\"\ncommand = \"foreign-hook\"\n")
	on, changed, err := syncCodexShellFeedbackHook(base, "mem", true)
	if err != nil || !changed || !strings.Contains(string(on), "tool-output codex-shell") || !strings.Contains(string(on), "foreign-hook") {
		t.Fatalf("install failed: %s (%v)", on, err)
	}
	if _, changed, err := syncCodexShellFeedbackHook(on, "mem", true); err != nil || changed {
		t.Fatalf("not idempotent: %v %v", changed, err)
	}
	withLegacy := append(append([]byte(nil), on...), []byte("\n[[hooks.PostToolUse]]\n[[hooks.PostToolUse.hooks]]\ntype = \"command\"\ncommand = \"mem hook tool-output codex\"\n")...)
	clean, changed, err := syncCodexShellFeedbackHook(withLegacy, "mem", true)
	if err != nil || !changed || strings.Count(string(clean), "hook tool-output codex") != 1 {
		t.Fatalf("legacy hook survived alongside native hook: %s (%v)", clean, err)
	}
	off, _, err := syncCodexShellFeedbackHook(on, "mem", false)
	if err != nil || strings.Contains(string(off), "tool-output") || !strings.Contains(string(off), "foreign-hook") {
		t.Fatalf("unknown version did not retire own hook safely: %s (%v)", off, err)
	}
	t.Setenv("PATH", t.TempDir())
	if setup.CodexShellFeedbackSupported() {
		t.Fatal("missing host version promoted")
	}
}
