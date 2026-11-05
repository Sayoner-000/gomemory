package domain

import (
	"strings"
	"testing"
)

// HCD-027: graph-before-grep bloquea las búsquedas de código por shell cuando
// el proyecto tiene un grafo que indexa esa extensión (Q1 bloquear, Q2 grafo +
// lenguaje indexado). La cobertura llega del adaptador en IndexedExtensions.

var goIndexed = []string{".go", ".ts"}

func graphVerdict(cmd string, exts []string) RuleVerdict {
	return EvaluateRules(RuleEventBash, RuleInput{Command: cmd, IndexedExtensions: exts}, BuiltinRules())
}

func TestGraphBeforeGrep_Bloquea(t *testing.T) {
	for _, cmd := range []string{
		`grep -rn "Foo" --include=*.go .`,
		`grep -rn Foo --include='*.go' adapters`,
		`egrep -r "a|b" --include=*.ts src`,
		`rg -n Foo -g '*.go'`,
		`rg Foo -t go`,
		`rg -tgo Foo`,
		`ag Foo --go`,
		`git grep -n Foo -- '*.go'`,
		`grep -n Foo adapters/primary/cli/cmd_hook.go`,
		`find . -name '*.go' -path '*hook*'`,
		`cd repo && grep -rn Foo --include=*.go .`,
	} {
		v := graphVerdict(cmd, goIndexed)
		if v.Action != RuleBlock || v.Rule != "graph-before-grep" {
			t.Errorf("%q: veredicto %+v; esperaba bloqueo de graph-before-grep", cmd, v)
			continue
		}
		if !strings.Contains(v.Reason, "codegraph_explore") || !strings.Contains(v.Reason, "mem expand off rules") {
			t.Errorf("%q: el motivo debe nombrar la consulta de grafo y la salida de escape: %s", cmd, v.Reason)
		}
	}
}

func TestGraphBeforeGrep_NoInterviene(t *testing.T) {
	casos := []struct {
		cmd  string
		exts []string
	}{
		{`grep -rn Foo --include=*.go .`, nil},      // sin grafo
		{`git log --oneline | grep fix`, goIndexed}, // tubería: filtra salida
		{`go test ./... 2>&1 | grep FAIL`, goIndexed},
		{`grep -n foo README.md`, goIndexed},                // no indexado
		{`grep ERROR app.log`, goIndexed},                   // no indexado
		{`rg TODO -g '*.md'`, goIndexed},                    // filtro no indexado
		{`grep -rn Foo --include=*.py .`, goIndexed},        // lenguaje no indexado
		{`find . -name '*.yaml'`, goIndexed},                // no indexado
		{`find . -type f -newer x`, goIndexed},              // find sin filtro de nombre
		{`echo "grep -rn Foo --include=*.go ."`, goIndexed}, // texto, no comando
		{`ls adapters`, goIndexed},
	}
	for _, c := range casos {
		if v := graphVerdict(c.cmd, c.exts); v.Action != RuleAllow {
			t.Errorf("%q (exts %v): veredicto %+v; esperaba allow", c.cmd, c.exts, v)
		}
	}
}

func TestGraphBeforeGrep_Override(t *testing.T) {
	propia := Rule{Name: "mi-grep", Event: RuleEventBash, Action: RuleAllow, Enabled: true,
		Origin: RuleOriginProject, Overrides: "graph-before-grep", Match: MatchContains, Pattern: "zzz"}
	rules := append(BuiltinRules(), propia)
	v := EvaluateRules(RuleEventBash, RuleInput{Command: `grep -rn Foo --include=*.go .`, IndexedExtensions: goIndexed}, rules)
	if v.Action != RuleAllow {
		t.Fatalf("una regla propia con overrides debe anular graph-before-grep: %+v", v)
	}
	// destructive-shell sigue activa: anular una regla de serie no apaga las demás.
	if v := EvaluateRules(RuleEventBash, RuleInput{Command: "rm -rf /", IndexedExtensions: goIndexed}, rules); v.Action != RuleBlock {
		t.Fatalf("destructive-shell debe seguir bloqueando: %+v", v)
	}
}
