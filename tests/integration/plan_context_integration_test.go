package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// runPlanContext ejecuta `mem plan-context` en dir y devuelve salida y código.
// Se ejecuta contra el BINARIO construido, no contra el paquete: la regla de
// trabajo del proyecto exige verificar la ruta real, porque un test con dobles
// puede quedar verde mientras la cadena completa falla.
func runPlanContext(t *testing.T, dir string) (string, int) {
	t.Helper()
	cmd := exec.Command(buildMemBinary(t), "plan-context")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("ejecutar plan-context: %v", err)
	}
	return string(out), code
}

// TestPlanContext_ProyectoNuevo_EmiteElMetodo cubre la Historia 2 y FR-034
// (escenario 2 de quickstart.md): en un proyecto sin historial previo el método
// debe llegar igual. Es lo que mantiene la Historia 2 independiente de la
// Historia 1 — el valor no depende de tener memoria acumulada.
//
// Comportamiento real verificado: gomemory inicializa el store de forma perezosa
// al primer uso, así que en un directorio limpio ContextBuilder.Build() NO falla:
// devuelve un contexto mínimo (solo encabezados). Por eso aquí no se afirma que
// falte la sección de contexto. La rama estrictamente degradada —Build()
// devolviendo error, con salida de solo método— queda cubierta por el test
// unitario del caso de uso, que es donde puede provocarse de forma determinista.
func TestPlanContext_ProyectoNuevo_EmiteElMetodo(t *testing.T) {
	dir := t.TempDir()

	out, code := runPlanContext(t, dir)

	if code != 0 {
		t.Errorf("el código de salida debe ser 0 en un proyecto sin historial, fue %d", code)
	}
	if !strings.Contains(out, "Descomposición Atómica") {
		t.Errorf("sin historial debe emitirse el método; salida:\n%s", out)
	}
	if strings.Contains(strings.ToLower(out), "error") {
		t.Errorf("no debe aparecer ningún error visible; salida:\n%s", out)
	}
}

// TestPlanContext_ConMemoria_EmiteMetodoYContexto cubre la Historia 1: con
// memoria inicializada llegan las dos partes, y el método precede al contexto.
func TestPlanContext_ConMemoria_EmiteMetodoYContexto(t *testing.T) {
	dir := t.TempDir()
	if err := persistence.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	db, err := persistence.Open(dir)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	db.Close()

	out, code := runPlanContext(t, dir)

	if code != 0 {
		t.Errorf("código de salida = %d, se esperaba 0", code)
	}
	iMetodo := strings.Index(out, "Descomposición Atómica")
	iCtx := strings.Index(out, "Memoria del Proyecto")
	if iMetodo < 0 {
		t.Fatalf("falta el método en la salida:\n%s", out)
	}
	if iCtx < 0 {
		t.Fatalf("falta el contexto en la salida:\n%s", out)
	}
	if iMetodo > iCtx {
		t.Error("el método debe preceder al contexto (si el contexto se trunca, no debe perderse el método)")
	}
}

// TestPlanContext_Apagado_SalidaVacia cubre la Historia 4 y FR-032 (escenario 3
// de quickstart.md). Contrastado con el test "sin memoria", demuestra que las
// dos ramas de degradación son distintas: la ausencia de historial deja el
// método, el apagado explícito lo silencia todo.
func TestPlanContext_Apagado_SalidaVacia(t *testing.T) {
	dir := t.TempDir()
	if err := persistence.EnsureDir(dir); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	settings := filepath.Join(dir, ".memory", "settings.json")
	if err := os.WriteFile(settings, []byte(`{"atomic_plan_disabled":true}`), 0644); err != nil {
		t.Fatalf("escribir settings: %v", err)
	}

	out, code := runPlanContext(t, dir)

	if code != 0 {
		t.Errorf("código de salida = %d, se esperaba 0 incluso apagado", code)
	}
	if strings.TrimSpace(out) != "" {
		t.Errorf("con atomic_plan_disabled=true la salida debe ser vacía, se obtuvo:\n%s", out)
	}

	// Reactivar sin reinstalar debe restaurar el comportamiento.
	if err := os.WriteFile(settings, []byte(`{"atomic_plan_disabled":false}`), 0644); err != nil {
		t.Fatalf("reescribir settings: %v", err)
	}
	out, _ = runPlanContext(t, dir)
	if !strings.Contains(out, "Descomposición Atómica") {
		t.Error("al reactivar debe volver el método sin reinstalar nada")
	}
}
