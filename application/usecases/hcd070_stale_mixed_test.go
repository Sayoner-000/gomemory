package usecases_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mem/application/usecases"
	"mem/domain"
)

// HCD-070 / ACR C-001: `verify --stale` con tareas caducadas mixtas debe
// fallar cerrado antes de ejecutar comandos, no revalidar solo el subgrupo
// con evidencia de comando y dejar las manuales caducadas sin aviso.
func TestHCD070VerifyStaleMixtoFallaCerrado(t *testing.T) {
	env, ctx := newHCDEnv(t), context.Background()
	view, _ := env.uc.Start(ctx, usecases.HCDStartInput{
		Kind: "technical", Lane: "fast", Title: "Mixto", Why: "w", Intent: "i",
	})
	editDoc(t, env, view, append(commonFill,
		"## Checklist\n", "## Checklist\n- [ ] 1. Mirar → observar\n- [ ] 2. Ejecutar → git --version\n",
	)...)

	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"1"}, Manual: "lo vi"}); err != nil {
		t.Fatalf("verde manual: %v", err)
	}
	if _, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Tasks: []string{"2"}, Command: "git --version"}); err != nil {
		t.Fatalf("verde comando: %v", err)
	}

	// Cambiar el código caduca las dos evidencias anteriores.
	writeIn(t, env.root, "save.go", "package save // cambio\n")
	view, _ = env.uc.Show(ctx, view.Unit.ID)
	if stale := view.State.Stale(); len(stale) != 2 {
		t.Fatalf("stale = %v, want [1 2]", stale)
	}
	before := len(view.State.Evidence)

	_, _, err := env.uc.Verify(ctx, usecases.HCDVerifyInput{Stale: true})
	if !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "1") {
		t.Fatalf("err = %v, want ErrValidation que nombre la tarea 1", err)
	}
	view, _ = env.uc.Show(ctx, view.Unit.ID)
	if len(view.State.Evidence) != before {
		t.Fatalf("evidencia = %d, want %d: no debe ejecutarse el subgrupo con comando", len(view.State.Evidence), before)
	}
	if got := view.State.Stale(); len(got) != 2 {
		t.Fatalf("stale tras el error = %v, want [1 2]", got)
	}
}
