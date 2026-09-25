package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var refHex = regexp.MustCompile(`ref=([0-9a-f]{12,16})\b`)

func refRe(s string) []string {
	var out []string
	for _, m := range refHex.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	return out
}

// T028 — G1: el contexto que inyectan los hooks (arranque y post-compact) pasa
// por el motor con el nivel max, y sus refs se recuperan. Con structural no
// lleva marcadores.
//
// El post-compact entrega a propósito el contexto en modo índice (sin contenido
// de memorias) y el resumen de sesión acotado: normalmente no hay nada que
// comprimir. Ahí se exige que funcione y que toda marca que aparezca sea
// recuperable, no que las haya.
func TestHookContextCompression(t *testing.T) {
	bin := buildMemBinary(t)
	log, err := os.ReadFile(filepath.Join(repoRootIntegration(t), "tests", "testdata", "compression_corpus", "gotest-fail.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, nivel := range []string{"max", "structural"} {
		for _, hook := range []string{"session-start", "post-compact"} {
			t.Run(nivel+"/"+hook, func(t *testing.T) {
				env := storeAislado(t)
				dir := proyectoCompresion(t, `{"context_compression_level": "`+nivel+`", "budget": -1}`)
				correrMem(t, bin, dir, env, "", "session", "start")
				correrMem(t, bin, dir, env, "", "save", "-t", "log grande", "-y", "bugfix", string(log))
				out := correrMem(t, bin, dir, env, "{}", "hook", hook).stdout
				if strings.TrimSpace(out) == "" {
					t.Fatal("el hook no emitió nada: la prueba no mide lo que cree")
				}
				refs := refRe(out)
				if nivel == "structural" {
					if strings.Contains(out, "⟦mem⟧") {
						t.Error("con structural no debe haber marcadores")
					}
					return
				}
				if len(refs) == 0 && hook == "session-start" {
					t.Fatalf("con max, el contexto del hook debería salir comprimido:\n%.600s", out)
				}
				for _, r := range refs {
					got := correrMem(t, bin, dir, env, "", "pack", "retrieve", r)
					if got.code != 0 || !strings.Contains(got.stdout, "ERROR lote id=217") {
						t.Errorf("ref %s no devuelve el original (code=%d)", r, got.code)
					}
				}
				if len(refs) > 0 && !strings.Contains(out, "pack_retrieve(ref=X)") {
					t.Error("falta la línea que explica cómo recuperar")
				}
			})
		}
	}
}
