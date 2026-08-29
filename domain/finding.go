package domain

import "strings"

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
	SeverityInfo     Severity = "INFO"
)

func (s Severity) Valid() bool {
	switch s {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow, SeverityInfo:
		return true
	default:
		return false
	}
}

func (s Severity) Severe() bool {
	return s == SeverityCritical || s == SeverityHigh
}

type EvidenceClass string

const (
	EvidenceDeterministic      EvidenceClass = "deterministic"
	EvidenceReproduced         EvidenceClass = "reproduced"
	EvidenceContract           EvidenceClass = "contract"
	EvidenceStaticAnalysis     EvidenceClass = "static-analysis"
	EvidenceTestFailure        EvidenceClass = "test-failure"
	EvidenceRuntimeObservation EvidenceClass = "runtime-observation"
	EvidenceProbabilistic      EvidenceClass = "probabilistic"
)

func (c EvidenceClass) Valid() bool {
	switch c {
	case EvidenceDeterministic, EvidenceReproduced, EvidenceContract, EvidenceStaticAnalysis,
		EvidenceTestFailure, EvidenceRuntimeObservation, EvidenceProbabilistic:
		return true
	default:
		return false
	}
}

type Finding struct {
	ID               int64
	ReviewerResultID int64
	LocalID          string
	Location         string
	Severity         Severity
	Category         string
	Claim            string
	EvidenceClass    EvidenceClass
	Evidence         []string
	Confidence       string
}

func (f Finding) Confirmable() bool {
	if !f.Severity.Valid() || !f.EvidenceClass.Valid() || strings.TrimSpace(f.Claim) == "" {
		return false
	}
	for _, evidence := range f.Evidence {
		if strings.TrimSpace(evidence) != "" {
			return true
		}
	}
	return false
}
