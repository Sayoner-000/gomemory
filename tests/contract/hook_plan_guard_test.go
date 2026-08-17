package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// claudeEnvelope construye el payload real que Claude Code envía a
// PreToolUse(ExitPlanMode), verificado en vivo (feature 019, T001-T004):
// hook_event_name + tool_name + tool_input.plan.
func claudeEnvelope(plan string) string {
	env := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "ExitPlanMode",
		"tool_input":      map[string]any{"plan": plan},
	}
	b, _ := json.Marshal(env)
	return string(b)
}

func parseClaudeDeny(t *testing.T, stdout string) (denied bool, reason string) {
	t.Helper()
	if strings.TrimSpace(stdout) == "{}" || stdout == "" {
		return false, ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout no es JSON válido: %v (%q)", err, stdout)
	}
	hso, _ := parsed["hookSpecificOutput"].(map[string]any)
	if hso == nil {
		return false, ""
	}
	reason, _ = hso["permissionDecisionReason"].(string)
	return hso["permissionDecision"] == "deny", reason
}

// TestHookPlanGuardClaude_ProsaLargaSeDevuelveConMotivo cubre el caso base:
// un plan en prosa larga, con la envoltura real de Claude Code, se devuelve
// con permissionDecision=deny y un motivo no vacío. exit 0 (el dialecto
// claude transporta la decisión en la salida, no en el código).
func TestHookPlanGuardClaude_ProsaLargaSeDevuelveConMotivo(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))

	if res.exitCode != 0 {
		t.Errorf("el dialecto claude siempre sale con 0, got %d (stderr=%q)", res.exitCode, res.stderr)
	}
	denied, reason := parseClaudeDeny(t, res.stdout)
	if !denied {
		t.Fatalf("un plan en prosa larga debe devolverse, stdout=%q", res.stdout)
	}
	if reason == "" {
		t.Error("el motivo de la devolución no debe estar vacío")
	}
}

// TestHookPlanGuardClaude_ArbolSePermite cubre el camino de permiso: {}.
func TestHookPlanGuardClaude_ArbolSePermite(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, claudeEnvelope(planEnArbol))

	denied, _ := parseClaudeDeny(t, res.stdout)
	if denied {
		t.Errorf("un plan en árbol no debe devolverse, stdout=%q", res.stdout)
	}
}

// TestHookPlanGuardClaude_PlanTrivialSePermite cubre FR-003.
func TestHookPlanGuardClaude_PlanTrivialSePermite(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, claudeEnvelope("Cambiar el título del README."))

	denied, _ := parseClaudeDeny(t, res.stdout)
	if denied {
		t.Errorf("un plan trivial nunca debe devolverse, stdout=%q", res.stdout)
	}
}

// TestHookPlanGuardClaude_SegundaInvocacionYaNoDevuelve cubre FR-002: como
// máximo una devolución por episodio. La misma prosa, invocada dos veces
// seguidas en el mismo directorio (mismo episodio), solo se devuelve la
// primera vez.
func TestHookPlanGuardClaude_SegundaInvocacionYaNoDevuelve(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	first := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	deniedFirst, _ := parseClaudeDeny(t, first.stdout)
	if !deniedFirst {
		t.Fatalf("la primera invocación debe devolver el plan, stdout=%q", first.stdout)
	}

	second := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	deniedSecond, _ := parseClaudeDeny(t, second.stdout)
	if deniedSecond {
		t.Error("la segunda invocación del mismo episodio no debe volver a devolver el plan (FR-002)")
	}
}

// TestHookPlanGuardClaude_TrasPlanApprovedVuelveADevolver cubre el cierre y
// reapertura de episodio: tras `plan-approved`, un plan en prosa vuelve a
// producir una devolución (episodio nuevo).
func TestHookPlanGuardClaude_TrasPlanApprovedVuelveADevolver(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	first := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	deniedFirst, _ := parseClaudeDeny(t, first.stdout)
	if !deniedFirst {
		t.Fatalf("la primera invocación debe devolver el plan, stdout=%q", first.stdout)
	}

	// Cierra el episodio: aprobar el plan (mismo binario, subcomando ya
	// existente de la feature 007).
	approveRes := runPlanApproved(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	if approveRes.exitCode != 0 {
		t.Fatalf("plan-approved debe salir con 0, got %d", approveRes.exitCode)
	}

	third := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))
	deniedThird, _ := parseClaudeDeny(t, third.stdout)
	if !deniedThird {
		t.Error("tras plan-approved, el episodio se reinicia: el mismo plan en prosa debe volver a devolverse")
	}
}

// TestHookPlanGuardClaude_FormasDePayloadEquivalentes cubre que
// tool_input.plan y plan de nivel superior producen el mismo resultado,
// forzando el dialecto claude en ambos casos para aislar la comparación del
// mecanismo de detección automática (ya cubierto en hook_dialect_test.go).
func TestHookPlanGuardClaude_FormasDePayloadEquivalentes(t *testing.T) {
	bin := buildPlanGuardBinary(t)

	nested := claudeEnvelope(planEnProsaLarga)
	dirNested := t.TempDir()
	resNested := runPlanGuard(t, bin, dirNested, nested, "--emit=claude")
	deniedNested, _ := parseClaudeDeny(t, resNested.stdout)

	topLevel := `{"plan":"` + planEnProsaLarga + `"}`
	dirTopLevel := t.TempDir()
	resTopLevel := runPlanGuard(t, bin, dirTopLevel, topLevel, "--emit=claude")
	deniedTopLevel, _ := parseClaudeDeny(t, resTopLevel.stdout)

	if deniedNested != deniedTopLevel {
		t.Errorf("tool_input.plan (denied=%v) y plan de nivel superior (denied=%v) deben coincidir",
			deniedNested, deniedTopLevel)
	}
}

// TestHookPlanGuardClaude_JSONInvalidoPermite cubre robustez: un payload que
// no es JSON válido ni texto con forma de árbol se trata según el contenido
// crudo (texto plano); forzado a --emit=claude, con contenido corto no debe
// bloquear.
func TestHookPlanGuardClaude_JSONInvalidoPermite(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()

	res := runPlanGuard(t, bin, dir, `{esto no es JSON válido`, "--emit=claude")

	denied, _ := parseClaudeDeny(t, res.stdout)
	if denied {
		t.Errorf("un payload sin plan reconocible nunca debe devolverse, stdout=%q", res.stdout)
	}
}

// TestHookPlanGuardClaude_InterruptorApagadoSiemprePermite cubre FR-004: con
// plan_guard_disabled=true, incluso una prosa larga se permite, sin escribir
// el marcador de episodio.
func TestHookPlanGuardClaude_InterruptorApagadoSiemprePermite(t *testing.T) {
	bin := buildPlanGuardBinary(t)
	dir := t.TempDir()
	writePlanGuardDisabledSettings(t, dir)

	res := runPlanGuard(t, bin, dir, claudeEnvelope(planEnProsaLarga))

	denied, _ := parseClaudeDeny(t, res.stdout)
	if denied {
		t.Errorf("con la exigencia apagada nunca debe devolverse, stdout=%q", res.stdout)
	}
	assertNoPlanEpisodeMarker(t, dir)
}
