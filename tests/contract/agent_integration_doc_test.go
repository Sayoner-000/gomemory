package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgentIntegrationDoc_MencionaLosDialectosYNivelesReales cubre T051: el
// documento publicado (docs/AGENT-INTEGRATION.md) no debe desincronizarse del
// comportamiento real. Verifica dos cosas: que el documento mencione los
// cuatro dialectos y los tres niveles, y que esos cuatro dialectos sean
// realmente aceptados por el binario (no solo texto aspiracional).
func TestAgentIntegrationDoc_MencionaLosDialectosYNivelesReales(t *testing.T) {
	docPath := filepath.Join(repoRootContract(t), "docs", "AGENT-INTEGRATION.md")
	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", docPath, err)
	}
	doc := string(data)

	for _, dialect := range []string{"neutral", "json", "claude", "text"} {
		if !strings.Contains(doc, "`"+dialect+"`") {
			t.Errorf("el documento no menciona el dialecto %q", dialect)
		}
	}
	for _, level := range []string{"Garantía de forma", "Contexto al planificar", "Piso textual"} {
		if !strings.Contains(doc, level) {
			t.Errorf("el documento no menciona el nivel %q", level)
		}
	}
	for _, cmd := range []string{"mem hook plan-guard", "mem hook plan-entered", "mem hook plan-approved", "mem hook nudge", "mem doctor"} {
		if !strings.Contains(doc, cmd) {
			t.Errorf("el documento no menciona el comando %q", cmd)
		}
	}

	// Los cuatro dialectos documentados son realmente aceptados por el
	// binario (no solo texto aspiracional): cada --emit produce una salida
	// bien formada para el mismo plan.
	bin := buildPlanGuardBinary(t)
	for _, dialect := range []string{"neutral", "json", "claude", "text"} {
		dir := t.TempDir()
		res := runPlanGuard(t, bin, dir, `{"plan":"Cambiar el título del README."}`, "--emit="+dialect)
		if res.exitCode != 0 {
			t.Errorf("--emit=%s con un plan trivial debe permitir (exit 0), got %d (stderr=%q)",
				dialect, res.exitCode, res.stderr)
		}
	}
}
