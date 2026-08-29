package main

import (
	"os/exec"
	"strings"
	"testing"
)

// runReview ejecuta `mem review <args...>` contra un proyecto temporal y
// devuelve la salida combinada y si terminó con éxito.
func runReview(t *testing.T, dir string, args ...string) (string, bool) {
	t.Helper()
	cmd := exec.Command(buildPlanGuardBinary(t), append([]string{"review"}, args...)...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err == nil
}

// TestReviewCLI_ContratoDeConsulta cubre la Historia 4 sobre el binario real.
//
// Lo que este test protege es la propiedad que hace útil la auditoría: una
// revisión abierta muestra su ETAPA y no un veredicto. Confundir «va por la
// mitad» con «terminó sin defectos» es exactamente el error que la feature
// existe para impedir, y sería invisible en un unitario del repositorio.
func TestReviewCLI_ContratoDeConsulta(t *testing.T) {
	dir := t.TempDir()

	salida, ok := runReview(t, dir, "--file", ".")
	if !ok {
		t.Fatalf("mem review --file .: %s", salida)
	}
	reviewID := strings.TrimSpace(strings.SplitN(salida, "\n", 2)[0])
	if reviewID == "" {
		t.Fatalf("no se imprimió el review_id:\n%s", salida)
	}

	t.Run("status muestra etapa, no veredicto", func(t *testing.T) {
		out, ok := runReview(t, dir, "status", reviewID)
		if !ok {
			t.Fatalf("mem review status: %s", out)
		}
		for _, terminal := range []string{"APPROVED", "ESCALATED", "INCOMPLETE"} {
			if strings.Contains(out, terminal) {
				t.Errorf("una revisión recién abierta reporta el veredicto %q:\n%s", terminal, out)
			}
		}
		if !strings.Contains(out, reviewID) {
			t.Errorf("la salida no identifica la revisión consultada:\n%s", out)
		}
	})

	t.Run("history lista la revisión", func(t *testing.T) {
		out, ok := runReview(t, dir, "history")
		if !ok {
			t.Fatalf("mem review history: %s", out)
		}
		if !strings.Contains(out, reviewID) {
			t.Errorf("el historial no incluye la revisión creada:\n%s", out)
		}
	})

	t.Run("show reconstruye el linaje", func(t *testing.T) {
		out, ok := runReview(t, dir, "show", reviewID)
		if !ok {
			t.Fatalf("mem review show: %s", out)
		}
		for _, seccion := range []string{"target", "revisores", "consenso", "veredicto"} {
			if !strings.Contains(strings.ToLower(out), seccion) {
				t.Errorf("el detalle no expone %q; el linaje queda incompleto:\n%s", seccion, out)
			}
		}
	})

	t.Run("un review-id inexistente es error, no una revisión vacía", func(t *testing.T) {
		for _, sub := range []string{"status", "show"} {
			out, ok := runReview(t, dir, sub, "acr_no_existe")
			if ok {
				t.Errorf("`review %s acr_no_existe` terminó con éxito:\n%s", sub, out)
			}
		}
	})
}
