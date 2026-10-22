package domain

import (
	"reflect"
	"testing"
)

func act(kind ActionKind, target string) SupertoolAction {
	return SupertoolAction{Kind: kind, Target: target}
}

// TestRouteSupertoolsTruthTable es la tabla de verdad de
// specs/040-expand-supertools/contracts/router.md (filas 1–18).
func TestRouteSupertoolsTruthTable(t *testing.T) {
	off := func(names ...SupertoolName) ExpandSwitches {
		m := map[SupertoolName]bool{}
		for _, n := range names {
			m[n] = true
		}
		return ExpandSwitches{Off: m}
	}
	cases := []struct {
		row  string
		sig  ExpandSignals
		sw   ExpandSwitches
		want []SupertoolAction
	}{
		{"1", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "ls"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetBash)}},
		{"2", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "git commit -m x"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetBash), act(ActionSuggest, SkillVerifyLoop)}},
		{"3", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "gh pr create"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetBash), act(ActionSuggest, SkillVerifyLoop)}},
		{"4", ExpandSignals{Event: ExpandPreTool, Tool: ToolFile}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetFile)}},
		{"5", ExpandSignals{Event: ExpandPreTool, Tool: ToolDelegate}, ExpandSwitches{},
			[]SupertoolAction{act(ActionSuggest, SkillIterativeRetrieval)}},
		{"6", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "ls", ToolCount: 50, CompactThreshold: 50}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetBash), act(ActionSuggest, SkillStrategicCompact)}},
		{"7", ExpandSignals{Event: ExpandPreTool, Tool: ToolOther, ToolCount: 500}, ExpandSwitches{}, nil},
		{"8", ExpandSignals{Event: ExpandPostTool, Tool: ToolShell, Command: "go test ./..."}, ExpandSwitches{},
			[]SupertoolAction{act(ActionInstinctsObserve, "")}},
		{"9", ExpandSignals{Event: ExpandPostTool, Tool: ToolFile}, ExpandSwitches{}, nil},
		{"10", ExpandSignals{Event: ExpandPrompt, Prompt: "añade la librería zod"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetPrompt), act(ActionSuggest, SkillSearchFirst), act(ActionInstinctsObserve, "")}},
		{"11", ExpandSignals{Event: ExpandPrompt, Prompt: "explica este archivo"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetPrompt), act(ActionInstinctsObserve, "")}},
		{"12", ExpandSignals{Event: ExpandSessionStart, Source: "startup", ConfigHash: "h1", LastShieldHash: "h1"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionInstinctsFold, "")}},
		{"13", ExpandSignals{Event: ExpandSessionStart, Source: "startup", ConfigHash: "h2", LastShieldHash: "h1"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionShieldScan, ""), act(ActionInstinctsFold, "")}},
		{"14", ExpandSignals{Event: ExpandSessionStart, Source: "compact", ConfigHash: "h1", LastShieldHash: "h1"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionInstinctsFold, ""), act(ActionCounterReset, ""), act(ActionEpisodeReset, "")}},
		{"15", ExpandSignals{Event: ExpandStop}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetStop), act(ActionInstinctsFold, "")}},
		{"16", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "git commit"}, off(SupertoolFlow),
			[]SupertoolAction{act(ActionRulesEval, RuleTargetBash)}},
		{"16b", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "git commit"}, off(SupertoolRules),
			[]SupertoolAction{act(ActionSuggest, SkillVerifyLoop)}},
		{"16c", ExpandSignals{Event: ExpandSessionStart, Source: "startup", ConfigHash: "h2", LastShieldHash: "h1"}, off(SupertoolShield, SupertoolInstincts), nil},
		{"17", ExpandSignals{Event: ExpandPreTool, Tool: ToolShell, Command: "git commit"}, ExpandSwitches{Disabled: true}, nil},
		{"18", ExpandSignals{Event: "bogus", Tool: ToolShell}, ExpandSwitches{}, nil},
		// Huella desconocida: ante la duda no se escanea (I6).
		{"extra-hash-vacío", ExpandSignals{Event: ExpandSessionStart, Source: "startup", LastShieldHash: "h1"}, ExpandSwitches{},
			[]SupertoolAction{act(ActionInstinctsFold, "")}},
		// Umbral fuera de rango → default 50.
		{"extra-umbral", ExpandSignals{Event: ExpandPreTool, Tool: ToolFile, ToolCount: 49, CompactThreshold: 3}, ExpandSwitches{},
			[]SupertoolAction{act(ActionRulesEval, RuleTargetFile)}},
	}
	for _, c := range cases {
		got := RouteSupertools(c.sig, c.sw)
		if len(got) == 0 && len(c.want) == 0 {
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("fila %s: got %v, want %v", c.row, got, c.want)
		}
	}
}

// I1: pureza — la misma entrada produce la misma salida.
func TestRouteSupertoolsIsDeterministic(t *testing.T) {
	sig := ExpandSignals{Event: ExpandPrompt, Prompt: "add a dependency for retries"}
	a := RouteSupertools(sig, ExpandSwitches{})
	b := RouteSupertools(sig, ExpandSwitches{})
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("no determinista: %v vs %v", a, b)
	}
}

// I4: el orden de grupos es estable en todos los eventos.
func TestRouteSupertoolsOrder(t *testing.T) {
	rank := map[ActionKind]int{
		ActionRulesEval: 0, ActionShieldScan: 1, ActionSuggest: 2, ActionInstinctsObserve: 3,
		ActionInstinctsFold: 4, ActionCounterReset: 5, ActionEpisodeReset: 5,
	}
	sigs := []ExpandSignals{
		{Event: ExpandPrompt, Prompt: "instala el paquete x"},
		{Event: ExpandPreTool, Tool: ToolShell, Command: "git commit", ToolCount: 99},
		{Event: ExpandSessionStart, Source: "compact", ConfigHash: "a", LastShieldHash: "b"},
		{Event: ExpandStop},
	}
	for _, s := range sigs {
		prev := -1
		for _, a := range RouteSupertools(s, ExpandSwitches{}) {
			if rank[a.Kind] < prev {
				t.Errorf("%s: orden roto en %v", s.Event, RouteSupertools(s, ExpandSwitches{}))
			}
			prev = rank[a.Kind]
		}
	}
}
