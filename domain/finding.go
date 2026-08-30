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

// severityRank ordena las severidades para poder derivar "la más alta aplicable"
// (FR-003). Una severidad desconocida se ordena por debajo de INFO en vez de
// provocar un error: la validación de la severidad ocurre en el borde del sistema,
// y aquí lo prudente es no dejar que un valor basura gane la comparación.
var severityRank = map[Severity]int{
	SeverityInfo:     1,
	SeverityLow:      2,
	SeverityMedium:   3,
	SeverityHigh:     4,
	SeverityCritical: 5,
}

func (s Severity) Rank() int {
	return severityRank[s]
}

// MaxSeverity devuelve la severidad más alta del conjunto.
//
// Sin fuentes devuelve INFO, la mínima: una clasificación sin respaldo no puede
// heredar gravedad de la nada. La alternativa —devolver la cadena vacía— produciría
// una severidad inválida que Severe() daría por no severa, que es exactamente el
// tipo de silencio que esta funcionalidad existe para cerrar.
func MaxSeverity(severities ...Severity) Severity {
	max := SeverityInfo
	for _, severity := range severities {
		if severity.Rank() > max.Rank() {
			max = severity
		}
	}
	return max
}
