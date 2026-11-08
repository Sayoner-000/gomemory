package cli

import (
	"strings"
	"testing"
)

func TestI18nHints(t *testing.T) {
	// 1. Probar hints en español
	nudgeES := GetSaveNudge("es")
	if !strings.Contains(nudgeES, "RECORDATORIO DE MEMORIA") {
		t.Errorf("se esperaba recordatorio en español, se obtuvo: %s", nudgeES)
	}

	planES := GetPlanModeReminder("es")
	if !strings.Contains(planES, "modo plan") {
		t.Errorf("se esperaba recordatorio de plan en español, se obtuvo: %s", planES)
	}

	// 2. Probar hints en inglés
	nudgeEN := GetSaveNudge("en")
	if !strings.Contains(nudgeEN, "MEMORY REMINDER") {
		t.Errorf("se esperaba recordatorio en inglés, se obtuvo: %s", nudgeEN)
	}

	planEN := GetPlanModeReminder("en")
	if !strings.Contains(planEN, "plan mode") {
		t.Errorf("se esperaba recordatorio de plan en inglés, se obtuvo: %s", planEN)
	}

	// 3. Fallback a español si idioma está vacío o desconocido
	nudgeFallback := GetSaveNudge("")
	if !strings.Contains(nudgeFallback, "RECORDATORIO DE MEMORIA") {
		t.Errorf("fallback debe devolver español")
	}
}
