package usecases

import (
	"fmt"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

type RecordFixInput struct {
	Project  string
	ReviewID string
	// AddressedConsensusIDs son los identificadores locales de consenso que
	// esta corrección resuelve. Al menos uno, y todos confirmados (INV-006).
	AddressedConsensusIDs []string
	BaseTargetDigest      string
	FixedTargetDigest     string
	ModifiedPaths         []string
	Verification          []string
	DiffDigest            string
	// ExplicitAuthorization amplía la política de severidad para esta
	// corrección concreta. NO levanta la exigencia de corroboración.
	ExplicitAuthorization bool
}

// RecordFix registra una corrección aplicada FUERA de gomemory y hace avanzar
// la revisión a re-revisión.
//
// gomemory no corrige nada: recibe el resultado y decide si tenía derecho a
// existir. Todo lo que valida aquí es estructural y verificable —quién estaba
// confirmado, qué severidad admite la política, cuántas rondas van— y por eso
// ninguna de estas reglas depende de que el agente las respete.
//
// El número de ronda se DERIVA de las correcciones ya registradas; no hay
// parámetro de entrada para pedirlo (INV-009).
func RecordFix(
	reviews ports.ReviewRepository,
	ledger ports.ConsensusRepository,
	input RecordFixInput,
) (*domain.FixDelta, error) {
	review, err := reviews.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	if review == nil {
		return nil, fmt.Errorf("review %s not found", input.ReviewID)
	}
	if len(input.AddressedConsensusIDs) == 0 {
		return nil, fmt.Errorf("una corrección debe referenciar al menos un hallazgo confirmado")
	}
	if err := validarDigestsDeCorreccion(input); err != nil {
		return nil, err
	}

	// Autorización de TODOS los hallazgos antes de escribir nada: un rechazo a
	// mitad dejaría una ronda registrada por una corrección que no procedía.
	for _, localID := range input.AddressedConsensusIDs {
		finding, err := ledger.GetConsensusFinding(input.Project, input.ReviewID, localID)
		if err != nil {
			return nil, err
		}
		if finding == nil {
			return nil, fmt.Errorf("el hallazgo de consenso %s no existe en esta revisión", localID)
		}
		if err := domain.AuthorizeFix(*review, *finding, input.ExplicitAuthorization); err != nil {
			return nil, err
		}
	}

	existentes, err := ledger.ListFixDeltas(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	round, err := review.NextFixRound(len(existentes))
	if err != nil {
		return nil, err
	}

	delta := &domain.FixDelta{
		ReviewID:              review.ID,
		Round:                 round,
		BaseTargetDigest:      input.BaseTargetDigest,
		FixedTargetDigest:     input.FixedTargetDigest,
		AddressedConsensusIDs: append([]string(nil), input.AddressedConsensusIDs...),
		ModifiedPaths:         append([]string(nil), input.ModifiedPaths...),
		Verification:          append([]string(nil), input.Verification...),
		DiffDigest:            input.DiffDigest,
	}
	if err := ledger.UpsertFixDelta(input.Project, input.ReviewID, delta); err != nil {
		return nil, err
	}

	review.Round = round
	review.Status = domain.ReviewRejudging
	if err := reviews.UpdateReview(review); err != nil {
		return nil, err
	}
	return delta, nil
}

// validarDigestsDeCorreccion exige que la corrección produzca una revisión
// NUEVA del target (INV-007). Un digest corregido igual al base significa que
// no cambió nada: registrarlo dejaría el ledger afirmando un arreglo que no
// existe, y la re-revisión evaluaría el mismo código de antes.
func validarDigestsDeCorreccion(input RecordFixInput) error {
	base := strings.TrimSpace(input.BaseTargetDigest)
	fixed := strings.TrimSpace(input.FixedTargetDigest)
	if base == "" || fixed == "" {
		return fmt.Errorf("una corrección debe declarar el digest del target base y del corregido")
	}
	if base == fixed {
		return fmt.Errorf("el target corregido tiene el mismo digest que el base: la corrección no cambió nada")
	}
	return nil
}
