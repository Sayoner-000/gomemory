package domain

import (
	"strings"
	"testing"
)

func editVerdict(cmd string) RuleVerdict {
	return EvaluateRules(RuleEventBash, RuleInput{Command: cmd}, BuiltinRules())
}

func TestNativeEditOverScripts_Detecta(t *testing.T) {
	for _, cmd := range []string{
		`python3 -c "with open('src/main.ts', 'w') as f: f.write('hello')"`,
		`python3 - <<'EOF'
p='src/infrastructure/composition.ts'
with open(p, 'w') as f: f.write('hello')
EOF`,
		`python -c "open('app.py', 'w').write('x')"`,
		`node -e "fs.writeFileSync('src/index.js', '...') "`,
		`bun -e "await Bun.write('src/app.tsx', '...')"`,
		`ruby -e "File.write('main.rb', '...') "`,
		`perl -pi -e 's/foo/bar/' src/server.go`,
		`sed -i 's/foo/bar/g' src/main.rs`,
		`sed -i.bak 's/foo/bar/g' src/app.py`,
		`cat <<'EOF' > src/service.java
public class Service {}
EOF`,
		`echo "package main" > main.go`,
		`printf "package main\n" > main.go`,
		`cat content.txt | tee src/handler.go`,
	} {
		v := editVerdict(cmd)
		if v.Action != RuleBlock || v.Rule != NativeEditOverScriptsRule {
			t.Errorf("%q: veredicto %+v; esperaba bloqueo de %s", cmd, v, NativeEditOverScriptsRule)
			continue
		}
		if !strings.Contains(v.Reason, "edit_file") && !strings.Contains(v.Reason, "Edit") {
			t.Errorf("%q: el motivo debe nombrar herramientas de edición nativas: %s", cmd, v.Reason)
		}
	}
}

func TestNativeEditOverScripts_NoInterviene(t *testing.T) {
	for _, cmd := range []string{
		`pytest tests/`,
		`python3 -m unittest discover`,
		`go test ./...`,
		`npm test`,
		`cargo test`,
		`python3 script.py`,
		`node server.js`,
		`cat src/main.go | grep func`,
		`head -n 20 src/main.ts`,
		`echo "hello" > README.md`,
		`cat <<'EOF' > config.json
{"key": "value"}
EOF`,
		`python3 -c "print('reading docs/Guia.html')"`,
		`python3 -c "print('reading style.css')"`,
	} {
		v := editVerdict(cmd)
		if v.Action != RuleAllow {
			t.Errorf("%q: veredicto %+v; esperaba allow en comando no destructivo o fuera de código", cmd, v)
		}
	}
}
