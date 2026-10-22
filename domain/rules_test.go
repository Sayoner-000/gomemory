package domain

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func mustRule(t *testing.T, name, raw string) Rule {
	t.Helper()
	r, err := ParseRule(name, []byte(raw))
	if err != nil {
		t.Fatalf("ParseRule(%s): %v", name, err)
	}
	return r
}

func TestParseRuleValid(t *testing.T) {
	r := mustRule(t, "no-prod.md", "---\nname: no-prod\nevent: bash\nmatch: regex\npattern: 'psql .*prod'\naction: block\nunknown: x\n---\nNo conectes a producción.\n")
	if r.Name != "no-prod" || r.Event != RuleEventBash || r.Match != MatchRegex || r.Pattern != "psql .*prod" ||
		r.Action != RuleBlock || !r.Enabled || r.Message != "No conectes a producción." || r.Origin != RuleOriginProject {
		t.Fatalf("regla: %+v", r)
	}
	d := mustRule(t, "Aviso.md", "---\nevent: prompt\npattern: \"producción\"\naction: warn # comentario\n---\n")
	if d.Name != "aviso" || d.Match != MatchContains || d.Pattern != "producción" || d.Action != RuleWarn {
		t.Fatalf("defaults: %+v", d)
	}
}

func TestParseRuleErrors(t *testing.T) {
	cases := map[string]string{
		"sin-frontmatter.md": "solo texto",
		"evento.md":          "---\nevent: deploy\npattern: x\naction: block\n---\n",
		"vacio.md":           "---\nevent: bash\npattern: ''\naction: block\n---\n",
		"largo.md":           "---\nevent: bash\npattern: " + strings.Repeat("a", 513) + "\naction: block\n---\n",
		"regex.md":           "---\nevent: bash\nmatch: regex\npattern: '([a-'\naction: block\n---\n",
		"accion.md":          "---\nevent: bash\npattern: x\naction: deny\n---\n",
		"match.md":           "---\nevent: bash\nmatch: glob\npattern: x\naction: warn\n---\n",
		"nombre.md":          "---\nname: 'mal nombre!'\nevent: bash\npattern: x\naction: warn\n---\n",
	}
	for name, raw := range cases {
		_, err := ParseRule(name, []byte(raw))
		var le *RuleLoadError
		if !errors.As(err, &le) || le.Source != name || le.Reason == "" {
			t.Errorf("%s: se esperaba RuleLoadError, hubo %v", name, err)
		}
	}
}

func TestParseRuleSanitizesAndCapsMessage(t *testing.T) {
	r := mustRule(t, "x.md", "---\nevent: bash\npattern: x\naction: warn\n---\nhola \x1b[31mrojo\x1b[0m\x07 "+strings.Repeat("z", 3000))
	if strings.ContainsAny(r.Message, "\x1b\x07") {
		t.Fatalf("controles de terminal sin sanear: %q", r.Message[:40])
	}
	if len(r.Message) > RuleMessageMaxBytes {
		t.Fatalf("mensaje de %d bytes", len(r.Message))
	}
}

func TestEvaluateRulesMatching(t *testing.T) {
	rules := []Rule{
		mustRule(t, "a-bash.md", "---\nevent: bash\npattern: terraform apply\naction: block\n---\nusa el pipeline"),
		mustRule(t, "b-file.md", "---\nevent: file\nmatch: regex\npattern: '\\.env$'\naction: warn\n---\nno toques .env"),
		mustRule(t, "c-content.md", "---\nevent: file\npattern: BEGIN PRIVATE KEY\naction: block\n---\nclave privada"),
		mustRule(t, "d-prompt.md", "---\nevent: prompt\npattern: Producción\naction: warn\n---\ncuidado"),
		mustRule(t, "e-stop.md", "---\nevent: stop\npattern: TODO\naction: block\n---\ntermina el trabajo"),
		mustRule(t, "f-any.md", "---\nevent: any\npattern: kubectl delete\naction: warn\n---\nojo"),
	}
	cases := []struct {
		target RuleEvent
		in     RuleInput
		action RuleAction
		rule   string
	}{
		{RuleEventBash, RuleInput{Command: "terraform apply -auto-approve"}, RuleBlock, "a-bash"},
		{RuleEventBash, RuleInput{Command: "terraform plan"}, RuleAllow, ""},
		{RuleEventFile, RuleInput{FilePaths: []string{"config/.env"}}, RuleWarn, "b-file"},
		{RuleEventFile, RuleInput{FilePaths: []string{"k.pem"}, Content: "-----BEGIN PRIVATE KEY-----"}, RuleBlock, "c-content"},
		{RuleEventPrompt, RuleInput{Prompt: "despliega a producción"}, RuleWarn, "d-prompt"},
		{RuleEventStop, RuleInput{LastMessage: "queda un TODO"}, RuleBlock, "e-stop"},
		{RuleEventPrompt, RuleInput{Prompt: "kubectl delete pod x"}, RuleWarn, "f-any"},
		{RuleEventBash, RuleInput{Command: "kubectl delete ns y"}, RuleWarn, "f-any"},
	}
	for _, c := range cases {
		v := EvaluateRules(c.target, c.in, rules)
		if v.Action != c.action || v.Rule != c.rule {
			t.Errorf("%s %+v → %+v, want %s/%s", c.target, c.in, v, c.action, c.rule)
		}
	}
}

func TestEvaluateRulesPrecedenceAndTies(t *testing.T) {
	rules := []Rule{
		mustRule(t, "a.md", "---\nevent: bash\npattern: deploy\naction: warn\n---\nprimer aviso"),
		mustRule(t, "b.md", "---\nevent: bash\npattern: deploy\naction: warn\n---\nsegundo aviso"),
		mustRule(t, "c.md", "---\nevent: bash\npattern: deploy\naction: block\n---\nbloqueo"),
	}
	if v := EvaluateRules(RuleEventBash, RuleInput{Command: "deploy"}, rules); v.Action != RuleBlock || v.Rule != "c" {
		t.Fatalf("block > warn: %+v", v)
	}
	if v := EvaluateRules(RuleEventBash, RuleInput{Command: "deploy"}, rules[:2]); v.Action != RuleWarn || v.Rule != "a" {
		t.Fatalf("empate → primera por nombre: %+v", v)
	}
}

func TestEvaluateRulesOverrideDisablesBuiltin(t *testing.T) {
	override := mustRule(t, "permitir-rm.md", "---\nevent: bash\npattern: never-matches-xyz\naction: warn\nenabled: false\noverrides: destructive-shell\n---\n")
	rules := append(BuiltinRules(), override)
	if v := EvaluateRules(RuleEventBash, RuleInput{Command: "rm -rf build"}, rules); v.Action != RuleAllow {
		t.Fatalf("la regla de serie anulada no debe actuar: %+v", v)
	}
	if v := EvaluateRules(RuleEventBash, RuleInput{Command: "rm -rf build"}, BuiltinRules()); v.Action != RuleBlock {
		t.Fatalf("sin anulación, bloquea: %+v", v)
	}
}

func TestBlockReasonFormat(t *testing.T) {
	v := EvaluateRules(RuleEventBash, RuleInput{Command: "rm -rf /tmp/x"}, BuiltinRules())
	if !strings.HasPrefix(v.Reason, "gomemory expand · regla destructive-shell: ") || !strings.Contains(v.Reason, "mem expand off rules") {
		t.Fatalf("motivo: %q", v.Reason)
	}
}

func TestParseApplyPatch(t *testing.T) {
	patch := "*** Begin Patch\n*** Update File: <scratch>/work/a.txt\n@@\n-hola\n+adios\n*** Add File: <scratch>/work/probe_denied.txt\n+hola\n*** Delete File: <scratch>/work/old.txt\n*** End Patch"
	paths, body := ParseApplyPatch(patch)
	want := []string{"<scratch>/work/a.txt", "<scratch>/work/probe_denied.txt", "<scratch>/work/old.txt"}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("rutas: %v", paths)
	}
	if body != "adios\nhola" {
		t.Fatalf("cuerpo: %q", body)
	}
	if p, b := ParseApplyPatch("ls -la"); len(p) != 0 || b != "" {
		t.Fatalf("no es un parche: %v %q", p, b)
	}
}

func TestParseFrontmatter(t *testing.T) {
	fm, body, ok := ParseFrontmatter([]byte("---\na: 1\nb: 'dos # no comentario'\nc: \"tres\" # comentario\n\nd:\n---\ncuerpo\nlínea"))
	if !ok || fm["a"] != "1" || fm["b"] != "dos # no comentario" || fm["c"] != "tres" || fm["d"] != "" || body != "cuerpo\nlínea" {
		t.Fatalf("%v %q %v", fm, body, ok)
	}
	if _, _, ok := ParseFrontmatter([]byte("sin cabecera")); ok {
		t.Fatal("sin --- inicial no hay frontmatter")
	}
	if _, _, ok := ParseFrontmatter([]byte("---\na: 1\nsin cierre")); ok {
		t.Fatal("sin --- de cierre no hay frontmatter")
	}
}

// A-002 y B-001 (ronda 4) a nivel de guardia: el cuerpo de un heredoc no
// bloquea, lo que sigue al delimitador sí, y la continuación no lo elude.
func TestDestructiveShellHeredocAndContinuation(t *testing.T) {
	cases := []struct {
		cmd  string
		want RuleAction
	}{
		{"cat <<'EOF' > doc.md\nrm -rf build\nEOF", RuleAllow},
		{"cat <<EOF > doc.md\nrm -rf build\nEOF\nrm -rf build", RuleBlock},
		{"rm -rf \\\nbuild", RuleBlock},
		{"r\\\nm -r\\\nf build", RuleBlock},
		{"echo 'rm -rf \\\nbuild'", RuleAllow},
	}
	for _, c := range cases {
		if v := EvaluateRules(RuleEventBash, RuleInput{Command: c.cmd}, BuiltinRules()); v.Action != c.want {
			t.Errorf("%q → %+v, want %v", c.cmd, v, c.want)
		}
	}
}
