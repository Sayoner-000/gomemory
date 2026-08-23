package domain

import (
	"fmt"
	"time"
)

// La vitalidad distingue un canal presente de uno que funciona.
//
// El informe comprobaba que el artefacto existiera. Un complemento cuyo agente
// renombró la operación que usa queda muerto con el archivo intacto, así que
// presencia y salud no son lo mismo.

type LivenessVerdict string

const (
	// LivenessUnused: nunca se ejerció y tampoco hubo trabajo. No es un fallo.
	LivenessUnused LivenessVerdict = "unused"
	// LivenessIdle: hace tiempo que no se ejerce, pero tampoco hubo sesiones.
	// Una semana sin trabajar no es un canal roto.
	LivenessIdle LivenessVerdict = "idle"
	// LivenessOK: se ejerció dentro del umbral.
	LivenessOK LivenessVerdict = "ok"
	// LivenessStale: hubo trabajo y el canal no respondió. Es el único que
	// merece reportarse.
	LivenessStale LivenessVerdict = "stale"
)

// DefaultLivenessThreshold es el tiempo tras el cual un canal sin uso pasa a
// examinarse. Siete días cubren una semana de trabajo normal sin producir
// falsas alarmas por un fin de semana largo.
const DefaultLivenessThreshold = 7 * 24 * time.Hour

// EvaluateLiveness decide si un canal merece reportarse como inactivo.
//
// La distinción que importa (FR-011) es entre "no se ejerció porque nadie
// trabajó" y "no se ejerció habiendo trabajo". Sin ella, unas vacaciones se
// reportarían igual que un complemento roto, y el informe perdería credibilidad
// justo donde debe ganarla.
func EvaluateLiveness(fired time.Time, sessionsSince int, threshold time.Duration, now time.Time) (LivenessVerdict, string) {
	if threshold <= 0 {
		threshold = DefaultLivenessThreshold
	}

	// Un canal que nunca se ejerció NO se acusa, aunque haya habido sesiones.
	// Las sesiones no se atribuyen a un agente concreto, así que trabajar con
	// un agente haría parecer muerto el canal de otro que simplemente no se
	// usa. Solo se reporta el canal que demostró funcionar y dejó de hacerlo:
	// ahí sí hay evidencia de deterioro, no de desuso.
	if fired.IsZero() {
		return LivenessUnused, "nunca se ha ejercido; no hay evidencia de que este agente se use en este proyecto"
	}

	inactivo := now.Sub(fired)
	if inactivo <= threshold {
		return LivenessOK, ""
	}
	if sessionsSince == 0 {
		return LivenessIdle, fmt.Sprintf("sin uso desde %s, pero tampoco ha habido sesiones", fired.Format("2006-01-02"))
	}
	return LivenessStale, fmt.Sprintf("sin ejercerse desde %s pese a %d sesión(es) posteriores", fired.Format("2006-01-02"), sessionsSince)
}
