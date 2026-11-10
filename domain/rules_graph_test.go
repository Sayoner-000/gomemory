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

// Un agente que trabaja en otro proyecto (sin grafo propio) y busca en el
// código de un repositorio indexado: la cobertura la da la ruta buscada, no la
// raíz de la sesión. Es el caso real visto en OpenCode 2.2.5.
func coverageOf(covered ...string) func(string) []string {
	return func(p string) []string {
		for _, c := range covered {
			if p == c || strings.HasPrefix(p, c+"/") {
				return codeExtsForTest
			}
		}
		return nil
	}
}

var codeExtsForTest = []string{".go", ".py", ".ts"}

func TestGraphBeforeGrep_CoberturaPorRuta(t *testing.T) {
	cov := coverageOf("/home/x/gomemory")
	casos := []struct {
		cmd    string
		agent  string
		action RuleAction
	}{
		{`grep -rln "graph_status" /home/x/gomemory --include=*.py | head`, "opencode", RuleBlock},
		{`grep -rn "def graph_status" /home/x/gomemory/adapters /home/x/gomemory/application`, "opencode", RuleBlock},
		{`cd /home/x/gomemory; grep -rn additionalProperties adapters/primary/cli`, "claude", RuleBlock},
		{`rg graph_status /home/x/gomemory`, "claude", RuleBlock},
		{`git grep -n Foo`, "claude", RuleAllow},            // cwd sin cobertura
		{`grep -rn TODO /home/x/otro`, "claude", RuleAllow}, // ruta sin grafo
		{`grep -rn Foo /home/x/gomemory`, "codex", RuleWarn},
		{`rg TODO -g '*.md' /home/x/gomemory`, "claude", RuleAllow}, // filtro no indexado
		{`grep -n Foo /home/x/gomemory/README.md`, "claude", RuleAllow},
		{`git log | grep -r fix`, "claude", RuleAllow}, // tubería
	}
	for _, c := range casos {
		v := EvaluateRules(RuleEventBash, RuleInput{Command: c.cmd, Agent: c.agent, GraphCoverage: cov}, BuiltinRules())
		if v.Action != c.action {
			t.Errorf("%s %q: veredicto %+v; esperaba %s", c.agent, c.cmd, v, c.action)
			continue
		}
		if c.action != RuleAllow && v.Rule != GraphBeforeGrepRule {
			t.Errorf("%q: regla %q; esperaba %s", c.cmd, v.Rule, GraphBeforeGrepRule)
		}
	}
}

// ADM 1: las extensiones del proyecto de la sesión no cubren rutas externas.
// ADM 3: un heredoc no exime a un buscador que también recibe rutas.
// ADM 4: git -C cambia el repositorio contra el que se resuelven las rutas.
func TestGraphBeforeGrep_RemediacionADM(t *testing.T) {
	cov := coverageOf("/home/x/gomemory")
	casos := []struct {
		name   string
		cmd    string
		base   []string
		action RuleAction
	}{
		{"externa sin grafo con extensiones base", `grep -rn Foo --include=*.go /home/x/otro`, goIndexed, RuleAllow},
		{"externa sin grafo, archivo indexable", `grep -n Foo /home/x/otro/main.go`, goIndexed, RuleAllow},
		{"relativa sigue usando la base", `grep -rn Foo --include=*.go src`, goIndexed, RuleBlock},
		{"externa cubierta sigue bloqueando", `grep -rn Foo --include=*.go /home/x/gomemory`, nil, RuleBlock},
		{"heredoc con ruta explícita", "grep -rn graph_status /home/x/gomemory <<EOF\nx\nEOF", nil, RuleBlock},
		{"here-string con ruta explícita", `grep -rn graph_status /home/x/gomemory/adapters <<< "x"`, nil, RuleBlock},
		{"heredoc sin rutas filtra la entrada", "grep -n Foo <<EOF\nFoo\nEOF", goIndexed, RuleAllow},
		{"git -C hacia repo cubierto", `git -C /home/x/gomemory grep -n Foo`, nil, RuleBlock},
		{"git -C con ruta relativa al repo", `git -C /home/x/gomemory grep -n Foo -- adapters`, nil, RuleBlock},
		{"git -C hacia repo sin grafo", `git -C /home/x/otro grep -n Foo`, goIndexed, RuleAllow},
	}
	for _, c := range casos {
		v := EvaluateRules(RuleEventBash, RuleInput{Command: c.cmd, Agent: "claude", IndexedExtensions: c.base, GraphCoverage: cov}, BuiltinRules())
		if v.Action != c.action {
			t.Errorf("%s: %q → %+v; esperaba %s", c.name, c.cmd, v, c.action)
		}
	}
}

// En Windows las rutas absolutas llevan letra de unidad (Git Bash acepta
// C:/…): son externas y no se resuelven contra un cd previo.
func TestGraphBeforeGrep_RutasWindows(t *testing.T) {
	cov := coverageOf("C:/x/gomemory")
	for _, c := range []struct {
		cmd    string
		action RuleAction
	}{
		{`grep -rn graph_status C:/x/gomemory/adapters`, RuleBlock},
		{`cd C:/x/gomemory; grep -rn graph_status adapters`, RuleBlock},
		{`cd /tmp; grep -rn Foo C:/x/gomemory`, RuleBlock},
		{`grep -rn Foo --include=*.go C:/x/otro`, RuleAllow},
	} {
		v := EvaluateRules(RuleEventBash, RuleInput{Command: c.cmd, Agent: "claude", IndexedExtensions: goIndexed, GraphCoverage: cov}, BuiltinRules())
		if v.Action != c.action {
			t.Errorf("%q → %+v; esperaba %s", c.cmd, v, c.action)
		}
	}
	for _, p := range []string{"C:/x", `C:\x`, "c:/", `\\srv\share`} {
		if !externalPath(p) {
			t.Errorf("externalPath(%q) = false; es absoluta en Windows", p)
		}
	}
	if got := joinShellPath("/tmp", "D:/repo"); got != "D:/repo" {
		t.Errorf("joinShellPath con unidad = %q; una ruta absoluta no se une al cd", got)
	}
}

func TestSearchPaths(t *testing.T) {
	casos := []struct {
		name string
		args []string
		want []string
	}{
		{"grep", []string{"-rn", "Foo", "a", "b"}, []string{"a", "b"}},
		{"grep", []string{"-n", "Foo"}, nil},
		{"grep", []string{"-rn", "Foo"}, []string{"."}},
		{"grep", []string{"-A", "3", "-rn", "Foo", "src"}, []string{"src"}},
		{"grep", []string{"-e", "Foo", "-r", "src"}, []string{"src"}},
		{"rg", []string{"Foo", "-g", "*.go"}, []string{"."}},
		{"git", []string{"grep", "-n", "Foo", "--", "dir"}, []string{"dir"}},
		{"find", []string{"src", "-name", "*.go"}, []string{"src"}},
		{"find", []string{"-name", "*.go"}, []string{"."}},
		{"ls", []string{"src"}, nil},
	}
	for _, c := range casos {
		got, _ := searchPaths(c.name, c.args)
		if strings.Join(got, "|") != strings.Join(c.want, "|") {
			t.Errorf("searchPaths(%s %v) = %v; esperaba %v", c.name, c.args, got, c.want)
		}
	}
}
