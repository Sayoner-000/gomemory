package domain

import (
	"strings"
	"testing"
)

func readVerdict(cmd, agent string) RuleVerdict {
	return EvaluateRules(RuleEventBash, RuleInput{Command: cmd, Agent: agent}, BuiltinRules())
}

func TestNativeReadOverShell_BloqueaConLecturaNativa(t *testing.T) {
	for _, agent := range []string{"claude", "opencode"} {
		for _, cmd := range []string{
			`cat adapters/primary/cli/cmd_hook.go`,
			`sed -n '25,70p' adapters/primary/cli/cmd_mcp_code_tools.go`,
			`head -n 40 domain/rules.go`,
			`tail -20 src/app.ts`,
			`awk 'NR>=10 && NR<=20' main.py`,
			`cd /home/x/repo; sed -n '1,9p' internal/a.go`,
			`nl -ba "lib/x.rs"`,
			`less infra/plugin.mjs`,
		} {
			v := readVerdict(cmd, agent)
			if v.Action != RuleBlock || v.Rule != NativeReadOverShellRule {
				t.Errorf("%s %q: veredicto %+v; esperaba bloqueo de %s", agent, cmd, v, NativeReadOverShellRule)
				continue
			}
			want := "Read"
			if agent == "opencode" {
				want = "usa read"
			}
			if !strings.Contains(v.Reason, want) || !strings.Contains(v.Reason, "codegraph_explore") {
				t.Errorf("%s %q: el motivo debe nombrar %q y codegraph_explore: %s", agent, cmd, want, v.Reason)
			}
		}
	}
}

// ADM 2: leer con un intérprete inline que recibe la ruta como argumento
// aparte. ADM 3: un heredoc no exime a un lector que también recibe rutas.
func TestNativeReadOverShell_RemediacionADM(t *testing.T) {
	for _, cmd := range []string{
		`python3 -c "import sys; print(open(sys.argv[1]).read())" domain/rules.go`,
		`python -c 'import sys;print(open(sys.argv[1]).read()[:400])' /home/x/repo/a.py`,
		`node -e "console.log(require('fs').readFileSync(process.argv[1],'utf8'))" src/app.ts`,
		`ruby -e 'puts File.read(ARGV[0])' lib/x.rb`,
		`perl -ne 'print if 1..20' internal/a.go`,
		"python3 - domain/rules.go <<'EOF'\nimport sys; print(open(sys.argv[1]).read())\nEOF",
		"cat domain/rules.go <<EOF\nx\nEOF",
		`head -n 5 main.go <<< "x"`,
	} {
		if v := readVerdict(cmd, "claude"); v.Action != RuleBlock || v.Rule != NativeReadOverShellRule {
			t.Errorf("%q: veredicto %+v; esperaba bloqueo de %s", cmd, v, NativeReadOverShellRule)
		}
	}
	for _, cmd := range []string{
		`python3 -c "print(1)"`,
		`python3 -c "import json,sys; json.load(sys.stdin)" data.json`,
		`python3 scripts/tool.py domain/rules.go`, // un script en archivo no es lectura inline
		"cat <<EOF\nmain.go\nEOF",
	} {
		if v := readVerdict(cmd, "claude"); v.Rule == NativeReadOverShellRule {
			t.Errorf("%q: veredicto %+v; native-read-over-shell no debía actuar", cmd, v)
		}
	}
}

func TestNativeReadOverShell_SoloAvisaSinLecturaNativa(t *testing.T) {
	for _, agent := range []string{"codex", "otro", ""} {
		v := readVerdict(`sed -n '1,40p' domain/rules.go`, agent)
		if v.Action != RuleWarn || v.Rule != NativeReadOverShellRule {
			t.Errorf("agente %q: veredicto %+v; esperaba warn", agent, v)
		}
	}
}

func TestNativeReadOverShell_NoInterviene(t *testing.T) {
	for _, cmd := range []string{
		`cat README.md`,
		`tail -f /var/log/app.log`,
		`head -c 200 data.json`,
		`go test ./... 2>&1 | tail -20`,
		`git show HEAD:domain/rules.go`,
		`git diff domain/rules.go`,
		`cat <<EOF | mem save -t x
texto
EOF`,
		`echo "cat main.go"`,
		`ls domain`,
		`sed -n '1,5p' config.yaml`,
	} {
		if v := readVerdict(cmd, "claude"); v.Rule == NativeReadOverShellRule {
			t.Errorf("%q: veredicto %+v; esperaba que native-read-over-shell no actuara", cmd, v)
		}
	}
}

func TestNativeReadOverShell_Override(t *testing.T) {
	propia := Rule{Name: "leer-libre", Event: RuleEventBash, Action: RuleWarn, Enabled: true,
		Origin: RuleOriginProject, Overrides: NativeReadOverShellRule, Match: MatchContains, Pattern: "zzz"}
	v := EvaluateRules(RuleEventBash, RuleInput{Command: `cat main.go`, Agent: "claude"}, append(BuiltinRules(), propia))
	if v.Action != RuleAllow {
		t.Fatalf("la regla propia con overrides debe anular la de serie: %+v", v)
	}
}
