package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// Ningún hook reinyecta preferencias. Se queda bajo el umbral de compactar pero
// por encima del tercio que antes disparaba el refuerzo.
func TestHook_SinRefuerzoDePreferencias(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	correrMem(t, bin, target, nil, "", "save", "-t", "Regla", "-y", "preference", "preferencia centinela")
	footprint := filepath.Join(target, ".memory", ".footprint")
	for _, evento := range []string{"nudge", "turn-end"} {
		_ = os.Remove(filepath.Join(target, ".memory", ".last-preference-nudge"))
		if err := os.WriteFile(footprint, []byte("20000"), 0o644); err != nil {
			t.Fatal(err)
		}
		if out := runHook(t, bin, target, evento); strings.Contains(out, "REFUERZO") {
			t.Errorf("%s reinyecta preferencias: %q", evento, out)
		}
	}
}
