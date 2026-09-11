package main

import (
	"regexp"
	"strings"
	"testing"

	"mem/domain"
)

// terminosProhibidos son los nombres de agentes/clientes y la forma de un
// comando de cliente que ningún texto de la feature 030 puede nombrar
// (spec.md FR-018, SC-007).
var terminosProhibidos = []string{"claude", "codex", "opencode", "cursor"}

var comandoClienteRe = regexp.MustCompile(`/\w`)

// TestCompactionTexts_Agnosticos100Porciento recorre domain.CompactionTexts()
// y falla si algún texto nombra un agente, un cliente o un comando de
// cliente. Es el equivalente automatizado del grep manual que se corrió a
// mano sobre la spec.
func TestCompactionTexts_Agnosticos100Porciento(t *testing.T) {
	for _, texto := range domain.CompactionTexts() {
		lower := strings.ToLower(texto)
		for _, prohibido := range terminosProhibidos {
			if strings.Contains(lower, prohibido) {
				t.Errorf("texto contiene el término prohibido %q: %q", prohibido, texto)
			}
		}
		if comandoClienteRe.MatchString(texto) {
			t.Errorf("texto parece nombrar un comando de cliente (patrón /palabra): %q", texto)
		}
	}
}

// TestRecoverySteps_PideSaveSessionSummaryNoEndSession cubre el cierre del
// defecto latente (research.md R4): la recuperación tras compactar dejaba de
// tener sesión activa porque pedía end_session, que la cierra. El paso 1
// ahora pide save_session_summary, que actualiza el resumen SIN cerrar la
// sesión.
func TestRecoverySteps_PideSaveSessionSummaryNoEndSession(t *testing.T) {
	if !strings.Contains(domain.RecoverySteps, "save_session_summary") {
		t.Errorf("RecoverySteps debe pedir save_session_summary como primer paso: %q", domain.RecoverySteps)
	}
	if strings.Contains(domain.RecoverySteps, "end_session") {
		t.Errorf("RecoverySteps NO debe pedir end_session: cerraría la sesión a mitad de la conversación: %q",
			domain.RecoverySteps)
	}
}
