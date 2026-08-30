package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

// gitInit prepara un repositorio con un commit inicial para poder probar el
// congelado de cambios pendientes contra git real.
func gitInit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q", "."},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
}

func gitCommitAll(t *testing.T, dir, mensaje string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", mensaje}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %s", args, out)
		}
	}
}

func digestDeSalida(t *testing.T, salida string) string {
	t.Helper()
	for _, linea := range strings.Split(salida, "\n") {
		if strings.HasPrefix(linea, "target_digest: ") {
			return strings.TrimPrefix(linea, "target_digest: ")
		}
	}
	t.Fatalf("la salida no declara target_digest:\n%s", salida)
	return ""
}

// TestReviewCLI_PendingCongelaTodoElTrabajo cubre FR-025 y SC-004.
//
// El motivo de que exista --pending: --diff usa `git diff --binary`, que NO ve los
// archivos sin seguimiento. Una revisión de trabajo en curso con archivos recién
// creados congelaba un target que no los contenía, y los revisores inspeccionaban
// menos de lo que se creía.
func TestReviewCLI_PendingCongelaTodoElTrabajo(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	escribir(t, dir, "seguido.txt", "base")
	gitCommitAll(t, dir, "inicial")

	// Los tres estados a la vez: preparado, sin preparar y nuevo sin seguimiento.
	escribir(t, dir, "seguido.txt", "base modificada")
	escribir(t, dir, "preparado.txt", "contenido")
	cmd := exec.Command("git", "add", "preparado.txt")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %s", out)
	}
	// Un nombre con espacios: rompe cualquier parseo por líneas de git status.
	escribir(t, dir, "archivo nuevo con espacios.txt", "nuevo")
	escribir(t, dir, ".gitignore", "ignorado.txt\n")
	escribir(t, dir, "ignorado.txt", "no debe contar")

	salida, ok := runReview(t, dir, "--pending")
	if !ok {
		t.Fatalf("mem review --pending: %s", salida)
	}
	if !strings.Contains(salida, "target_files: 4") {
		t.Errorf("se esperaban 4 archivos (3 cambios + .gitignore), salida:\n%s", salida)
	}
	primero := digestDeSalida(t, salida)

	// Reproducible: sin tocar nada, el mismo digest.
	repetida, ok := runReview(t, dir, "--pending")
	if !ok {
		t.Fatalf("mem review --pending (repetida): %s", repetida)
	}
	if digestDeSalida(t, repetida) != primero {
		t.Error("el digest de cambios pendientes no es reproducible")
	}

	// Cambiar el contenido de un archivo con espacios cambia la identidad.
	escribir(t, dir, "archivo nuevo con espacios.txt", "nuevo y distinto")
	tercera, ok := runReview(t, dir, "--pending")
	if !ok {
		t.Fatalf("mem review --pending (modificada): %s", tercera)
	}
	if digestDeSalida(t, tercera) == primero {
		t.Error("modificar un archivo pendiente no cambió el digest del target")
	}

	// Borrar un archivo seguido también cambia la identidad.
	if err := os.Remove(filepath.Join(dir, "seguido.txt")); err != nil {
		t.Fatal(err)
	}
	cuarta, ok := runReview(t, dir, "--pending")
	if !ok {
		t.Fatalf("mem review --pending (borrado): %s", cuarta)
	}
	if digestDeSalida(t, cuarta) == digestDeSalida(t, tercera) {
		t.Error("borrar un archivo no cambió el digest del target")
	}
}

// TestReviewCLI_PendingRechazaArbolLimpio cubre FR-026.
func TestReviewCLI_PendingRechazaArbolLimpio(t *testing.T) {
	dir := t.TempDir()
	gitInit(t, dir)
	escribir(t, dir, "a.txt", "contenido")
	gitCommitAll(t, dir, "inicial")

	salida, ok := runReview(t, dir, "--pending")
	if ok {
		t.Fatalf("un árbol limpio no puede congelar un target:\n%s", salida)
	}
	if !strings.Contains(salida, "no hay cambios pendientes") {
		t.Errorf("el diagnóstico no explica la causa:\n%s", salida)
	}
}

// TestReviewCLI_ReadOnlyDeclaraElAlcance cubre FR-018 sobre el binario real.
func TestReviewCLI_ReadOnlyDeclaraElAlcance(t *testing.T) {
	dir := t.TempDir()
	escribir(t, dir, "a.txt", "contenido")

	salida, ok := runReview(t, dir, "--file", ".", "--read-only")
	if !ok {
		t.Fatalf("mem review --file . --read-only: %s", salida)
	}
	if !strings.Contains(salida, "fix_authorized: false") {
		t.Errorf("--read-only debe declarar que no autoriza corregir:\n%s", salida)
	}

	autorizada, ok := runReview(t, dir, "--file", ".")
	if !ok {
		t.Fatalf("mem review --file .: %s", autorizada)
	}
	if !strings.Contains(autorizada, "fix_authorized: true") {
		t.Errorf("sin --read-only la revisión autoriza corregir:\n%s", autorizada)
	}
	// La política efectiva se imprime siempre: hasta la funcionalidad 028 salía de
	// constantes del código y no había forma de saber con qué reglas quedó
	// congelada la revisión.
	if !strings.Contains(autorizada, "max_fix_rounds:") ||
		!strings.Contains(autorizada, "auto_fix_severities:") {
		t.Errorf("la salida debe declarar la política efectiva:\n%s", autorizada)
	}
}

func escribir(t *testing.T, dir, nombre, contenido string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, nombre), []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}
