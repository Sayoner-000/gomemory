package domain

import (
	"fmt"
	"time"
)

// DefaultMaxFixRounds es el presupuesto de rondas cuando el proyecto no
// configura otro. No existe la opción «sin techo»: un presupuesto ausente que
// se interpretara como infinito sería el bucle silencioso que INV-009 prohíbe.
const DefaultMaxFixRounds = 2

// AuthorizeFix decide si un hallazgo de consenso puede corregirse. Es una
// función pura: la elegibilidad no depende del estado del disco ni de lo que
// afirme quien llama.
//
// Dos reglas de naturaleza distinta, y conviene no confundirlas:
//
//   - La CORROBORACIÓN (solo CONFIRMED) es una invariante del protocolo
//     (INV-005/INV-006). `explicitAuthorization` no la levanta: un hallazgo de
//     un solo revisor no se vuelve fiable porque alguien lo autorice.
//   - La SEVERIDAD admitida es POLÍTICA del proyecto, configurable, y sí puede
//     ampliarse caso a caso con autorización explícita.
func AuthorizeFix(review Review, finding ConsensusFinding, explicitAuthorization bool) error {
	if finding.Status != ConsensusConfirmed {
		return fmt.Errorf(
			"el hallazgo %s es %s: solo un CONFIRMED tiene corroboración independiente y puede corregirse",
			finding.ConsensusLocalID, finding.Status,
		)
	}
	if explicitAuthorization {
		return nil
	}
	for _, admitida := range review.autoFixSeverities() {
		if finding.Severity == admitida {
			return nil
		}
	}
	return fmt.Errorf(
		"la severidad %s del hallazgo %s está fuera de la política de corrección automática del proyecto",
		finding.Severity, finding.ConsensusLocalID,
	)
}

// NextFixRound devuelve el número de la siguiente ronda de corrección dado
// cuántas se han registrado ya, o error si excedería el presupuesto (INV-009).
//
// Recibe el conteo en vez de leerlo del propio Review para que el llamador no
// pueda pasar una ronda arbitraria: aquí el número se DERIVA, no se acepta.
func (r Review) NextFixRound(rondasRegistradas int) (int, error) {
	siguiente := rondasRegistradas + 1
	if siguiente > r.maxFixRounds() {
		return 0, fmt.Errorf(
			"presupuesto de rondas agotado (%d de %d): la revisión debe escalar, no seguir corrigiendo",
			rondasRegistradas, r.maxFixRounds(),
		)
	}
	return siguiente, nil
}

func (r Review) maxFixRounds() int {
	if r.MaxFixRounds <= 0 {
		return DefaultMaxFixRounds
	}
	return r.MaxFixRounds
}

func (r Review) autoFixSeverities() []Severity {
	if len(r.AutoFixSeverities) == 0 {
		return []Severity{SeverityCritical, SeverityHigh}
	}
	return r.AutoFixSeverities
}

type Reviewer string

const (
	ReviewerA Reviewer = "A"
	ReviewerB Reviewer = "B"
)

func (r Reviewer) Valid() bool {
	return r == ReviewerA || r == ReviewerB
}

type ReviewerResultStatus string

const (
	ReviewerResultSuccess ReviewerResultStatus = "success"
	ReviewerResultFailure ReviewerResultStatus = "failure"
)

type ReviewerResult struct {
	ID          int64
	ReviewID    string
	Reviewer    Reviewer
	Round       int
	Provider    string
	Model       string
	Status      ReviewerResultStatus
	Findings    []Finding
	SubmittedAt time.Time
}

type FixDelta struct {
	ID                    int64
	ReviewID              string
	Round                 int
	BaseTargetDigest      string
	FixedTargetDigest     string
	AddressedConsensusIDs []string
	ModifiedPaths         []string
	Verification          []string
	DiffDigest            string
	CreatedAt             time.Time
}
