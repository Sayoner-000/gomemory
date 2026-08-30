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
	if err := review.EnsureMutable(); err != nil {
		return nil, err
	}
	// Una revisión de solo lectura no puede mutar el target bajo ningún concepto:
	// su alcance es validar, y el ledger no debe ofrecer una vía para saltárselo
	// (FR-018).
	if !review.FixAuthorized {
		return nil, fmt.Errorf("esta revisión es de solo lectura y no admite correcciones")
	}
	if len(input.AddressedConsensusIDs) == 0 {
		return nil, fmt.Errorf("una corrección debe referenciar al menos un hallazgo confirmado")
	}
	if err := validarDigestsDeCorreccion(input); err != nil {
		return nil, err
	}
	// La cadena de targets no admite saltos: la ronda 1 parte del original y la
	// ronda N del corregido por la N-1. Sin esta comprobación, una corrección podía
	// declarar como base una revisión del código que ya nadie estaba inspeccionando
	// (FR-009).
	if vigente := review.ActiveTargetDigest(); strings.TrimSpace(input.BaseTargetDigest) != vigente {
		return nil, fmt.Errorf(
			"la corrección parte de %s pero el target vigente es %s",
			input.BaseTargetDigest, vigente,
		)
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

	// La transición completa en UNA transacción. Antes eran cuatro operaciones
	// sueltas —contar rondas, derivar el número, insertar el delta, actualizar la
	// revisión— y dos procesos concurrentes leían el mismo recuento, derivaban la
	// misma ronda y el segundo sobrescribía la corrección del primero por el
	// UPSERT, sin error y sin rastro (FR-010).
	siguiente := *review
	if err := siguiente.TransitionTo(domain.ReviewRejudging); err != nil {
		return nil, err
	}
	if err := ledger.RecordFixAtomically(input.Project, input.ReviewID, ports.FixTransition{
		Delta:               delta,
		ExpectedRounds:      len(existentes),
		ExpectedBaseDigest:  review.ActiveTargetDigest(),
		NextRound:           round,
		NextStatus:          siguiente.Status,
		CurrentTargetDigest: input.FixedTargetDigest,
	}); err != nil {
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
