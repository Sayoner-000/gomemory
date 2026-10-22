package rulesfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

func writeRule(t *testing.T, root, name, body string) string {
	t.Helper()
	dir := filepath.Join(root, RulesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const warnRule = "---\nevent: prompt\npattern: producción\naction: warn\n---\nojo con producción\n"

func projectRules(rules []domain.Rule) []domain.Rule {
	var out []domain.Rule
	for _, r := range rules {
		if r.Origin == domain.RuleOriginProject {
			out = append(out, r)
		}
	}
	return out
}

func TestLoadWithoutDirReturnsOnlyBuiltin(t *testing.T) {
	rules, errs := New().Load(context.Background(), t.TempDir())
	if len(errs) != 0 || len(rules) != 1 || rules[0].Name != "destructive-shell" {
		t.Fatalf("sin directorio: %+v %+v", rules, errs)
	}
}

func TestLoadCapsCountAndSize(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 65; i++ {
		writeRule(t, root, fmt.Sprintf("r%02d.md", i), warnRule)
	}
	rules, errs := New().Load(context.Background(), root)
	if n := len(projectRules(rules)); n != domain.RuleMaxCount {
		t.Fatalf("se cargan %d, tope %d", n, domain.RuleMaxCount)
	}
	if len(errs) != 1 || errs[0].Source != "r64.md" {
		t.Fatalf("la regla 65 se informa como error: %+v", errs)
	}

	root2 := t.TempDir()
	writeRule(t, root2, "grande.md", "---\nevent: bash\npattern: x\naction: warn\n---\n"+strings.Repeat("a", 17*1024))
	rules, errs = New().Load(context.Background(), root2)
	if len(projectRules(rules)) != 0 || len(errs) != 1 || errs[0].Source != "grande.md" {
		t.Fatalf("archivo de 17 KiB: %+v %+v", rules, errs)
	}
}

func TestLoadIgnoresSymlinksAndNonMarkdown(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "fuera.md")
	_ = os.WriteFile(target, []byte(warnRule), 0o644)
	dir := filepath.Join(root, RulesDir)
	_ = os.MkdirAll(dir, 0o755)
	if err := os.Symlink(target, filepath.Join(dir, "enlace.md")); err != nil {
		t.Skip("symlinks no disponibles:", err)
	}
	writeRule(t, root, "notas.txt", warnRule)
	writeRule(t, root, "buena.md", warnRule)
	rules, errs := New().Load(context.Background(), root)
	pr := projectRules(rules)
	if len(pr) != 1 || pr[0].Name != "buena" || len(errs) != 0 {
		t.Fatalf("solo archivos regulares .md: %+v %+v", pr, errs)
	}
}

func TestLoadSanitizesTerminalControls(t *testing.T) {
	root := t.TempDir()
	writeRule(t, root, "esc.md", "---\nevent: bash\npattern: x\naction: warn\n---\n\x1b]0;titulo\x07hola \x1b[2Jmundo\n")
	rules, _ := New().Load(context.Background(), root)
	pr := projectRules(rules)
	if len(pr) != 1 || strings.ContainsAny(pr[0].Message, "\x1b\x07") {
		t.Fatalf("mensaje sin sanear: %+v", pr)
	}
}

func TestLoadUsesCacheUntilFilesChange(t *testing.T) {
	root := t.TempDir()
	p := writeRule(t, root, "a.md", "---\nevent: bash\nmatch: regex\npattern: 'dep.oy'\naction: block\n---\nno\n")
	l := New()
	if rules, _ := l.Load(context.Background(), root); len(projectRules(rules)) != 1 {
		t.Fatal("primera carga")
	}
	cache := filepath.Join(root, ".memory", cacheFile)
	data, err := os.ReadFile(cache)
	if err != nil {
		t.Fatalf("debe existir la caché: %v", err)
	}
	if info, _ := os.Stat(cache); info.Mode().Perm() != 0o600 {
		t.Fatalf("permisos de la caché: %v", info.Mode().Perm())
	}
	// Se manipula la caché: si se reutiliza, se verá el cambio.
	_ = os.WriteFile(cache, []byte(strings.Replace(string(data), "dep.oy", "CACHED", 1)), 0o600)
	rules, _ := l.Load(context.Background(), root)
	if projectRules(rules)[0].Pattern != "CACHED" {
		t.Fatal("sin cambios en los archivos debe usarse la caché")
	}
	// La regla de la caché sigue siendo evaluable (regex recompilada).
	if v := domain.EvaluateRules(domain.RuleEventBash, domain.RuleInput{Command: "CACHED"}, rules); v.Action != domain.RuleBlock {
		t.Fatalf("regex de la caché sin compilar: %+v", v)
	}
	// Tocar el archivo invalida la caché.
	later := time.Now().Add(time.Minute)
	_ = os.Chtimes(p, later, later)
	rules, _ = l.Load(context.Background(), root)
	if projectRules(rules)[0].Pattern != "dep.oy" {
		t.Fatal("tras tocar el archivo debe recompilarse")
	}
	// Una caché corrupta se recompila sin error.
	_ = os.WriteFile(cache, []byte("{corrupta"), 0o600)
	rules, errs := l.Load(context.Background(), root)
	if len(projectRules(rules)) != 1 || len(errs) != 0 {
		t.Fatalf("caché corrupta: %+v %+v", rules, errs)
	}
}

func TestLoadRespectsCancelledContext(t *testing.T) {
	root := t.TempDir()
	writeRule(t, root, "a.md", warnRule)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	rules, _ := New().Load(ctx, root)
	if len(projectRules(rules)) != 0 || len(rules) != 1 {
		t.Fatalf("con ctx cancelado solo la regla de serie: %+v", rules)
	}
}
