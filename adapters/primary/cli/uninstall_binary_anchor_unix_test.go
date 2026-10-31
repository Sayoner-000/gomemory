//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escribirBinario(t *testing.T, path, salida string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\necho '"+salida+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestAnclaBinario_CambioDeRutaNoSaleDeHome(t *testing.T) {
	home, externo := t.TempDir(), t.TempDir()
	dir := filepath.Join(home, "bin")
	path := filepath.Join(dir, "mem")
	centinela := filepath.Join(externo, "mem")
	escribirBinario(t, path, "gomemory 2.42.1")
	escribirBinario(t, centinela, "ajeno")

	anchor, err := anchorBinary(home, path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer anchor.Close()
	if err := os.Rename(dir, filepath.Join(home, "viejo")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externo, dir); err != nil {
		t.Fatal(err)
	}
	_ = anchor.Remove()
	if _, err := os.Stat(centinela); err != nil {
		t.Fatalf("el borrado siguió el enlace fuera de HOME: %v", err)
	}
}

func TestAnclaBinario_DirectorioMovidoFueraNoSeBorra(t *testing.T) {
	home, externo := t.TempDir(), t.TempDir()
	dir := filepath.Join(home, "bin")
	path := filepath.Join(dir, "mem")
	escribirBinario(t, path, "gomemory 2.42.1")
	anchor, err := anchorBinary(home, path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer anchor.Close()
	movido := filepath.Join(externo, "bin")
	if err := os.Rename(dir, movido); err != nil {
		t.Fatal(err)
	}
	_ = anchor.Remove()
	if _, err := os.Stat(filepath.Join(movido, "mem")); err != nil {
		t.Fatalf("se borró un archivo ya fuera de HOME: %v", err)
	}
}

func TestAnclaBinario_BorraDentroDeHome(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "bin", "mem")
	escribirBinario(t, path, "gomemory 2.42.1")
	anchor, err := anchorBinary(home, path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer anchor.Close()
	if err := anchor.Remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("el binario legítimo debe retirarse")
	}
}

func TestAnclaBinario_AperturaConfinadaRechazaElEnlace(t *testing.T) {
	home, externo := t.TempDir(), t.TempDir()
	if err := os.Symlink(externo, filepath.Join(home, "bin")); err != nil {
		t.Fatal(err)
	}
	real, err := filepath.EvalSymlinks(home)
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(real)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if sub, err := root.OpenRoot("bin"); err == nil {
		sub.Close()
		t.Fatal("la apertura confinada siguió un enlace fuera de HOME")
	}
}

func TestAnclaBinario_CerradoNoBorra(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "bin", "mem")
	escribirBinario(t, path, "gomemory 2.42.1")
	anchor, err := anchorBinary(home, path, "")
	if err != nil {
		t.Fatal(err)
	}
	pl := &uninstallPlanner{anchors: []*binaryAnchor{anchor}}
	pl.closeAnchors()
	if anchor.Remove() == nil {
		t.Fatal("un ancla cerrada no debe borrar")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestPlanBinaries_AjenoSinAviso(t *testing.T) {
	home, fuera := t.TempDir(), t.TempDir()
	t.Setenv("PATH", fuera)
	path := filepath.Join(fuera, "mem")
	for _, c := range []struct {
		salida string
		aviso  bool
	}{{"soy otra herramienta", false}, {"gomemory 2.42.1", true}} {
		escribirBinario(t, path, c.salida)
		pl := &uninstallPlanner{home: home, o: uninstallOptions{target: t.TempDir()}}
		out := captureStdout(t, func() { pl.planBinaries() })
		if got := strings.Contains(out, "a mano"); got != c.aviso {
			t.Fatalf("%q: aviso=%v, salida=%q", c.salida, got, out)
		}
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
}
