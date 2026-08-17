package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// runPlanEntered invoca `mem hook plan-entered` contra el binario real.
func runPlanEntered(t *testing.T, bin, dir string, extraArgs ...string) planGuardResult {
	t.Helper()
	args := append([]string{"hook", "plan-entered"}, extraArgs...)
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("ejecutar plan-entered: %v", err)
		}
	}
	return planGuardResult{exitCode: exitCode, stdout: stdout.String(), stderr: stderr.String()}
}

// extractAdditionalContext extrae additionalContext de un stdout en dialecto
// claude ({} o {"hookSpecificOutput":{"additionalContext":"..."}}).
func extractAdditionalContext(t *testing.T, stdout string) string {
	t.Helper()
	if stdout == "" || stdout == "{}" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout no es JSON válido: %v (%q)", err, stdout)
	}
	hso, _ := parsed["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		return ""
	}
	ctx, _ := hso["additionalContext"].(string)
	return ctx
}

func claudeEnteredEnvelope() string {
	env := map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "EnterPlanMode",
		"tool_input":      map[string]any{},
	}
	b, _ := json.Marshal(env)
	return string(b)
}

// gitInitProject crea un directorio temporal con `.git`, para que FindRoot lo
// reconozca como proyecto propio en vez de resolver a un ancestro compartido.
func gitInitProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	return dir
}

// TestHookPlanEntered_SalidaClaudeNoExcedeElTope cubre el presupuesto por
// defecto (9500) en el dialecto claude, con el método completo presente.
func TestHookPlanEntered_SalidaClaudeNoExcedeElTope(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := gitInitProject(t)

	res := runPlanEntered(t, bin, dir, "--emit=claude")
	if res.exitCode != 0 {
		t.Fatalf("plan-entered debe salir con 0, got %d (stderr=%q)", res.exitCode, res.stderr)
	}
	ctx := extractAdditionalContext(t, res.stdout)
	if len(res.stdout) > 9500+200 { // margen por el envoltorio JSON (claves, comillas)
		t.Errorf("la salida (%d chars) excede holgadamente el tope de 9500", len(res.stdout))
	}
	if ctx == "" {
		t.Fatal("se esperaba un documento no vacío en additionalContext")
	}
}

// TestHookPlanEntered_SalidaNeutralYJSON cubre los otros dos dialectos
// documentados para este hook.
func TestHookPlanEntered_SalidaNeutralYJSON(t *testing.T) {
	bin := buildPlanGuardBinary(t)

	dirNeutral := gitInitProject(t)
	resNeutral := runPlanEntered(t, bin, dirNeutral, "--emit=neutral")
	if resNeutral.exitCode != 0 || resNeutral.stdout == "" {
		t.Errorf("neutral: exit=%d stdout=%q", resNeutral.exitCode, resNeutral.stdout)
	}

	dirJSON := gitInitProject(t)
	resJSON := runPlanEntered(t, bin, dirJSON, "--emit=json")
	var parsed map[string]any
	if err := json.Unmarshal([]byte(resJSON.stdout), &parsed); err != nil {
		t.Fatalf("json: stdout no es JSON válido: %v (%q)", err, resJSON.stdout)
	}
	if _, ok := parsed["context"]; !ok {
		t.Errorf("json: falta el campo context, got %v", parsed)
	}
}

// seedMemories guarda varias memorias reales en dir vía `mem save`, para que
// el historial que compone ContextBuilder tenga contenido de sobra que
// recortar. Sin esto, un proyecto recién creado no tiene nada que trimear y
// la comparación entre presupuestos queda vacía de contenido.
func seedMemories(t *testing.T, bin, dir string) {
	t.Helper()
	for i := 0; i < 8; i++ {
		content := "Decisión de arquitectura de prueba número " + string(rune('A'+i)) +
			" con bastante texto de relleno para que el historial tenga volumen real que recortar entre presupuestos distintos."
		cmd := exec.Command(bin, "save", "-t", "Decisión de prueba "+string(rune('A'+i)), "-y", "decision", content)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("mem save: %v\n%s", err, out)
		}
	}
}

// TestHookPlanEntered_BudgetCambiaElTope cubre que --budget es un parámetro
// real, no una constante fija: con historial real de sobra, un presupuesto
// menor produce un documento más corto que uno mayor — ambos por encima del
// tamaño propio del método (~4.2 KB, research.md §4), para que la
// comparación recaiga sobre el recorte del historial y no sobre el caso
// límite "el método solo ya no cabe".
func TestHookPlanEntered_BudgetCambiaElTope(t *testing.T) {
	bin := buildPlanGuardBinary(t)

	dirBig := gitInitProject(t)
	seedMemories(t, bin, dirBig)
	resBig := runPlanEntered(t, bin, dirBig, "--emit=neutral", "--budget=9500")

	dirSmall := gitInitProject(t)
	seedMemories(t, bin, dirSmall)
	resSmall := runPlanEntered(t, bin, dirSmall, "--emit=neutral", "--budget=5000")

	if len(resSmall.stdout) > 5000 {
		t.Errorf("con --budget=5000 la salida no debe exceder 5000 chars, got %d", len(resSmall.stdout))
	}
	if len(resBig.stdout) <= len(resSmall.stdout) {
		t.Errorf("un budget mayor debe producir una salida más larga cuando hay historial de sobra: big=%d small=%d",
			len(resBig.stdout), len(resSmall.stdout))
	}
}

// TestHookPlanEntered_AtomicPlanDisabledProduceSilencio cubre el gate ya
// existente de la feature 013: con atomic_plan_disabled=true, silencio.
func TestHookPlanEntered_AtomicPlanDisabledProduceSilencio(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := gitInitProject(t)

	memDir := filepath.Join(dir, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir .memory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "settings.json"),
		[]byte(`{"atomic_plan_disabled":true}`), 0644); err != nil {
		t.Fatalf("write settings.json: %v", err)
	}

	res := runPlanEntered(t, bin, dir, "--emit=claude")
	if res.exitCode != 0 {
		t.Errorf("con atomic_plan_disabled debe salir con 0, got %d", res.exitCode)
	}
	if ctx := extractAdditionalContext(t, res.stdout); ctx != "" {
		t.Errorf("con atomic_plan_disabled no debe emitirse documento, got %q", ctx)
	}
}

// TestHookPlanEntered_SegundaInvocacionEsCorta cubre FR-008: reentrar en modo
// plan dentro de la misma sesión no repite el bloque completo.
func TestHookPlanEntered_SegundaInvocacionEsCorta(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := gitInitProject(t)

	first := runPlanEntered(t, bin, dir, "--emit=claude")
	firstCtx := extractAdditionalContext(t, first.stdout)
	if firstCtx == "" {
		t.Fatal("la primera invocación debe emitir el documento completo")
	}

	second := runPlanEntered(t, bin, dir, "--emit=claude")
	secondCtx := extractAdditionalContext(t, second.stdout)
	if len(secondCtx) >= len(firstCtx) {
		t.Errorf("la segunda invocación debe ser más corta que la primera: first=%d second=%d",
			len(firstCtx), len(secondCtx))
	}
}

// TestHookPlanEntered_ReiniciaElEpisodio cubre que entrar en modo plan abre
// un episodio nuevo (data-model.md §2): tras una devolución de plan-guard,
// plan-entered deja el contador en 0 de nuevo.
func TestHookPlanEntered_ReiniciaElEpisodio(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := gitInitProject(t)

	// Abre un episodio y lo consume con una devolución.
	runPlanEntered(t, bin, dir, "--emit=claude")
	prosaRes := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	denied, _ := parseClaudeDeny(t, prosaRes.stdout)
	if !denied {
		t.Fatal("precondición: la primera devolución debe ocurrir")
	}

	// Entrar en modo plan otra vez reinicia el episodio.
	runPlanEntered(t, bin, dir, "--emit=claude")
	again := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	deniedAgain, _ := parseClaudeDeny(t, again.stdout)
	if !deniedAgain {
		t.Error("plan-entered debe reiniciar el episodio: la prosa debe volver a devolverse")
	}
}
