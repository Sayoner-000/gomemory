package cli

import (
	"context"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// HCD-001: la regla de serie native-edit-over-scripts actúa igual en los cuatro runtimes
// y se desactiva con mem expand off rules.

func TestNativeEditOverScripts_Hook(t *testing.T) {
	for _, cmd := range []string{
		`sed -i 's/a/b/' src/app.ts`,
		`python3 -c \"open('src/app.ts', 'w').write('x')\"`,
		`cat content.txt | tee src/handler.go`,
	} {
		cases := []struct {
			name, emit, payload string
			blocked             func(hookRenderedOutput) bool
		}{
			{"claude", "", `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s","tool_input":{"command":"` + cmd + `"}}`,
				func(o hookRenderedOutput) bool { return strings.Contains(o.stdout, `"permissionDecision":"deny"`) }},
			{"codex", "json", `{"tool_name":"Bash","session_id":"s","tool_input":{"command":"` + cmd + `"}}`,
				func(o hookRenderedOutput) bool { return strings.Contains(o.stdout, `"permissionDecision":"deny"`) }},
			{"agy", "agy", `{"tool_name":"Bash","session_id":"s","tool_input":{"command":"` + cmd + `"}}`,
				func(o hookRenderedOutput) bool { return strings.HasPrefix(o.stdout, "BLOCK: ") }},
			{"opencode", "text", `{"tool":"bash","sessionID":"s","args":{"command":"` + cmd + `"}}`,
				func(o hookRenderedOutput) bool { return strings.HasPrefix(o.stdout, "BLOCK: ") }},
		}
		for _, c := range cases {
			for _, off := range []bool{false, true} {
				var rec []recorded
				d := expandTestDeps(t, &rec)
				if off {
					d.settings.ExpandOff = []string{string(domain.SupertoolRules)}
				}
				d.hook = usecases.ExpandHookDeps{Rules: usecases.ExpandRules{
					Source: expandStaticRules(domain.BuiltinRules()), Root: d.root,
				}}
				out := runHookExpand(context.Background(), d, "pre-tool", c.emit, []byte(c.payload))
				if got := c.blocked(out); got == off {
					t.Errorf("%s %s (off rules=%v): bloqueado=%v; salida %+v", c.name, cmd, off, got, out)
					continue
				}
				if !off && !strings.Contains(out.stdout+out.stderr, domain.NativeEditOverScriptsRule) {
					t.Errorf("%s %s: el motivo debe nombrar la regla: %+v", c.name, cmd, out)
				}
			}
		}
	}
}
