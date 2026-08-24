package cli

import (
	"strings"
	"testing"
	"time"
)

// actividadDoble es el doble de ChannelActivityLog para las pruebas del
// informe de vitalidad: devuelve lo que la prueba necesita, sin base de datos.
type actividadDoble struct {
	firedAt  time.Time // cero = canal sin ningún registro
	lastErr  string
	sessions int // incluye el -1 de "dato desconocido"
}

func (a *actividadDoble) RecordFired(agent, scope, kind string) error { return nil }

func (a *actividadDoble) RecordError(agent, scope, kind, msg string) error { return nil }

func (a *actividadDoble) Last(agent, scope, kind string) (time.Time, string, bool) {
	return a.firedAt, a.lastErr, !a.firedAt.IsZero() || a.lastErr != ""
}

func (a *actividadDoble) SessionsSince(time.Time) int { return a.sessions }

// TestPrintDoctorLiveness_SesionesDesconocidasCalla: un conteo fallido (-1) no
// alcanza para acusar a un canal sin rastro — no hay evidencia en ninguna
// dirección, y alarmar con datos inventados le cuesta credibilidad al informe.
func TestPrintDoctorLiveness_SesionesDesconocidasCalla(t *testing.T) {
	deps := &Deps{ChannelActivity: &actividadDoble{sessions: -1}}

	out := captureStdout(t, func() { printDoctorLiveness(deps) })

	if strings.Contains(out, "no responden") {
		t.Errorf("con sesiones desconocidas el informe debía callar, avisó:\n%s", out)
	}
}

// TestPrintDoctorLiveness_CanalApagadoAvisa: el único veredicto que merece
// línea es el deterioro demostrado — el canal ejerció, dejó de hacerlo y hubo
// trabajo después. El aviso nombra el efecto y el comando que lo restablece.
func TestPrintDoctorLiveness_CanalApagadoAvisa(t *testing.T) {
	deps := &Deps{ChannelActivity: &actividadDoble{
		firedAt:  time.Now().Add(-30 * 24 * time.Hour),
		sessions: 2,
	}}

	out := captureStdout(t, func() { printDoctorLiveness(deps) })

	for _, esperado := range []string{
		"Canales que no responden",
		"claude · user · plan_entry",
		"pese a 2 sesión(es)",
		"mem setup-mcp --scope global --agents claude",
	} {
		if !strings.Contains(out, esperado) {
			t.Errorf("el aviso debía mencionar %q;\nsalida:\n%s", esperado, out)
		}
	}
}
