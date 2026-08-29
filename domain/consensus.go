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
}
