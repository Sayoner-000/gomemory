package domain

import "testing"

// HCD-066: la reproducción vista contra el hook real. Un perl -pi sobre un
// Markdown del proyecto pasaba sin control, y una búsqueda de texto literal en
// código quedaba bloqueada aunque el harness no ofreciera búsqueda nativa.
func TestHCD066_Reproduccion(t *testing.T) {
	if v := editVerdict(`perl -0pi -e 's/a/b/' skills/defensive-qa-audit/SKILL.md`); v.Action != RuleBlock {
		t.Errorf("perl -pi sobre un .md del proyecto debe bloquearse, got %+v", v)
	}
	literal := EvaluateRules(RuleEventBash, RuleInput{
		Command: `git grep -n 'cuerpo omitido\|ref=' -- '*.go'`, IndexedExtensions: goIndexed, Agent: "claude",
	}, BuiltinRules())
	if literal.Action != RuleWarn {
		t.Errorf("una búsqueda de texto literal en código debe avisar, no bloquear, got %+v", literal)
	}
}

// Q1 + Q3: los archivos de texto se protegen en la edición in situ siempre, y
// en redirección o tee solo dentro del proyecto. Los intérpretes inline siguen
// mirando solo código: leer un .json es lo normal.
func TestNativeEditOverScripts_ArchivosDeTexto(t *testing.T) {
	bloquea := []string{
		`sed -i 's/a/b/' README.md`,
		`sed -i.bak 's/a/b/' infrastructure/templates/locales/es/get_context.md.tmpl`,
		`perl -pi -e 's/a/b/' docs/guia.html`,
		`perl -0pi -e 's/a/b/' .github/workflows/test.yml`,
		`perl -pi -e 's/a/b/' config.toml`,
		`echo x > docs/notas.txt`,
		`echo '{}' >> .memory/settings.json`,
		`printf x | tee web/estilo.css`,
		`sed -i 's/a/b/' pom.xml`,
		`sed -i 's/a/b/' app.yaml`,
	}
	for _, cmd := range bloquea {
		if v := editVerdict(cmd); v.Action != RuleBlock || v.Rule != NativeEditOverScriptsRule {
			t.Errorf("%q debe bloquearse por %s, got %+v", cmd, NativeEditOverScriptsRule, v)
		}
	}
	permite := []string{
		`go test ./... > /tmp/salida.txt`,
		`mem doctor --json > /private/tmp/claude-501/x/scratchpad/doctor.json`,
		`cmd | tee /tmp/log.txt`,
		`echo x > ~/notas.md`,
		`python3 -c "import json; print(json.load(open('a.json')))"`,
		`node -e "require('./package.json')"`,
		`sed 's/a/b/' README.md`,
		`cat README.md`,
	}
	for _, cmd := range permite {
		if v := editVerdict(cmd); v.Rule == NativeEditOverScriptsRule {
			t.Errorf("%q no debe bloquearse por %s, got %+v", cmd, NativeEditOverScriptsRule, v)
		}
	}
}

// Q2: un patrón de texto literal (con espacio, guion, «|», punto o comillas)
// no es algo que el grafo indexe —comentarios, cadenas, mensajes— y se avisa;
// un patrón con forma de identificador sí lo resuelve el grafo y se bloquea.
func TestGraphBeforeGrep_PatronLiteralAvisa(t *testing.T) {
	avisa := []string{
		`git grep -n 'tiempo-limite' -- '*.go'`,
		`grep -rn "no disponible" --include=*.go .`,
		`rg -n 'a|b' -g '*.go'`,
		`rg -F 'fmt.Errorf("x' -t go`,
		`git grep -n "cuerpo omitido" -- '*.go'`,
	}
	for _, cmd := range avisa {
		v := EvaluateRules(RuleEventBash, RuleInput{Command: cmd, IndexedExtensions: goIndexed, Agent: "claude"}, BuiltinRules())
		if v.Action != RuleWarn || v.Rule != GraphBeforeGrepRule {
			t.Errorf("%q es texto literal: debe avisar por %s, got %+v", cmd, GraphBeforeGrepRule, v)
		}
	}
	bloquea := []string{
		`git grep -n NewProviders -- '*.go'`,
		`grep -rn "evalGraphBeforeGrep" --include=*.go .`,
		`rg -n 'ReadSettings' -t go`,
		`git grep -n -e evalNativeEditOverScripts -- '*.go'`,
	}
	for _, cmd := range bloquea {
		v := EvaluateRules(RuleEventBash, RuleInput{Command: cmd, IndexedExtensions: goIndexed, Agent: "claude"}, BuiltinRules())
		if v.Action != RuleBlock || v.Rule != GraphBeforeGrepRule {
			t.Errorf("%q busca un símbolo: debe bloquear por %s, got %+v", cmd, GraphBeforeGrepRule, v)
		}
	}
}
