package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func memBinName() string {
	if runtime.GOOS == "windows" {
		return "mem.exe"
	}
	return "mem"
}

func writeExec(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

// R1 (feature 034): hay global cuando `mem` se resuelve por el PATH y no es el
// mismo archivo que la copia del proyecto.
func TestResolveGlobalBinary_SinGlobal(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if path, ok := resolveGlobalBinary(t.TempDir()); ok {
		t.Fatalf("sin mem en el PATH no debe haber global; obtuve %q", path)
	}
}

func TestResolveGlobalBinary_GlobalDistintoDeLaCopia(t *testing.T) {
	binDir := t.TempDir()
	global := filepath.Join(binDir, memBinName())
	writeExec(t, global)
	project := t.TempDir()
	writeExec(t, filepath.Join(project, memBinName()))
	t.Setenv("PATH", binDir)

	path, ok := resolveGlobalBinary(project)
	if !ok {
		t.Fatal("con mem en el PATH y distinto de la copia debe haber global")
	}
	if !sameFile(path, global) {
		t.Fatalf("global resuelto = %q; quiero %q", path, global)
	}
}

// Si el PATH apunta al propio proyecto, la "global" es la copia: no cuenta.
func TestResolveGlobalBinary_ElPathResuelveLaCopiaDelProyecto(t *testing.T) {
	project := t.TempDir()
	writeExec(t, filepath.Join(project, memBinName()))
	t.Setenv("PATH", project)

	if path, ok := resolveGlobalBinary(project); ok {
		t.Fatalf("la copia del proyecto no es un global; obtuve %q", path)
	}
}

// binRefFor conserva su comportamiento: con global, todo por nombre.
func TestBinRefFor_ConGlobalUsaMem(t *testing.T) {
	binDir := t.TempDir()
	writeExec(t, filepath.Join(binDir, memBinName()))
	t.Setenv("PATH", binDir)

	br := binRefFor(t.TempDir())
	if br.HookCommand != "mem" || br.MCPCommand != "mem" || !br.Global {
		t.Fatalf("binRefFor con global = %+v", br)
	}
}

// Sin global y con copia local, los hooks de Claude usan la copia.
func TestBinRefFor_SinGlobalConCopiaLocal(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	project := t.TempDir()
	writeExec(t, filepath.Join(project, memBinName()))

	br := binRefFor(project)
	if br.Global || br.HookCommand != "${CLAUDE_PROJECT_DIR}/"+memBinName() {
		t.Fatalf("binRefFor sin global = %+v", br)
	}
}

func sameFile(a, b string) bool {
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}
