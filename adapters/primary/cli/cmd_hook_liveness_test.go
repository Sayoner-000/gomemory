package cli

import (
	"testing"
)

// TestRecordPlanEntryActivity_RegistraElCanalDeClaude cubre la paridad con el
// complemento de OpenCode (feature 024, FR-009): la entrada al plan de Claude
// Code debe dejar rastro claude·user·plan_entry. Sin él, un cambio silencioso
// en el agente dejaba el canal muerto y la vitalidad ni se enteraba.
func TestRecordPlanEntryActivity_RegistraElCanalDeClaude(t *testing.T) {
	casos := []struct {
		nombre    string
		conPuerto bool
	}{
		{"con puerto registra claude·plan_entry", true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			reg := &registroFalso{}
			deps := &Deps{}
			if c.conPuerto {
				deps.ChannelActivity = reg
			}

			recordPlanEntryActivity(deps)

			if !c.conPuerto {
				return
			}
			if reg.firedAgent != "claude" || reg.firedKind != "plan_entry" {
				t.Errorf("rastro equivocado: agente=%q canal=%q", reg.firedAgent, reg.firedKind)
			}
		})
	}
}

// TestRecordPlanEntryActivity_SinPuertoNoRompe: deps sin ChannelActivity (o nil)
// no puede provocar pánico — el hook corre en el turno de quien trabaja.
func TestRecordPlanEntryActivity_SinPuertoNoRompe(t *testing.T) {
	recordPlanEntryActivity(&Deps{})
	recordPlanEntryActivity(nil)
}

// TestCanalesDeInyeccion_VigilaAmbosAgentes: el informe de vitalidad no puede
// volver a quedarse ciego para un agente. Hoy la lista debe incluir a OpenCode
// y a Claude; si mañana otro agente gana un canal de inyección vivo, esta
// prueba es el recordatorio de sumarlo aquí.
func TestCanalesDeInyeccion_VigilaAmbosAgentes(t *testing.T) {
	vistos := map[string]bool{}
	for _, c := range canalesDeInyeccion {
		vistos[c.agent] = true
	}
	for _, agente := range []string{"opencode", "claude"} {
		if !vistos[agente] {
			t.Errorf("la vitalidad no vigila a %s", agente)
		}
	}
}
