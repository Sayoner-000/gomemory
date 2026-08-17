package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var (
	planGuardBinOnce sync.Once
	planGuardBinPath string
	planGuardBinErr  error
)

// buildPlanGuardBinary compila el binario una sola vez para todos los tests
// de plan-guard de este paquete (mismo patrón que buildMemBinary en
// tests/integration/hook_marker_integration_test.go).
func buildPlanGuardBinary(t *testing.T) string {
	t.Helper()
	planGuardBinOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gomemory-plan-guard-bin-*")
		if err != nil {
			planGuardBinErr = err
			return
		}
		planGuardBinPath = filepath.Join(dir, "mem-plan-guard-test-bin")
		cmd := exec.Command("go", "build", "-o", planGuardBinPath, "./infrastructure")
		cmd.Dir = repoRootContract(t)
		out, err := cmd.CombinedOutput()
		if err != nil {
			planGuardBinErr = err
			t.Logf("go build output: %s", out)
		}
	})
	if planGuardBinErr != nil {
		t.Fatalf("compilar binario: %v", planGuardBinErr)
	}
	return planGuardBinPath
}

// planGuardResult es el resultado crudo de invocar `mem hook plan-guard`
// contra el binario real, para poder afirmar sobre exit code, stdout y
// stderr por separado — el contrato neutral distingue los tres.
type planGuardResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runPlanGuard(t *testing.T, bin, dir string, stdin string, extraArgs ...string) planGuardResult {
	t.Helper()
	args := append([]string{"hook", "plan-guard"}, extraArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewBufferString(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("ejecutar plan-guard: %v", err)
		}
	}
	return planGuardResult{exitCode: exitCode, stdout: stdout.String(), stderr: stderr.String()}
}

const planEnArbol = `🎯 objetivo de prueba
├─ [1] subtarea
│  └─ [1.1] ✓ verbo + objeto → resultado verificable
└─ [2] ✓ otra subtarea → resultado verificable   (∥)`

const planEnProsaLarga = `Voy a implementar la integración completa revisando todo el código
relevante y haciendo los cambios necesarios en varios archivos del proyecto
para que todo funcione correctamente de principio a fin sin dejar nada
pendiente ni ningún caso sin cubrir en el flujo principal de la aplicación.`

// TestPlanGuardNeutral_DevuelveConCodigoDeSalidaYMotivoPorStderr cubre el
// dialecto neutral (contracts/hook-plan-guard.md): sin envoltura reconocible,
// un plan en prosa se devuelve con código de salida != 0, stdout vacío, y el
// motivo por stderr.
func TestPlanGuardNeutral_DevuelveConCodigoDeSalidaYMotivoPorStderr(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, `{"plan":"`+planEnProsaLarga+`"}`)

	if res.exitCode == 0 {
		t.Error("dialecto neutral + plan sin forma debe salir con código != 0")
	}
	if res.stdout != "" {
		t.Errorf("dialecto neutral no debe escribir stdout, got %q", res.stdout)
	}
	if res.stderr == "" {
		t.Error("dialecto neutral debe llevar el motivo por stderr")
	}
}

// TestPlanGuardNeutral_PermiteArbol cubre el camino de permiso: un plan con
// forma de árbol no se devuelve, exit 0, sin salida.
func TestPlanGuardNeutral_PermiteArbol(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, `{"plan":"`+jsonEscape(planEnArbol)+`"}`)

	if res.exitCode != 0 {
		t.Errorf("un plan en árbol debe permitirse (exit 0), got %d, stderr=%q", res.exitCode, res.stderr)
	}
	if res.stdout != "" || res.stderr != "" {
		t.Errorf("permitir debe ser silencioso, got stdout=%q stderr=%q", res.stdout, res.stderr)
	}
}

// TestPlanGuardNeutral_TextoPlanoEquivaleAJSON cubre
// contracts/agent-integration.md «Entrada»: texto plano por stdin produce el
// mismo veredicto que {"plan":"..."}.
func TestPlanGuardNeutral_TextoPlanoEquivaleAJSON(t *testing.T) {
	bin := buildPlanGuardBinary(t)

	dirJSON := t.TempDir()
	resJSON := runPlanGuard(t, bin, dirJSON, `{"plan":"`+planEnProsaLarga+`"}`)

	dirText := t.TempDir()
	resText := runPlanGuard(t, bin, dirText, planEnProsaLarga)

	if (resJSON.exitCode == 0) != (resText.exitCode == 0) {
		t.Errorf("texto plano y JSON deben producir el mismo veredicto: exit JSON=%d texto=%d",
			resJSON.exitCode, resText.exitCode)
	}
}

// TestPlanGuardNeutral_PlanTrivialSiemprePermite cubre FR-003: un plan
// trivial de un solo paso nunca se devuelve, incluso en dialecto neutral.
func TestPlanGuardNeutral_PlanTrivialSiemprePermite(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, `{"plan":"Cambiar el título del README."}`)

	if res.exitCode != 0 {
		t.Errorf("un plan trivial nunca debe devolverse, exit=%d stderr=%q", res.exitCode, res.stderr)
	}
}

// TestPlanGuardNeutral_SinEnvolturaNuncaResponeEnDialectoDeAgente cubre
// INV-6: sin --emit y sin envoltura reconocible, la respuesta es neutral —
// nunca el JSON de Claude Code ni de ningún otro dialecto.
func TestPlanGuardNeutral_SinEnvolturaNuncaResponeEnDialectoDeAgente(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, `{"plan":"`+planEnProsaLarga+`"}`)

	if res.stdout != "" {
		t.Errorf("la respuesta neutral no debe llevar JSON en stdout, got %q", res.stdout)
	}
}

// runPlanApproved invoca `mem hook plan-approved` (feature 007, ya
// existente) para cerrar un episodio de plan en las pruebas de plan-guard.
func runPlanApproved(t *testing.T, bin, dir, stdin string) planGuardResult {
	t.Helper()
	cmd := exec.Command(bin, "hook", "plan-approved")
	cmd.Dir = dir
	cmd.Stdin = bytes.NewBufferString(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("ejecutar plan-approved: %v", err)
		}
	}
	return planGuardResult{exitCode: exitCode, stdout: stdout.String(), stderr: stderr.String()}
}

// writePlanGuardDisabledSettings escribe un settings.json mínimo con
// plan_guard_disabled=true en dir, para probar FR-004.
func writePlanGuardDisabledSettings(t *testing.T, dir string) {
	t.Helper()
	memDir := filepath.Join(dir, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir .memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "settings.json"),
		[]byte(`{"plan_guard_disabled":true}`), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}
}

// assertNoPlanEpisodeMarker verifica que el marcador de episodio de plan no
// se haya escrito — con la exigencia apagada, plan-guard no debe tocar
// estado (contracts/hook-plan-guard.md, Caso A).
func assertNoPlanEpisodeMarker(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".memory", ".plan-episode-denials")
	if _, err := os.Stat(path); err == nil {
		t.Error("con la exigencia apagada no debe escribirse el marcador de episodio")
	}
}

func jsonEscape(s string) string {
	out := make([]byte, 0, len(s))
	for _, r := range s {
		switch r {
		case '"':
			out = append(out, '\\', '"')
		case '\\':
			out = append(out, '\\', '\\')
		case '\n':
			out = append(out, '\\', 'n')
		default:
			out = append(out, []byte(string(r))...)
		}
	}
	return string(out)
}
