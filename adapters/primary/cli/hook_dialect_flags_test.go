package cli

import "testing"

// Estos casos ya están cubiertos funcionalmente por tests/contract (subprocess
// contra el binario real), pero el instrumentador de cobertura de Go no ve
// ejecuciones cruzadas de proceso — este archivo cierra esa brecha de MÉTRICA
// con pruebas directas de las mismas funciones puras (T068).

func TestEmitFlagValue(t *testing.T) {
	if got := emitFlagValue([]string{"--emit=claude"}); got != "claude" {
		t.Errorf("emitFlagValue = %q, se esperaba claude", got)
	}
	if got := emitFlagValue([]string{"otro-arg"}); got != "" {
		t.Errorf("sin --emit, se esperaba cadena vacía, got %q", got)
	}
}

func TestBudgetFlagValue(t *testing.T) {
	if got := budgetFlagValue([]string{"--budget=500"}); got != 500 {
		t.Errorf("budgetFlagValue = %d, se esperaba 500", got)
	}
	if got := budgetFlagValue([]string{"--budget=-5"}); got != 0 {
		t.Errorf("un budget negativo debe ignorarse, got %d", got)
	}
	if got := budgetFlagValue([]string{"--budget=nan"}); got != 0 {
		t.Errorf("un budget no numérico debe ignorarse, got %d", got)
	}
	if got := budgetFlagValue(nil); got != 0 {
		t.Errorf("sin --budget, se esperaba 0, got %d", got)
	}
}

func TestRenderEnteredDocument(t *testing.T) {
	out := renderEnteredDocument(dialectClaude, "")
	if out.stdout != "{}" {
		t.Errorf("claude con doc vacío debe ser {}, got %q", out.stdout)
	}

	out = renderEnteredDocument(dialectClaude, "hola")
	if out.stdout == "" || out.stdout == "{}" {
		t.Errorf("claude con doc no vacío debe llevar additionalContext, got %q", out.stdout)
	}

	out = renderEnteredDocument(dialectJSON, "hola")
	if out.stdout == "" {
		t.Error("json debe producir salida no vacía")
	}

	out = renderEnteredDocument(dialectNeutral, "hola")
	if out.stdout != "hola" {
		t.Errorf("neutral debe pasar el documento tal cual a stdout, got %q", out.stdout)
	}
}

// TestCmdDoctor_CaminoNoStrictNoAborta ejercita CmdDoctor en proceso (sin
// --strict, así que no llama a os.Exit) contra un proyecto de prueba. Reusa
// fakeProjectRepo de cmd_save_test.go (mismo paquete).
func TestCmdDoctor_CaminoNoStrictNoAborta(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // aísla del $HOME real de la máquina
	deps := &Deps{
		Root:        t.TempDir(),
		ProjectRepo: &fakeProjectRepo{root: t.TempDir()},
	}
	CmdDoctor(deps, []string{"--json"})
	CmdDoctor(deps, nil) // camino humano (sin --json) también, sin abortar
}
