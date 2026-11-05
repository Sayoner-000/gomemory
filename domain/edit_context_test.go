package domain

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// HCD-028 VS-01: el dominio decide y compone el contexto de edición.

func TestRouteSupertools_EditContext(t *testing.T) {
	sig := ExpandSignals{Event: ExpandPreTool, Tool: ToolFile, FilePath: "domain/rules.go"}
	on := RouteSupertools(sig, ExpandSwitches{Off: map[SupertoolName]bool{}})
	if !slices.Contains(on, SupertoolAction{Kind: ActionEditContext}) {
		t.Fatalf("pre-tool de archivo debe pedir edit.context: %+v", on)
	}
	off := RouteSupertools(sig, ExpandSwitches{Off: map[SupertoolName]bool{SupertoolContext: true}})
	if slices.Contains(off, SupertoolAction{Kind: ActionEditContext}) {
		t.Fatalf("con context apagada no debe pedirse: %+v", off)
	}
	if !slices.Contains(off, SupertoolAction{Kind: ActionRulesEval, Target: RuleTargetFile}) {
		t.Fatalf("apagar context no apaga las reglas de archivo: %+v", off)
	}
	shell := RouteSupertools(ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "ls"}, ExpandSwitches{Off: map[SupertoolName]bool{}})
	if slices.Contains(shell, SupertoolAction{Kind: ActionEditContext}) {
		t.Fatalf("solo las herramientas de archivo piden contexto: %+v", shell)
	}
	if n, ok := ParseSupertoolName("context"); !ok || n != SupertoolContext {
		t.Fatal("context debe ser una supertool válida para mem expand off")
	}
}

func TestConstitutionSectionFor(t *testing.T) {
	casos := []struct {
		path string
		num  int
	}{
		{"domain/rules.go", 3},
		{"application/usecases/expand_rules.go", 3},
		{"adapters/primary/cli/hook_expand.go", 3},
		{"infrastructure/container.go", 4},
		{"adapters/secondary/persistence/memory.go", 5},
		{"adapters/secondary/persistence/migrations/001.sql", 6},
		{"domain/rules_test.go", 10},
		{"tests/integration/hook_expand_test.go", 10},
		{"web/src/App.tsx", 14},
	}
	for _, c := range casos {
		s, ok := ConstitutionSectionFor(c.path)
		if !ok || s.Number != c.num || s.Title == "" {
			t.Errorf("%s: %+v %v; esperaba §%d", c.path, s, ok, c.num)
		}
	}
	for _, p := range []string{"README.md", "docs/MANUAL.md", ".github/workflows/ci.yml"} {
		if s, ok := ConstitutionSectionFor(p); ok {
			t.Errorf("%s no tiene capa de código: %+v", p, s)
		}
	}
}

func TestRenderEditContext(t *testing.T) {
	in := EditContextInput{
		Path: "domain/rules.go",
		Memories: []EditContextMemory{
			{ID: 1712, Type: "decision", Title: "graph-before-grep bloquea"},
			{ID: 1700, Type: "bugfix", Title: "destructive-shell y sustituciones"},
			{ID: 1690, Type: "pattern", Title: "reglas sobre tokens"},
			{ID: 1600, Type: "learning", Title: "cuarta, sobra"},
		},
		Tests: []EditContextTest{
			{Name: "TestGraphBeforeGrep_Bloquea", Dir: "./domain"},
			{Name: "TestDestructiveShellBuiltin", Dir: "./domain"},
		},
		Impact: &CodeImpactAnnotation{Hotspot: true, Symbol: "EvaluateRules", FanIn: 13},
	}
	out := RenderEditContext(in)
	orden := []string{"domain/rules.go", "get_memory 1712", "get_memory 1690", "go test ./domain -run 'TestGraphBeforeGrep_Bloquea|TestDestructiveShellBuiltin'", "§3", "EvaluateRules", "13"}
	pos := -1
	for _, want := range orden {
		i := strings.Index(out, want)
		if i < 0 {
			t.Fatalf("falta %q en:\n%s", want, out)
		}
		if i < pos {
			t.Fatalf("%q fuera de orden en:\n%s", want, out)
		}
		pos = i
	}
	if strings.Contains(out, "1600") {
		t.Errorf("como mucho 3 memorias:\n%s", out)
	}

	largo := in
	largo.Memories = nil
	for i := range 3 {
		largo.Memories = append(largo.Memories, EditContextMemory{ID: int64(i + 1), Type: "decision", Title: strings.Repeat("título largo ", 200)})
	}
	if n := utf8.RuneCountInString(RenderEditContext(largo)); n > EditContextMaxRunes {
		t.Errorf("el contexto ocupa %d caracteres; tope %d", n, EditContextMaxRunes)
	}

	if out := RenderEditContext(EditContextInput{Path: "README.md"}); out != "" {
		t.Errorf("sin fuentes ni capa no hay contexto: %q", out)
	}
}
