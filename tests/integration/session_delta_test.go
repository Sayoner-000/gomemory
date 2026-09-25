package main

import (
	"fmt"
	"strings"
	"testing"
)

// T049 — SC-003, SC-004 y FR-019 contra el binario real.
func TestSessionDelta20Turns(t *testing.T) {
	bin := buildMemBinary(t)

	simular := func(nivel string) (first, rest int, dir string, env []string) {
		env = storeAislado(t)
		dir = proyectoCompresion(t, `{"context_compression_level": "`+nivel+`"}`)
		for i := 1; i <= 25; i++ {
			correrMem(t, bin, dir, env, "", "save", "-t", fmt.Sprintf("Decisión número %d sobre el módulo %d", i, i), "-y", "decision",
				fmt.Sprintf("Contenido de la decisión %d: se eligió la opción %d por rendimiento y mantenimiento, con pruebas en el paquete p%d.", i, i, i))
		}
		correrMem(t, bin, dir, env, "", "session", "start")
		for turn := 1; turn <= 20; turn++ {
			if turn == 8 || turn == 15 {
				correrMem(t, bin, dir, env, "", "save", "-t", fmt.Sprintf("Nueva en turno %d", turn), "-y", "learning", fmt.Sprintf("aprendizaje del turno %d", turn))
			}
			n := len(correrMem(t, bin, dir, env, "", "context").stdout) + len(correrMem(t, bin, dir, env, "", "plan-context").stdout)
			if turn == 1 {
				first = n
			} else {
				rest += n
			}
		}
		return first, rest, dir, env
	}

	_, restStructural, _, _ := simular("structural")
	_, restMax, dir, env := simular("max")
	reduccion := 100 * (1 - float64(restMax)/float64(restStructural))
	t.Logf("turnos 2-20: structural=%d max=%d → -%.1f%%", restStructural, restMax, reduccion)
	if reduccion < 80 {
		t.Errorf("SC-003: el contenido reenviado solo baja un %.1f%% (< 80%%)", reduccion)
	}

	// Las memorias nuevas sí viajan completas en su turno.
	correrMem(t, bin, dir, env, "", "save", "-t", "Memoria recién guardada", "-y", "decision", "contenido nuevo que el agente no tiene")
	if out := correrMem(t, bin, dir, env, "", "context").stdout; !strings.Contains(out, "contenido nuevo que el agente no tiene") {
		t.Error("una memoria nueva debe entregarse completa")
	}

	// FR-019: tras compactar, todo vuelve a entregarse completo.
	correrMem(t, bin, dir, env, "{}", "hook", "post-compact")
	if out := correrMem(t, bin, dir, env, "", "context").stdout; strings.Contains(out, "ya entregad") {
		t.Error("tras post-compact no debe quedar nada marcado como ya entregado")
	}
	if plan := correrMem(t, bin, dir, env, "", "plan-context").stdout; !strings.Contains(plan, "Descomposición Atómica") {
		t.Error("tras compactar, el método de planificación debe volver completo")
	}

	// SC-004: dos arranques con la misma memoria y distinta actividad dan el
	// mismo prefijo estable.
	prefijo := func() string {
		out := correrMem(t, bin, dir, env, "{}", "hook", "session-start").stdout
		i := strings.Index(out, "<!-- mem:volatile -->")
		if i < 0 {
			t.Fatalf("falta el separador volátil en el arranque:\n%.400s", out)
		}
		return out[:i]
	}
	p1 := prefijo()
	correrMem(t, bin, dir, env, "", "session", "start")
	p2 := prefijo()
	if p1 != p2 {
		t.Error("el prefijo estable del contexto de arranque cambió sin cambiar las memorias")
	}
}
