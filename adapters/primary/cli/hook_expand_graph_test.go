package cli

import (
	"context"
	"encoding/json"
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

// El caso real: OpenCode abierto en un proyecto SIN grafo lee y busca por
// shell el código de otro repositorio que sí está indexado. La cobertura sale
// de la ruta del comando, no de la raíz de la sesión.
func TestGraphCoverage_RutaFueraDeLaSesion(t *testing.T) {
	session := t.TempDir()
	other := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.Mkdir(filepath.Join(other, ".codegraph"), 0o700))
	must(os.MkdirAll(filepath.Join(other, "adapters", "cli"), 0o700))
	must(os.MkdirAll(filepath.Join(other, "docs"), 0o700))
	must(os.WriteFile(filepath.Join(other, "adapters", "cli", "tools.go"), []byte("package cli\n"), 0o600))
	must(os.WriteFile(filepath.Join(other, "docs", "guia.md"), []byte("# guía\n"), 0o600))

	cov := newGraphCoverage(session, func() []string { return nil })
	if got := cov(filepath.Join(other, "adapters")); !slices.Contains(got, ".go") {
		t.Errorf("un directorio con código bajo .codegraph/ debe estar cubierto: %v", got)
	}
	if got := cov(filepath.Join(other, "docs")); len(got) != 0 {
		t.Errorf("un directorio sin código no está cubierto: %v", got)
	}
	if got := cov(filepath.Join(other, "docs", "guia.md")); len(got) != 0 {
		t.Errorf("un archivo no indexado no está cubierto: %v", got)
	}
	if got := cov(filepath.Join(other, "no-existe")); len(got) != 0 {
		t.Errorf("una ruta inexistente no está cubierta: %v", got)
	}

	cases := []struct{ cmd, rule string }{
		{"grep -rn graph_status " + other + "/adapters", domain.GraphBeforeGrepRule},
		{"cd " + other + "; grep -rn graph_status adapters", domain.GraphBeforeGrepRule},
		{"sed -n '1,9p' " + other + "/adapters/cli/tools.go", domain.NativeReadOverShellRule},
	}
	for _, c := range cases {
		var rec []recorded
		d := expandTestDeps(t, &rec)
		d.root = session
		d.hook = usecases.ExpandHookDeps{Rules: usecases.ExpandRules{
			Source: expandStaticRules(domain.BuiltinRules()), Root: session,
			IndexedExtensions: func() []string { return nil },
			GraphCoverage:     cov,
		}}
		payload, _ := json.Marshal(map[string]any{"tool": "bash", "sessionID": "s", "args": map[string]any{"command": c.cmd}})
		out := runHookExpand(context.Background(), d, "pre-tool", "text", payload)
		if !strings.HasPrefix(out.stdout, "BLOCK: ") || !strings.Contains(out.stdout, c.rule) {
			t.Errorf("%q: esperaba BLOCK de %s en OpenCode; salida %+v", c.cmd, c.rule, out)
		}
	}
}
