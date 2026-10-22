package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"mem/domain"
)

var allDialects = []hookDialect{dialectClaude, dialectJSON, dialectText, dialectNeutral}

func decodeOut(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("salida no es JSON: %q (%v)", s, err)
	}
	return m
}

// contracts/hook-expand.md «Salida por dialecto».
func TestRenderExpandOutput_AllowSilent(t *testing.T) {
	for _, ev := range []domain.ExpandEvent{domain.ExpandPreTool, domain.ExpandPrompt, domain.ExpandStop, domain.ExpandSessionStart} {
		for _, d := range allDialects {
			out := renderExpandOutput(d, ev, domain.ExpandOutcome{})
			want := ""
			if d == dialectClaude || d == dialectJSON {
				want = "{}"
			}
			if out.stdout != want || out.stderr != "" || out.exitCode != 0 {
				t.Errorf("%s/%s allow: %+v, want stdout %q", ev, d, out, want)
			}
		}
	}
}

func TestRenderExpandOutput_PostToolAlwaysSilent(t *testing.T) {
	o := domain.ExpandOutcome{Decision: domain.DecisionWarn, Context: []string{"x"}}
	for _, d := range allDialects {
		if out := renderExpandOutput(d, domain.ExpandPostTool, o); out != (hookRenderedOutput{}) {
			t.Errorf("post-tool/%s debe ser silencioso: %+v", d, out)
		}
	}
}

func TestRenderExpandOutput_PreToolBlock(t *testing.T) {
	o := domain.ExpandOutcome{Decision: domain.DecisionBlock, Reason: "regla destructive-shell"}
	for _, d := range []hookDialect{dialectClaude, dialectJSON} {
		m := decodeOut(t, renderExpandOutput(d, domain.ExpandPreTool, o).stdout)
		h, _ := m["hookSpecificOutput"].(map[string]any)
		if h["hookEventName"] != "PreToolUse" || h["permissionDecision"] != "deny" || h["permissionDecisionReason"] != "regla destructive-shell" {
			t.Errorf("%s: %v", d, m)
		}
	}
	if out := renderExpandOutput(dialectText, domain.ExpandPreTool, o); out.stdout != "BLOCK: regla destructive-shell" {
		t.Errorf("text: %+v", out)
	}
	if out := renderExpandOutput(dialectNeutral, domain.ExpandPreTool, o); out.stderr != "regla destructive-shell" || out.exitCode != 1 || out.stdout != "" {
		t.Errorf("neutral: %+v", out)
	}
}

func TestRenderExpandOutput_PromptAndStopBlock(t *testing.T) {
	o := domain.ExpandOutcome{Decision: domain.DecisionBlock, Reason: "no a producción"}
	for _, ev := range []domain.ExpandEvent{domain.ExpandPrompt, domain.ExpandStop} {
		for _, d := range []hookDialect{dialectClaude, dialectJSON} {
			m := decodeOut(t, renderExpandOutput(d, ev, o).stdout)
			if m["decision"] != "block" || m["reason"] != "no a producción" {
				t.Errorf("%s/%s: %v", ev, d, m)
			}
		}
		if out := renderExpandOutput(dialectText, ev, o); out.stdout != "BLOCK: no a producción" {
			t.Errorf("%s/text: %+v", ev, out)
		}
		if out := renderExpandOutput(dialectNeutral, ev, o); out.stderr != "no a producción" || out.exitCode != 1 {
			t.Errorf("%s/neutral: %+v", ev, out)
		}
	}
}

func TestRenderExpandOutput_ContextPrefixAndEnvelope(t *testing.T) {
	o := domain.ExpandOutcome{Decision: domain.DecisionWarn, Context: []string{"aviso uno", "sugerencia dos"}}
	wantText := "[gomemory expand] aviso uno\n[gomemory expand] sugerencia dos"
	events := map[domain.ExpandEvent]string{
		domain.ExpandPreTool:      "PreToolUse",
		domain.ExpandPrompt:       "UserPromptSubmit",
		domain.ExpandSessionStart: "SessionStart",
	}
	for ev, name := range events {
		for _, d := range []hookDialect{dialectClaude, dialectJSON} {
			m := decodeOut(t, renderExpandOutput(d, ev, o).stdout)
			h, _ := m["hookSpecificOutput"].(map[string]any)
			if h["hookEventName"] != name || h["additionalContext"] != wantText {
				t.Errorf("%s/%s: %v", ev, d, m)
			}
		}
		for _, d := range []hookDialect{dialectText, dialectNeutral} {
			if out := renderExpandOutput(d, ev, o); out.stdout != wantText || out.exitCode != 0 {
				t.Errorf("%s/%s: %+v", ev, d, out)
			}
		}
	}
	// En stop el contexto viaja como systemMessage (Stop no admite additionalContext).
	m := decodeOut(t, renderExpandOutput(dialectClaude, domain.ExpandStop, o).stdout)
	if m["systemMessage"] != wantText {
		t.Errorf("stop/claude: %v", m)
	}
}

// G6 (2.º analyze H1): pre-tool nunca aprueba en silencio.
func TestRenderExpandOutput_PreToolNeverAllows(t *testing.T) {
	outcomes := []domain.ExpandOutcome{
		{},
		{Decision: domain.DecisionWarn, Context: []string{"aviso"}},
		{Decision: domain.DecisionAllow, Context: []string{"sugerencia"}},
		{Decision: domain.DecisionBlock, Reason: "x"},
	}
	for _, o := range outcomes {
		for _, d := range allDialects {
			out := renderExpandOutput(d, domain.ExpandPreTool, o)
			if strings.Contains(out.stdout, `"allow"`) || strings.Contains(out.stdout, "permissionDecision\":\"allow") {
				t.Errorf("%s %+v: la salida contiene allow: %s", d, o, out.stdout)
			}
		}
	}
}

func TestRenderExpandOutput_ContextCappedAt600(t *testing.T) {
	long := strings.Repeat("ñ", 2000)
	out := renderExpandOutput(dialectText, domain.ExpandPrompt, domain.ExpandOutcome{Context: []string{long}})
	if n := utf8.RuneCountInString(out.stdout); n > expandContextMaxRunes {
		t.Errorf("contexto de %d runas, tope %d", n, expandContextMaxRunes)
	}
	if !utf8.ValidString(out.stdout) {
		t.Error("el recorte rompió UTF-8")
	}
}
