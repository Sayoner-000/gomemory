package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestProjectIsolationWithDuplicateFolderNames cubre US3
// (specs/005-global-mcp-store): dos repositorios en rutas distintas que
// comparten el mismo nombre de carpeta final nunca deben compartir memorias
// en el store global — antes de esta feature la identidad de proyecto era
// filepath.Base(root), que colisionaría en este escenario exacto.
func TestProjectIsolationWithDuplicateFolderNames(t *testing.T) {
	bin := buildMemBinary(t)

	group1 := t.TempDir()
	group2 := t.TempDir()

	projA := filepath.Join(group1, "shared-name")
	projB := filepath.Join(group2, "shared-name")
	for _, p := range []string{projA, projB} {
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatalf("crear %q: %v", p, err)
		}
	}

	saveA := exec.Command(bin, "save", "-t", "Solo en proyecto A", "-y", "decision", "contenido A")
	saveA.Dir = projA
	if out, err := saveA.CombinedOutput(); err != nil {
		t.Fatalf("mem save en A: %v\n%s", err, out)
	}

	saveB := exec.Command(bin, "save", "-t", "Solo en proyecto B", "-y", "decision", "contenido B")
	saveB.Dir = projB
	if out, err := saveB.CombinedOutput(); err != nil {
		t.Fatalf("mem save en B: %v\n%s", err, out)
	}

	searchFromA := exec.Command(bin, "search", "proyecto B")
	searchFromA.Dir = projA
	var outA bytes.Buffer
	searchFromA.Stdout = &outA
	if err := searchFromA.Run(); err != nil {
		t.Fatalf("mem search desde A: %v", err)
	}
	if bytes.Contains(outA.Bytes(), []byte("Solo en proyecto B")) {
		t.Fatalf("el proyecto A no debió ver memorias del proyecto B, output:\n%s", outA.String())
	}

	searchFromB := exec.Command(bin, "search", "proyecto A")
	searchFromB.Dir = projB
	var outB bytes.Buffer
	searchFromB.Stdout = &outB
	if err := searchFromB.Run(); err != nil {
		t.Fatalf("mem search desde B: %v", err)
	}
	if bytes.Contains(outB.Bytes(), []byte("Solo en proyecto A")) {
		t.Fatalf("el proyecto B no debió ver memorias del proyecto A, output:\n%s", outB.String())
	}

	listA := exec.Command(bin, "list", "-n", "50")
	listA.Dir = projA
	var listAOut bytes.Buffer
	listA.Stdout = &listAOut
	if err := listA.Run(); err != nil {
		t.Fatalf("mem list desde A: %v", err)
	}
	if !bytes.Contains(listAOut.Bytes(), []byte("Solo en proyecto A")) {
		t.Fatalf("el proyecto A debió ver su propia memoria, output:\n%s", listAOut.String())
	}
}
