package domain

import (
	"fmt"
	"strings"
)

// ReJudgment es el resultado independiente de UN revisor sobre UN hallazgo corregido
// en UNA ronda.
//
// Es la entidad que faltaba. La 027 guardaba un único `rejudgment_state` agregado en
// consensus_findings, así que "un revisor lo da por resuelto y el otro no" era
// literalmente inexpresable: bastaba un juicio para marcar RESOLVED, y como el
// veredicto se deriva de ese estado, bastaba también para aprobar la revisión.
type ReJudgment struct {
	ID               int64
	ReviewID         string
	Round            int
	ConsensusLocalID string
	Reviewer         Reviewer
	State            ReJudgmentState
	Evidence         []string
}

func (j ReJudgment) Validate() error {
	if strings.TrimSpace(j.ConsensusLocalID) == "" {
		return fmt.Errorf("un re-juicio debe referenciar el hallazgo de consenso que revalida")
	}
	if !j.Reviewer.Valid() {
		return fmt.Errorf("un re-juicio debe declarar qué revisor lo emite (A o B)")
	}
	if !j.State.Valid() {
		return fmt.Errorf("estado de re-revisión inválido para %s: %q", j.ConsensusLocalID, j.State)
	}
	for _, evidencia := range j.Evidence {
		if strings.TrimSpace(evidencia) != "" {
			return nil
		}
	}
	return fmt.Errorf(
		"el re-juicio de %s sobre %s no aporta evidencia verificable",
		j.Reviewer, j.ConsensusLocalID,
	)
}

// AggregateReJudgment resuelve el estado de un hallazgo a partir de los re-juicios
// recibidos (FR-014).
//
// Es conservadora por construcción y el orden de precedencia no es arbitrario:
//
//  1. Un REGRESSED de cualquier revisor manda. Que uno vea una regresión y el otro
//     no significa que la corrección introdujo algo que nadie previó; darlo por
//     resuelto porque el otro revisor no lo vio sería elegir al que mira menos.
//  2. RESOLVED exige a los DOS revisores. Un solo juicio no es corroboración
//     independiente, y la corroboración independiente es la única propiedad que
//     hace útil a este protocolo.
//  3. Todo lo demás —discrepancia, un solo juicio, ninguno— queda UNRESOLVED, que
//     es el estado que NO permite aprobar.
func AggregateReJudgment(judgments []ReJudgment) ReJudgmentState {
	resueltos := map[Reviewer]bool{}
	for _, judgment := range judgments {
		if judgment.State == ReJudgmentRegressed {
			return ReJudgmentRegressed
		}
		if judgment.State == ReJudgmentResolved {
			resueltos[judgment.Reviewer] = true
		}
	}
	if resueltos[ReviewerA] && resueltos[ReviewerB] {
		return ReJudgmentResolved
	}
	return ReJudgmentUnresolved
}
