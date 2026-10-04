package main

import (
	"encoding/json"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
)

// T027 (feature 035, FR-008): en Claude Code —que tiene señal observable de
// entrada a plan— el recordatorio de modo plan sale al inicio, tras compactar o
// al entrar en plan; no en cada turno. En Codex (dialecto json) sigue en cada
// turno, como decidió la feature 019 para los agentes sin esa señal.

const recordatorioPlan = "Si vas a entrar en modo plan"

func promptCtx(t *testing.T, out string) string {
	t.Helper()
	out = strings.TrimSpace(out)
	if out == "{}" || out == "" {
		return ""
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatalf("salida no JSON: %v\n%s", err, out)
	}
	hso, _ := p["hookSpecificOutput"].(map[string]any)
	ctx, _ := hso["additionalContext"].(string)
	return ctx
}

func TestPlanReminder_ClaudeSoloAlInicioYAlEntrarEnPlan(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	up := func(stdin string) string {
		return promptCtx(t, runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit"}, stdin))
	}

	up(`{"prompt":"uno"}`) // primer prompt: bootstrap + protocolo
	if ctx := up(`{"prompt":"dos"}`); strings.Contains(ctx, recordatorioPlan) {
		t.Errorf("turno 2 sin cambio de estado: no debe repetir el recordatorio de plan: %q", ctx)
	}
	if ctx := up(`{"prompt":"tres","permission_mode":"plan"}`); !strings.Contains(ctx, recordatorioPlan) && !strings.Contains(ctx, "Descomposición Atómica") {
		t.Errorf("al entrar en plan por primera vez debe llegar el recordatorio o el documento: %q", ctx)
	}
	if ctx := up(`{"prompt":"cuatro","permission_mode":"plan"}`); strings.Contains(ctx, recordatorioPlan) {
		t.Errorf("seguir en plan no es un cambio de estado: %q", ctx)
	}

	runHookWithStdin(t, bin, target, "post-compact", `{}`)
	if ctx := up(`{"prompt":"cinco"}`); !strings.Contains(ctx, recordatorioPlan) {
		t.Errorf("tras compactar, el primer prompt vuelve a incluirlo: %q", ctx)
	}
}

func TestPlanReminder_CodexEnCadaTurno(t *testing.T) {
	bin := buildMemBinary(t)
	target := dirDeProyecto(t)
	if err := persistence.EnsureDir(target); err != nil {
		t.Fatal(err)
	}
	for i, p := range []string{`{"prompt":"a"}`, `{"prompt":"b"}`, `{"prompt":"c"}`} {
		out := runHookWithStdinArgs(t, bin, target, []string{"hook", "user-prompt-submit", "--emit=json"}, p)
		if !strings.Contains(promptCtx(t, out), recordatorioPlan) {
			t.Errorf("Codex (sin señal de plan) debe recibirlo en cada turno; turno %d: %q", i+1, out)
		}
	}
}

// T029 (feature 035, FR-007): el documento de plan del primer prompt no repite
// la memoria del proyecto si el arranque de la sesión ya la entregó.
func TestPlanDoc_NoRepiteLaMemoriaYaEntregada(t *testing.T) {
	bin := buildMemBinary(t)

	conArranque := dirDeProyecto(t)
	if err := persistence.EnsureDir(conArranque); err != nil {
		t.Fatal(err)
	}
	runMem(t, bin, conArranque, "save", "-t", "Marcador de memoria 035", "-y", "decision", "contenido marcador para el plan-doc de la feature 035")
	runHookWithStdin(t, bin, conArranque, "session-start", `{"session_id":"pd-1"}`)
	ctx := promptCtx(t, runHookWithStdinArgs(t, bin, conArranque, []string{"hook", "user-prompt-submit"}, `{"prompt":"planifica","permission_mode":"plan"}`))
	if !strings.Contains(ctx, "Descomposición Atómica") {
		t.Fatalf("el documento de plan debía entregarse: %q", ctx)
	}
	if strings.Contains(ctx, "# Memoria del Proyecto") {
		t.Errorf("la memoria ya entregada en el arranque no debe repetirse en el plan-doc")
	}

	sinArranque := dirDeProyecto(t)
	if err := persistence.EnsureDir(sinArranque); err != nil {
		t.Fatal(err)
	}
	runMem(t, bin, sinArranque, "save", "-t", "Marcador de memoria 035", "-y", "decision", "contenido marcador para el plan-doc de la feature 035")
	ctx = promptCtx(t, runHookWithStdinArgs(t, bin, sinArranque, []string{"hook", "user-prompt-submit"}, `{"prompt":"planifica","permission_mode":"plan"}`))
	if !strings.Contains(ctx, "# Memoria del Proyecto") {
		t.Errorf("sin arranque previo, el plan-doc lleva la memoria (dentro del tope)")
	}
}
