package domain

import (
	"testing"
	"time"
)

func TestEvaluateLiveness(t *testing.T) {
	ahora := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	hace := func(d time.Duration) time.Time { return ahora.Add(-d) }

	casos := []struct {
		nombre   string
		fired    time.Time
		sesiones int
		esperado LivenessVerdict
	}{
		{"nunca usado y sin trabajo no es fallo", time.Time{}, 0, LivenessUnused},
		{"nunca usado no se acusa: las sesiones no se atribuyen a un agente", time.Time{}, 3, LivenessUnused},
		{"usado hace poco está sano", hace(2 * time.Hour), 5, LivenessOK},
		{"sin uso pero sin sesiones es inactividad, no fallo", hace(30 * 24 * time.Hour), 0, LivenessIdle},
		{"sin uso habiendo sesiones es fallo", hace(30 * 24 * time.Hour), 4, LivenessStale},
		{"sesiones desconocidas no acusan deterioro", hace(30 * 24 * time.Hour), -1, LivenessIdle},
		{"justo en el umbral sigue sano", hace(DefaultLivenessThreshold), 2, LivenessOK},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			v, detalle := EvaluateLiveness(c.fired, c.sesiones, DefaultLivenessThreshold, ahora)
			if v != c.esperado {
				t.Fatalf("esperaba %q, obtuve %q (%s)", c.esperado, v, detalle)
			}
			if v != LivenessOK && detalle == "" {
				t.Error("un veredicto distinto de sano debe explicarse")
			}
		})
	}
}
