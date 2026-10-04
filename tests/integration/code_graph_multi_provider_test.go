package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// TestCodeGraphMultiProvider cubre la Historia 3 (feature 010): con dos
// proveedores candidatos declarados (uno inexistente, uno disponible),
// `mem context` debe enriquecerse igual usando el disponible, sin
// intervención manual — el ausente se salta en silencio.
func TestCodeGraphMultiProvider(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("eval symlinks: %v", err)
	}
	target = resolved

	if err := persistence.EnsureDir(target); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}
	if _, err := persistence.Open(target); err != nil {
		t.Fatalf("open db: %v", err)
	}

	fakeProvider := writeFakeCodeGraphProvider(t, target)
	missingProvider := filepath.Join(t.TempDir(), "no-existe-binario-xyz")

	settingsJSON := fmt.Sprintf(`{"code_graph_providers":[%q,%q]}`, missingProvider, fakeProvider)
	if err := os.WriteFile(persistence.SettingsPath(target), []byte(settingsJSON), 0644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	// mem context por sí solo NO espera el refresco (MaybeRefresh es
	// fire-and-forget/detached): hay que correr code-refresh de forma
	// síncrona primero para que el snapshot quede escrito en disco antes de
	// leerlo. En producción esto ocurre en background entre turnos; acá se
	// fuerza para no depender de timing.
	refresh := exec.Command(bin, "code-refresh")
	refresh.Dir = target
	if out, err := refresh.CombinedOutput(); err != nil {
		t.Fatalf("mem code-refresh: %v\n%s", err, out)
	}

	cmd := exec.Command(bin, "context")
	cmd.Dir = target
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem context: %v\n%s", err, out)
	}

	output := string(out)
	if !strings.Contains(output, "Grafo de código externo") {
		t.Fatalf("esperaba la sección de grafo externo enriquecida; salida:\n%s", output)
	}
	if !strings.Contains(output, "FakeLang99") {
		t.Fatalf("esperaba el dato distintivo del proveedor FAKE (prueba de que el disponible, no el ausente, respondió); salida:\n%s", output)
	}
}

func TestCodeGraphCLI_CoexisteConCodebaseMemory(t *testing.T) {
	bin := buildMemBinary(t)
	target, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	if db, err := persistence.Open(target); err != nil {
		t.Fatal(err)
	} else {
		_ = db.Close()
	}
	codegraph, synced := writeFakeCodeGraphCLI(t, target)
	cbm := writeFakeCodeGraphProvider(t, target)
	settings := fmt.Sprintf(`{"code_graph_providers":[%q,%q]}`, codegraph, cbm)
	if err := os.WriteFile(persistence.SettingsPath(target), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	refresh := exec.Command(bin, "code-refresh")
	refresh.Dir = target
	if out, err := refresh.CombinedOutput(); err != nil {
		t.Fatalf("code-refresh: %v: %s", err, out)
	}
	if _, err := os.Stat(synced); err != nil {
		t.Fatalf("el refresco debía sincronizar los cambios pendientes: %v", err)
	}
	cmd := exec.Command(bin, "context")
	cmd.Dir = target
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mem context: %v: %s", err, out)
	}
	got := string(out)
	for _, want := range []string{"Grafo de código externo (codegraph)", "Grafo de código externo (codebase-memory-mcp)",
		"codegraph_explore", "`projectPath`", "FakeLang99", "Lenguajes: go, typescript", "9 nodos, 7 relaciones"} {
		if !strings.Contains(got, want) {
			t.Errorf("falta %q en contexto:\n%s", want, got)
		}
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

// writeFakeCodeGraphProvider crea un script ejecutable que imita el CLI de
// codebase-memory-mcp lo justo para list_projects/get_architecture — evita
// depender de que el binario real esté instalado en el entorno de test.
func writeFakeCodeGraphProvider(t *testing.T, root string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fake-codebase-memory-mcp.sh")
	script := fmt.Sprintf(`#!/bin/sh
if [ "$1" = "cli" ]; then
  case "$2" in
    list_projects)
      echo '{"projects":[{"name":"fakeproj","root_path":%q}]}'
      ;;
    get_architecture)
      echo '{"total_nodes":5,"total_edges":3,"languages":[{"language":"FakeLang99","file_count":1}],"hotspots":[],"clusters":[]}'
      ;;
    *)
      echo '{}'
      ;;
  esac
fi
`, root)
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write fake provider script: %v", err)
	}
	return path
}
