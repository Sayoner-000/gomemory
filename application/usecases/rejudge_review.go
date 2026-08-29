package usecases

import (
	"fmt"
	"sort"

	"mem/application/ports"
	"mem/domain"
)

type RejudgeReviewInput struct {
	Project  string
	ReviewID string
	// States mapea cada identificador local de consenso al resultado de su
	// revalidación. Solo se admiten hallazgos CONFIRMED: son los únicos que
	// pasaron por corrección y, por tanto, los únicos con algo que resolver.
	States map[string]domain.ReJudgmentState
}

// RejudgeReview registra el resultado de la ronda de revalidación sobre los
// hallazgos confirmados.
//
// Exige que exista una corrección previa (INV-008): la re-revisión evalúa un
// fix delta concreto, no el aire. Sin esa comprobación se podría marcar
// RESOLVED sin que nadie hubiera arreglado nada — y como el veredicto se deriva
// justo de estos estados, eso bastaría para aprobar una revisión con su defecto
// intacto.
//
// Deliberadamente NO recalcula el consenso ni admite hallazgos nuevos: la
// revalidación está acotada a verificar lo ya confirmado (FR-023). Un defecto
// descubierto ahora pertenece a una revisión nueva sobre el target corregido.
func RejudgeReview(
	reviews ports.ReviewRepository,
	ledger ports.ConsensusRepository,
	input RejudgeReviewInput,
) ([]domain.ConsensusFinding, error) {
	review, err := reviews.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	if review == nil {
		return nil, fmt.Errorf("review %s not found", input.ReviewID)
	}
	if len(input.States) == 0 {
		return nil, fmt.Errorf("la re-revisión debe declarar el estado de al menos un hallazgo confirmado")
	}

	fixes, err := ledger.ListFixDeltas(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	if len(fixes) == 0 {
		return nil, fmt.Errorf("no hay ninguna corrección registrada que revalidar")
	}

	// Orden estable: el mapa de entrada no lo tiene, y una salida que cambia de
	// orden entre ejecuciones idénticas convierte cualquier comparación de
	// ledger en ruido.
	localIDs := make([]string, 0, len(input.States))
	for localID := range input.States {
		localIDs = append(localIDs, localID)
	}
	sort.Strings(localIDs)

	// Se valida todo antes de escribir: una entrada parcialmente válida no
	// puede dejar la mitad de los hallazgos actualizados.
	pendientes := make([]domain.ConsensusFinding, 0, len(localIDs))
	for _, localID := range localIDs {
		estado := input.States[localID]
		if !estado.Valid() {
			return nil, fmt.Errorf("estado de re-revisión inválido para %s: %q", localID, estado)
		}
		finding, err := ledger.GetConsensusFinding(input.Project, input.ReviewID, localID)
		if err != nil {
			return nil, err
		}
		if finding == nil {
			return nil, fmt.Errorf("el hallazgo de consenso %s no existe en esta revisión", localID)
		}
		if finding.Status != domain.ConsensusConfirmed {
			return nil, fmt.Errorf(
				"el hallazgo %s es %s: solo un CONFIRMED pasa por corrección y tiene resolución que declarar",
				localID, finding.Status,
			)
		}
		finding.RejudgmentState = estado
		pendientes = append(pendientes, *finding)
	}

	out := make([]domain.ConsensusFinding, 0, len(pendientes))
	for i := range pendientes {
		if err := ledger.UpsertConsensusFinding(input.Project, input.ReviewID, &pendientes[i]); err != nil {
			return nil, err
		}
		out = append(out, pendientes[i])
	}
	return out, nil
}
