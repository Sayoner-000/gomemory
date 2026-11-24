package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// TestCodeGraphMultiProvider verifica que con una lista que mezcla un comando
// legado/inexistente y CodeGraph disponible, `mem context` se enriquece con
// CodeGraph y omite el legado en silencio.
func TestCodeGraphMultiProvider(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("el proveedor simulado es un script de shell")
	}
	bin := buildMemBinary(t)
	target, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	if db, err := persistence.Open(target); err != nil {
		t.Fatalf("open db: %v", err)
	} else {
		_ = db.Close()
	}

	codegraph, synced := writeFakeCodeGraphCLI(t, target)
	legacyProvider := filepath.Join(t.TempDir(), "codebase-memory-mcp")

	settingsJSON := fmt.Sprintf(`{"code_graph_providers":[%q,%q]}`, legacyProvider, codegraph)
	if err := os.WriteFile(persistence.SettingsPath(target), []byte(settingsJSON), 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	refresh := exec.Command(bin, "code-refresh")
	refresh.Dir = target
	if out, err := refresh.CombinedOutput(); err != nil {
		t.Fatalf("mem code-refresh: %v\n%s", err, out)
	}
	if _, err := os.Stat(synced); err != nil {
		t.Fatalf("el refresco debía sincronizar los cambios pendientes: %v", err)
	}

	cmd := exec.Command(bin, "context")
	cmd.Dir = target
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem context: %v\n%s", err, out)
	}

	got := string(out)
	for _, want := range []string{
		"Grafo de código externo (codegraph)",
		"codegraph_explore",
		"`projectPath`",
		"Lenguajes: go, typescript",
		"9 nodos, 7 relaciones",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en contexto:\n%s", want, got)
		}
	}
	if strings.Contains(got, "codebase-memory-mcp") {
		t.Errorf("no debía aparecer codebase-memory-mcp en contexto:\n%s", got)
	}
}

func writeFakeCodeGraphCLI(t *testing.T, root string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "codegraph")
	marker := path + ".synced"
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "status" ]; then
  if [ -f %q ]; then
    pending=0
  else
    pending=1
  fi
  echo '{"initialized":true,"projectPath":%q,"nodeCount":9,"edgeCount":7,"languages":["go","typescript"],"index":{"state":"complete"},"pendingChanges":{"added":'"$pending"',"modified":0,"removed":0}}'
elif [ "$1" = "sync" ]; then
  : > %q
fi
`, marker, root, marker)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path, marker
}
