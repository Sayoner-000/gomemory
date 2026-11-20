package hcd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// HCD-058: `mem hcd rename` cambia título, archivo y encabezado.
func TestHCDRenameCLI(t *testing.T) {
	deps, root := newDeps(t)
	if code, r := call(t, deps, "start", "--kind", "fix", "--lane", "fast", "--title", "Titulo original", "--why", "w"); code != 0 {
		t.Fatalf("start = %d %+v", code, r.Error)
	}
	code, r := call(t, deps, "rename", "HCD-001", "--title", "Título renombrado")
	if code != 0 || r.DocPath != "hcd/HCD-001-titulo-renombrado.md" || r.Data["title"] != "Título renombrado" {
		t.Fatalf("rename = %d %+v %+v", code, r.Error, r)
	}
	data, err := os.ReadFile(filepath.Join(root, r.DocPath))
	if err != nil || !strings.HasPrefix(string(data), "# Título renombrado\n") {
		t.Fatalf("doc = %q %v", data, err)
	}
	if code, r := call(t, deps, "rename"); code == 0 || r.Error == nil || r.Error.Code != "validation" {
		t.Fatalf("rename sin título = %d %+v", code, r.Error)
	}
	if code, r := call(t, deps, "rename", "--quote", "x"); code == 0 || r.Error == nil || r.Error.Code != "usage" {
		t.Fatalf("rename con flag ajeno = %d %+v", code, r.Error)
	}
}
