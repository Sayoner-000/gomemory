package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const bloquesGlobales = "<!-- gomemory-universal-agent-instructions-v1 -->\n# Universal\n<!-- gomemory-universal-agent-instructions-end -->\n\n" +
	"<!-- gomemory-protocol-v10 -->\n## Memoria Persistente (`mem`) — Protocolo Activo\nreglas\n<!-- gomemory-protocol-end -->\n"

const textoPropio = "# Mis reglas\n\nPrimer párrafo propio.\n"
const textoFinal = "## Lo mío al final\n\nSigue siendo mío.\n"

func archivoConBloques(t *testing.T) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "CLAUDE.md")
	original := textoPropio + "\n" + bloquesGlobales + "\n" + textoFinal
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	return path, original
}

func TestMigracionGlobal_ConservaTextoYCopia(t *testing.T) {
	path, original := archivoConBloques(t)
	retired, errs := retireGlobalBlocks([]string{path}, "20261031")
	if len(errs) != 0 || len(retired) != 1 {
		t.Fatalf("retired=%v errs=%v", retired, errs)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "gomemory") || !strings.Contains(string(got), textoPropio) || !strings.Contains(string(got), textoFinal) {
		t.Fatalf("resultado:\n%s", got)
	}
	copia, err := os.ReadFile(path + ".gomemory-backup-20261031")
	if err != nil || string(copia) != original {
		t.Fatalf("copia = %q, %v", copia, err)
	}
	if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o640 {
		t.Fatalf("modo = %o", info.Mode().Perm())
	}
}

func TestMigracionGlobal_Idempotente(t *testing.T) {
	path, _ := archivoConBloques(t)
	if _, errs := retireGlobalBlocks([]string{path}, "uno"); len(errs) != 0 {
		t.Fatal(errs)
	}
	antes, _ := os.ReadFile(path)
	retired, errs := retireGlobalBlocks([]string{path}, "dos")
	if len(retired) != 0 || len(errs) != 0 {
		t.Fatalf("segunda pasada: %v %v", retired, errs)
	}
	despues, _ := os.ReadFile(path)
	if string(antes) != string(despues) {
		t.Fatal("la segunda pasada cambió el archivo")
	}
	if _, err := os.Stat(path + ".gomemory-backup-dos"); !os.IsNotExist(err) {
		t.Fatalf("segunda copia creada: %v", err)
	}
}

func TestMigracionGlobal_FalloDeCopia(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("modos POSIX")
	}
	path, original := archivoConBloques(t)
	dir := filepath.Dir(path)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	retired, errs := retireGlobalBlocks([]string{path}, "x")
	if len(errs) != 1 || len(retired) != 0 {
		t.Fatalf("retired=%v errs=%v", retired, errs)
	}
	if got, _ := os.ReadFile(path); string(got) != original {
		t.Fatal("se modificó el original sin copia")
	}
}

func TestMigracionGlobal_SinBloquesNoToca(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(path, []byte(textoPropio), 0o644); err != nil {
		t.Fatal(err)
	}
	ausente := filepath.Join(t.TempDir(), "no-existe.md")
	if retired, errs := retireGlobalBlocks([]string{path, ausente}, "x"); len(retired) != 0 || len(errs) != 0 {
		t.Fatalf("%v %v", retired, errs)
	}
}
