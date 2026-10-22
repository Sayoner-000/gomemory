package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// H-04 (auditoría 040): un instincts.db que no abre degrada el store global de
// instincts sin impedir guardar ni leer memorias; solo expand instincts falla,
// de forma explícita, y doctor informa la degradación.
func TestInstinctGlobalUnavailableDegrades(t *testing.T) {
	bin := buildLifecycleBinary(t)
	dir, data, home := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(data, "instincts.db"), 0o700); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "HOME="+home, "GOMEMORY_DATA_HOME="+data, "GOMEMORY_NO_UPDATE_CHECK=1")
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	if out, err := run("save", "-t", "x", "-y", "discovery", "hola"); err != nil {
		t.Fatalf("save debe funcionar sin instincts.db: %v\n%s", err, out)
	}
	if out, err := run("context"); err != nil {
		t.Fatalf("context debe funcionar sin instincts.db: %v\n%s", err, out)
	}
	if out, err := run("expand", "instincts", "list"); err == nil || !strings.Contains(out, "instincts globales no disponibles") {
		t.Errorf("expand instincts debe fallar de forma explícita: %v\n%s", err, out)
	}
	if out, _ := run("doctor", "--json"); !strings.Contains(out, "instincts globales no disponibles") {
		t.Errorf("doctor debe informar la degradación:\n%s", out)
	}
}
