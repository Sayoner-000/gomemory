package main

import (
	"os"
	"strings"
	"testing"
)

// HCD-064: la skill hcd-verify embebida y la ayuda de `mem hcd` enseñan
// `verify --stale` sin comando, que reejecuta la comprobación propia de cada
// tarea caducada, y que una caducada cuenta como progreso hasta el final.
func TestHCD064StaleSinComandoDocumentado(t *testing.T) {
	for path, wants := range map[string][]string{
		"../../infrastructure/templates/hcd/hcd-verify/SKILL.md": {
			"`mem hcd verify --stale --json --client <agente>`",
			"último comando que pasó",
			"cuentan como progreso",
		},
		"../../infrastructure/templates/hcd/hcd-implement/SKILL.md": {
			"`mem hcd verify --stale --json --client <agente>`",
		},
		"../../adapters/primary/cli/hcd/cmd_hcd.go": {
			"verify  [unidad] --stale [-- 'comando']",
		},
	} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range wants {
			if !strings.Contains(string(b), want) {
				t.Errorf("%s no contiene %q", path, want)
			}
		}
	}
}
