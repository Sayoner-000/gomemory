package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

type ConsensusMatch struct {
	Status     domain.ConsensusStatus
	FindingIDA int64
	FindingIDB int64
	// Severity es informativa desde la funcionalidad 028: la severidad persistida
	// se deriva de las fuentes. Si viene y no coincide, la operación se rechaza.
	Severity domain.Severity
	Claim    string
}

type ConsensusUnmatched struct {
	Status    domain.ConsensusStatus
	FindingID int64
	Severity  domain.Severity
}

type BuildConsensusInput struct {
	Project   string
	ReviewID  string
	Matches   []ConsensusMatch
	Unmatched []ConsensusUnmatched
}

type BuildConsensusOutput struct {
	Findings []domain.ConsensusFinding
	// Idempotent indica que la ronda ya tenía exactamente esta clasificación y no
	// se escribió nada.
	Idempotent bool
}

// BuildConsensus registra la clasificación COMPLETA de la ronda activa.
//
// "Completa" es la diferencia con la versión anterior: antes la entrada describía una
// parte cualquiera y cada hallazgo se validaba mientras se recorría, así que omitir
// uno no producía error — simplemente no se mencionaba. Ahora la decisión la toma
// domain.ValidateCoverage sobre el conjunto entero, y este caso de uso se limita a
// materializar las fuentes, consultar el ledger y persistir (FR-001 a FR-005).
func BuildConsensus(
	reviews ports.ReviewRepository,
	ledger ports.ConsensusRepository,
	input BuildConsensusInput,
) ([]domain.ConsensusFinding, error) {
	out, err := BuildConsensusWithOutcome(reviews, ledger, input)
	return out.Findings, err
}

// BuildConsensusWithOutcome expone además si la llamada fue idempotente, que es lo
// que el contrato MCP publica como `idempotent`.
func BuildConsensusWithOutcome(
	reviews ports.ReviewRepository,
	ledger ports.ConsensusRepository,
	input BuildConsensusInput,
) (BuildConsensusOutput, error) {
	review, err := reviews.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return BuildConsensusOutput{}, err
	}
	if review == nil {
		return BuildConsensusOutput{}, fmt.Errorf("review %s not found", input.ReviewID)
	}
	if err := review.EnsureMutable(); err != nil {
		return BuildConsensusOutput{}, err
	}
	// El consenso se construye UNA vez, sobre los resultados de la ronda de
	// descubrimiento. Las rondas posteriores son de revalidación (FR-023).
	//
	// Sin esta guarda el daño era grave y silencioso: los ConsensusLocalID se
	// generan por posición (C-001, C-002…) y son únicos por REVISIÓN, no por
	// ronda, así que un consenso de la ronda 1 reasignaba C-001 y SOBRESCRIBÍA el
	// hallazgo confirmado de la ronda 0 — borrando la evidencia que FR-028 exige
	// conservar y dejando el addressed_consensus_ids de la corrección apuntando a
	// un hallazgo distinto del que decía abordar.
	if review.Round > 0 {
		return BuildConsensusOutput{}, fmt.Errorf(
			"la ronda %d es de revalidación: el consenso ya se construyó en la ronda de "+
				"descubrimiento y no se recalcula. Usa review_rejudge para revalidar lo "+
				"confirmado, o abre una revisión nueva sobre el target corregido",
			review.Round,
		)
	}

	results, err := reviews.ListReviewerResults(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return BuildConsensusOutput{}, err
	}
	sources := make([]domain.ConsensusSource, 0)
	for _, result := range results {
		for _, finding := range result.Findings {
			sources = append(sources, domain.ConsensusSource{
				FindingID: finding.ID, Reviewer: result.Reviewer, Severity: finding.Severity,
				Claim: finding.Claim, Confirmable: finding.Confirmable(),
			})
		}
	}

	clasificacion := domain.ConsensusClassification{}
	for _, match := range input.Matches {
		clasificacion.Matches = append(clasificacion.Matches, domain.ConsensusPair{
			Status: match.Status, FindingIDA: match.FindingIDA, FindingIDB: match.FindingIDB,
			Claim: match.Claim, DeclaredSeverity: match.Severity,
		})
	}
	for _, unmatched := range input.Unmatched {
		clasificacion.Unmatched = append(clasificacion.Unmatched, domain.ConsensusSingle{
			Status: unmatched.Status, FindingID: unmatched.FindingID,
			DeclaredSeverity: unmatched.Severity,
		})
	}

	derivados, err := domain.ValidateCoverage(sources, clasificacion)
	if err != nil {
		return BuildConsensusOutput{}, err
	}
	huella := domain.ClassificationFingerprint(derivados)

	existentes, err := ledger.ListConsensusFindings(input.Project, input.ReviewID, review.Round)
	if err != nil {
		return BuildConsensusOutput{}, err
	}
	if len(existentes) > 0 {
		// Reenviar la ronda exacta es una lectura; reemplazarla por otra distinta
		// permitiría reclasificar un confirmado como informativo justo antes de
		// finalizar, que es una aprobación falsa por otra puerta (FR-005).
		if domain.ClassificationFingerprint(existentes) != huella {
			return BuildConsensusOutput{}, fmt.Errorf(
				"la ronda %d ya tiene un consenso registrado y no admite reemplazo", review.Round,
			)
		}
		return BuildConsensusOutput{Findings: existentes, Idempotent: true}, nil
	}

	for i := range derivados {
		derivados[i].ReviewID = input.ReviewID
		derivados[i].Round = review.Round
		derivados[i].RoundFingerprint = huella
		if err := ledger.UpsertConsensusFinding(input.Project, input.ReviewID, &derivados[i]); err != nil {
			return BuildConsensusOutput{}, err
		}
	}
	return BuildConsensusOutput{Findings: derivados}, nil
}
