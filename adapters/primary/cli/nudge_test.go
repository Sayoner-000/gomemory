package cli

import "testing"

// TestComputePlanModeReminder_ActivoPorDefecto cubre FR-003/FR-006 (feature
// 019, Historia 2): el recordatorio de modo plan se emite en cada turno,
// salvo que la planificación atómica esté apagada — sin debounce, a
// diferencia del nudge de guardado, porque el coste es una sola línea.
func TestComputePlanModeReminder_ActivoPorDefecto(t *testing.T) {
	msg, ok := computePlanModeReminder(false)
	if !ok || msg == "" {
		t.Fatal("con la planificación atómica activa, el recordatorio debe emitirse")
	}
}

func TestComputePlanModeReminder_ApagadoNoEmiteNada(t *testing.T) {
	msg, ok := computePlanModeReminder(true)
	if ok || msg != "" {
		t.Errorf("con atomic_plan_disabled=true no debe emitirse nada, got (%q, %v)", msg, ok)
	}
}

// TestComputePlanModeReminder_SinDebounce cubre que, a diferencia de
// computeSaveNudge, este recordatorio no se apaga solo por haberse emitido
// antes: dos llamadas consecutivas deben dar el mismo resultado.
func TestComputePlanModeReminder_SinDebounce(t *testing.T) {
	first, okFirst := computePlanModeReminder(false)
	second, okSecond := computePlanModeReminder(false)
	if !okFirst || !okSecond || first != second {
		t.Errorf("dos invocaciones consecutivas deben coincidir, got (%q,%v) y (%q,%v)", first, okFirst, second, okSecond)
	}
}
