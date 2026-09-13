package cli

import (
	"errors"
	"os"
	"strings"
	"testing"
)

type planPromptBrokenContext struct{}

func (planPromptBrokenContext) Build() (string, error) { return "", errors.New("almacén bloqueado") }
func (planPromptBrokenContext) WriteFile() error       { return nil }

// S-002 (revisión 031): sin historial, el documento no debe presentarse como
// completo. Lo declara y apunta a get_plan_context() para reintentarlo.
func TestPlanEntryFromPrompt_SinHistorialLoDeclara(t *testing.T) {
	deps, _, root := newPlanPromptDeps(t, false)
	deps.ContextBuilder = planPromptBrokenContext{}

	doc, ok := planEntryFromPrompt(deps, root, map[string]any{"permission_mode": "plan"})
	if !ok || !strings.Contains(doc, "MÉTODO DE DESCOMPOSICIÓN") {
		t.Fatalf("el método se entrega igual: ok=%t doc=%q", ok, doc)
	}
	if !strings.Contains(doc, "Historial del proyecto no disponible") || !strings.Contains(doc, "get_plan_context()") {
		t.Fatalf("la falta de historial debe declararse con su vía de reintento: %q", doc)
	}
}

// S-003 (3ª revisión 031): las dos entradas a plan (hook plan-entered y prompt en
// modo plan) reclaman el marcador con el mismo helper, que dice si pudo
// registrarlo. Sin registro, ninguna debe reentregar el documento completo.
func TestClaimPlanEntryMarker(t *testing.T) {
	deps, _, root := newPlanPromptDeps(t, false)
	if !claimPlanEntryMarker(deps, root) {
		t.Fatal("con el directorio escribible debe registrar la entrega")
	}
	if _, err := os.Stat(planEnteredMarkerPath(deps, root)); err != nil {
		t.Fatalf("el marcador debe existir: %v", err)
	}
	if os.Geteuid() == 0 {
		return
	}
	readOnly := t.TempDir()
	if err := os.Chmod(readOnly, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
	if claimPlanEntryMarker(deps, readOnly) {
		t.Fatal("sin poder escribir debe informar que no registró la entrega")
	}
}

// S-002 (4ª revisión 031): el episodio de plan-guard solo se reinicia cuando se
// registra de verdad una entrada nueva. Sin marcador, cada prompt en modo plan
// lo reiniciaría y el contador de devoluciones nunca acumularía.
func TestPlanEntryFromPrompt_SinMarcadorNoReiniciaElEpisodio(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignora los permisos de directorio")
	}
	deps, _, root := newPlanPromptDeps(t, false)
	// El episodio vive en root/.memory (escribible); el marcador del fake, en
	// root, que queda de solo lectura.
	planEpisodeMarkDenied(root)
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	if _, ok := planEntryFromPrompt(deps, root, map[string]any{"permission_mode": "plan"}); !ok {
		t.Fatal("en modo plan debe responder (con el recordatorio)")
	}
	if !planEpisodeDenied(root) {
		t.Fatal("sin registrar la entrada, el episodio no debe reiniciarse")
	}
}

// S-001 (revisión 031): sin marcador no hay "una vez por sesión", y cada prompt
// en modo plan reinyectaría el documento completo. Se degrada al recordatorio.
func TestPlanEntryFromPrompt_SinMarcadorDegradaAlRecordatorio(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignora los permisos de directorio")
	}
	deps, _, root := newPlanPromptDeps(t, false)
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o700) })

	for i := 0; i < 2; i++ {
		doc, ok := planEntryFromPrompt(deps, root, map[string]any{"permission_mode": "plan"})
		if !ok || strings.Contains(doc, "MÉTODO DE DESCOMPOSICIÓN") || !strings.Contains(doc, "get_plan_context()") {
			t.Fatalf("prompt %d: sin marcador debe entregar solo el recordatorio: ok=%t doc=%q", i, ok, doc)
		}
	}
}
