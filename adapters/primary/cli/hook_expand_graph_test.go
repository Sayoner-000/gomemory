package cli

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// HCD-027: el adaptador calcula qué extensiones cubre un grafo del proyecto y
// la regla graph-before-grep actúa igual en los cuatro runtimes.

func TestGraphIndexedExtensions(t *testing.T) {
	sin := t.TempDir()
	if got := graphIndexedExtensions(sin, func() int { return 0 }); len(got) != 0 {
		t.Fatalf("sin grafo no hay extensiones: %v", got)
	}
	if got := graphIndexedExtensions(sin, func() int { return 12 }); !slices.Equal(got, []string{".go"}) {
		t.Fatalf("con el índice propio de gomemory (solo Go): %v", got)
	}
	con := t.TempDir()
	if err := os.Mkdir(filepath.Join(con, ".codegraph"), 0o700); err != nil {
		t.Fatal(err)
	}
	llamado := false
	got := graphIndexedExtensions(con, func() int { llamado = true; return 0 })
	for _, want := range []string{".go", ".ts", ".py", ".java"} {
		if !slices.Contains(got, want) {
			t.Errorf("con .codegraph/ debe cubrir %s: %v", want, got)
		}
	}
	if slices.Contains(got, ".md") {
		t.Errorf("la documentación no la indexa el grafo: %v", got)
	}
	if llamado {
		t.Error("con .codegraph/ no hace falta consultar el índice propio")
	}
}

func TestGraphBeforeGrep_CuatroRuntimes(t *testing.T) {
	grep := `grep -rn Foo --include=*.go .`
	cases := []struct {
		name, emit, payload string
		blocked             func(hookRenderedOutput) bool
	}{
		{"claude", "", `{"hook_event_name":"PreToolUse","tool_name":"Bash","session_id":"s","tool_input":{"command":"` + grep + `"}}`,
			func(o hookRenderedOutput) bool { return strings.Contains(o.stdout, `"permissionDecision":"deny"`) }},
		{"codex", "json", `{"tool_name":"Bash","session_id":"s","tool_input":{"command":"` + grep + `"}}`,
			func(o hookRenderedOutput) bool { return strings.Contains(o.stdout, `"permissionDecision":"deny"`) }},
		{"agy", "agy", `{"tool_name":"Bash","session_id":"s","tool_input":{"command":"` + grep + `"}}`,
			func(o hookRenderedOutput) bool { return strings.HasPrefix(o.stdout, "BLOCK: ") }},
		{"opencode", "text", `{"tool":"bash","sessionID":"s","args":{"command":"` + grep + `"}}`,
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
				IndexedExtensions: func() []string { return []string{".go"} },
			}}
			out := runHookExpand(context.Background(), d, "pre-tool", c.emit, []byte(c.payload))
			if got := c.blocked(out); got == off {
				t.Errorf("%s (off rules=%v): bloqueado=%v; salida %+v", c.name, off, got, out)
				continue
			}
			if !off && !strings.Contains(out.stdout+out.stderr, domain.GraphBeforeGrepRule) {
				t.Errorf("%s: el motivo debe nombrar la regla: %+v", c.name, out)
			}
		}
	}
}
