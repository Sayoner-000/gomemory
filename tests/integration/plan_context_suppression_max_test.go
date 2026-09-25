package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const avisoSupresion = "El historial del proyecto ya está disponible en esta sesión"

// T027 — regresión de I1: con el nivel max, get_plan_context sigue suprimiendo
// el historial ya entregado por cualquier vía. Si alguna vía registrara el hash
// de su salida comprimida en vez del documento crudo, esta prueba fallaría.
func TestPlanContextSuppression_Max(t *testing.T) {
	bin := buildMemBinary(t)
	log, err := os.ReadFile(filepath.Join(repoRootIntegration(t), "tests", "testdata", "compression_corpus", "gotest-fail.log"))
	if err != nil {
		t.Fatal(err)
	}
	for _, nivel := range []string{"max", "structural"} {
		for _, via := range []string{"cli", "mcp", "hook"} {
			t.Run(nivel+"/"+via, func(t *testing.T) {
				env := storeAislado(t)
				// budget -1: memorias íntegras en el contexto, para que el motor
				// SÍ comprima y el hash de la salida difiera del crudo.
				dir := proyectoCompresion(t, `{"context_compression_level": "`+nivel+`", "budget": -1}`)
				correrMem(t, bin, dir, env, "", "session", "start")
				correrMem(t, bin, dir, env, "", "save", "-t", "log grande", "-y", "bugfix", string(log))
				switch via {
				case "cli":
					out := correrMem(t, bin, dir, env, "", "context").stdout
					if nivel == "max" && !strings.Contains(out, "⟦mem⟧") {
						t.Fatal("control: con max el contexto debería salir comprimido")
					}
				case "mcp":
					r := mcpLlamadas(t, bin, dir, env, [2]string{"get_context", "{}"})
					if r[0].IsError {
						t.Fatalf("get_context: %s", r[0].Text)
					}
				case "hook":
					correrMem(t, bin, dir, env, "{}", "hook", "session-start")
				}
				plan := correrMem(t, bin, dir, env, "", "plan-context").stdout
				if !strings.Contains(plan, avisoSupresion) {
					t.Errorf("get_plan_context debería suprimir el historial ya entregado por %s con %s", via, nivel)
				}
			})
		}
	}
}
