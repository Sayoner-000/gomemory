package domain

type ConsensusStatus string

const (
	ConsensusConfirmed     ConsensusStatus = "CONFIRMED"
	ConsensusSuspect       ConsensusStatus = "SUSPECT"
	ConsensusContradiction ConsensusStatus = "CONTRADICTION"
	ConsensusInfo          ConsensusStatus = "INFO"
)

func (s ConsensusStatus) Valid() bool {
	switch s {
	case ConsensusConfirmed, ConsensusSuspect, ConsensusContradiction, ConsensusInfo:
		return true
	default:
		return false
	}
}

type ConsensusFinding struct {
	ID               int64
	ReviewID         string
	Round            int
	ConsensusLocalID string
	Status           ConsensusStatus
	Severity         Severity
	Claim            string
	SourceFindingIDs []int64
	RejudgmentState  ReJudgmentState
	// RejudgmentRound es la ronda en la que se calculó RejudgmentState.
	//
	// Sin ella el estado agregado es un valor sin fecha: RecordFix abría la ronda
	// siguiente sin invalidarlo, y el veredicto leía como vigente un RESOLVED que
	// pertenecía a la corrección anterior. Una revisión se aprobaba así sin un solo
	// re-juicio sobre el target que de verdad estaba en evaluación.
	//
	// Vale 0 en los hallazgos que nunca se re-juzgaron y en las filas escritas por
	// versiones anteriores a esta columna. Como los re-juicios solo existen a partir
	// de la ronda 1, ese 0 nunca coincide con la ronda vigente y esas filas fallan
	// cerrado: hay que re-juzgarlas, que es exactamente lo que se quiere.
	RejudgmentRound int
	// RoundFingerprint identifica la clasificación completa de la ronda a la que
	// pertenece este hallazgo. Es lo que permite distinguir el reenvío exacto de
	// una ronda de un intento de reemplazarla (FR-005).
	RoundFingerprint string
}
