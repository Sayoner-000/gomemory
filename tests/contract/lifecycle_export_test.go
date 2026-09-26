package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// R12 (feature 034): una exportación contiene toda la memoria del proyecto;
// debe crearse legible solo por su dueño (0600), como el almacén (v2.26.3/4).
func TestExport_ArchivoPrivado(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permisos de Unix")
	}
	s := newLifecycleSandbox(t)
	p := s.Project("p1")
	if r := s.Run("", p, "save", "-t", "secreto", "-y", "learning", "contenido"); r.ExitCode != 0 {
		t.Fatalf("save: %s", r.Stderr)
	}
	out := filepath.Join(s.Root, "export.json")
	if r := s.Run("", p, "export", "--out", out); r.ExitCode != 0 {
		t.Fatalf("export: %s", r.Stdout+r.Stderr)
	}
	info, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permisos de la exportación = %o; quiero 0600", perm)
	}
}
