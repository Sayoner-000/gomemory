package hcd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HCD-064: `mem hcd verify --stale` sin comando reejecuta el comando propio
// de cada tarea caducada; un commit sin cambios de contenido no caduca nada.
func TestVerifyStaleSinComando(t *testing.T) {
	deps, root := newDeps(t)
	if code, r := call(t, deps, "start", "--kind", "technical", "--lane", "fast", "--title", "Stale", "--why", "w", "--client", "claude"); code != 0 {
		t.Fatalf("start = %d %+v", code, r)
	}
	doc := filepath.Join(root, "hcd", "HCD-001-stale.md")
	data, err := os.ReadFile(doc)
	if err != nil {
		t.Fatal(err)
	}
	filled := strings.NewReplacer(
		"## Intención\npendiente", "## Intención\nx",
		"Incluido: pendiente", "Incluido: a", "Excluido: pendiente", "Excluido: b",
		"## ¿Qué pasa si no lo hacemos?\npendiente", "## ¿Qué pasa si no lo hacemos?\nx",
		"## Alternativa más barata\npendiente", "## Alternativa más barata\ny",
	).Replace(string(data))
	filled = strings.Replace(filled, "## Checklist\n", "## Checklist\n- [ ] 1. Uno → git grep -q save -- save.go\n- [ ] 2. Dos → git --version\n", 1)
	if err := os.WriteFile(doc, []byte(filled), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, r := call(t, deps, "verify", "--task", "1", "--", "git grep -q save -- save.go"); code != 0 || r.Stage == "define" {
		t.Fatalf("verify 1 = %d %+v", code, r)
	}
	if code, r := call(t, deps, "verify", "--task", "2", "--", "git --version"); code != 0 || r.Stage != "close" {
		t.Fatalf("verify 2 = %d %+v", code, r)
	}

	// Un commit sin cambiar contenido deja la evidencia vigente.
	gitRun(t, root, "add", "save.go")
	gitRun(t, root, "commit", "-qm", "mitad", "--allow-empty")
	if code, r := call(t, deps, "show"); code != 0 || r.Stage != "close" {
		t.Fatalf("tras commit sin cambios = %d %s %+v", code, r.Stage, r.Tasks)
	}

	if err := os.WriteFile(filepath.Join(root, "save.go"), []byte("package save // cambio\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, r := call(t, deps, "verify", "--stale")
	if code != 0 || r.Stage != "close" {
		t.Fatalf("verify --stale sin comando = %d %s %+v", code, r.Stage, r.Error)
	}
	for _, task := range r.Tasks {
		if task.Status != "verified" {
			t.Fatalf("tarea %s = %s", task.ID, task.Status)
		}
	}
}
