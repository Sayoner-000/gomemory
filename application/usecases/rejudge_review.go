package usecases

import (
	"fmt"
	"sort"

	"mem/application/ports"
	"mem/domain"
)

// ReJudgeEntry es el veredicto de UN revisor sobre UN hallazgo corregido.
type ReJudgeEntry struct {
	State    domain.ReJudgmentState
	Evidence []string
}

type RejudgeReviewInput struct {
	Project  string
	ReviewID string
	// Reviewer es quién emite estos juicios. Es obligatorio: sin él no se puede
	// exigir corroboración independiente, que es la única propiedad que hace útil
	// al protocolo (FR-012, FR-013).
	Reviewer domain.Reviewer
	// Judgments mapea cada identificador local de consenso a su revalidación. Solo
	// se admiten hallazgos CONFIRMED incluidos en la corrección vigente.
	Judgments map[string]ReJudgeEntry
}

// RejudgeReview registra el resultado de la revalidación de un revisor sobre los
// hallazgos confirmados que la corrección vigente aborda.
//
// Exige que exista una corrección previa (INV-008): la re-revisión evalúa un fix
// delta concreto, no el aire. Sin esa comprobación se podría marcar RESOLVED sin que
// nadie hubiera arreglado nada — y como el veredicto se deriva justo de estos
// estados, eso bastaría para aprobar una revisión con su defecto intacto.
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
	if err := review.EnsureMutable(); err != nil {
		return nil, err
	}
	if !input.Reviewer.Valid() {
		return nil, fmt.Errorf("la re-revisión debe declarar qué revisor la emite (A o B)")
	}
	if len(input.Judgments) == 0 {
		return nil, fmt.Errorf("la re-revisión debe declarar el estado de al menos un hallazgo confirmado")
	}

	fixes, err := ledger.ListFixDeltas(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	if len(fixes) == 0 {
		return nil, fmt.Errorf("no hay ninguna corrección registrada que revalidar")
	}
	// La corrección VIGENTE es la última: un hallazgo que ninguna ronda abordó no
	// tiene nada que revalidar, y declararlo resuelto sería afirmar un arreglo que
	// no existe (FR-013).
	vigente := fixes[len(fixes)-1]
	abordados := make(map[string]bool, len(vigente.AddressedConsensusIDs))
	for _, localID := range vigente.AddressedConsensusIDs {
		abordados[localID] = true
	}

	// Orden estable: el mapa de entrada no lo tiene, y una salida que cambia de
	// orden entre ejecuciones idénticas convierte cualquier comparación de ledger
	// en ruido.
	localIDs := make([]string, 0, len(input.Judgments))
	for localID := range input.Judgments {
		localIDs = append(localIDs, localID)
	}
	sort.Strings(localIDs)

	// Se valida TODO antes de escribir: una entrada parcialmente válida no puede
	// dejar la mitad de los hallazgos actualizados.
	pendientes := make([]domain.ReJudgment, 0, len(localIDs))
	for _, localID := range localIDs {
		entrada := input.Judgments[localID]
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
		if !abordados[localID] {
			return nil, fmt.Errorf(
				"el hallazgo %s no forma parte de la corrección de la ronda %d",
				localID, vigente.Round,
			)
		}
		judgment := domain.ReJudgment{
			ReviewID: review.ID, Round: vigente.Round, ConsensusLocalID: localID,
			Reviewer: input.Reviewer, State: entrada.State, Evidence: entrada.Evidence,
		}
		if err := judgment.Validate(); err != nil {
			return nil, err
		}
		pendientes = append(pendientes, judgment)
	}

	out := make([]domain.ConsensusFinding, 0, len(pendientes))
	for i := range pendientes {
		if err := ledger.UpsertReJudgment(input.Project, input.ReviewID, &pendientes[i]); err != nil {
			return nil, err
		}
		// Se relee en vez de calcular aquí: el estado agregado lo decide el ledger
		// en la misma transacción que el re-juicio, y devolver una copia calculada
		// aparte abriría la puerta a que ambos discrepen.
		actualizado, err := ledger.GetConsensusFinding(input.Project, input.ReviewID, pendientes[i].ConsensusLocalID)
		if err != nil {
			return nil, err
		}
		if actualizado != nil {
			out = append(out, *actualizado)
		}
	}
	return out, nil
}

// ReJudgmentsByReviewer devuelve el estado declarado por cada revisor sobre un
// hallazgo, para que la auditoría pueda mostrar quién dijo qué (FR-023).
func ReJudgmentsByReviewer(
	ledger ports.ConsensusRepository, project, reviewID, consensusLocalID string,
) (map[domain.Reviewer]domain.ReJudgmentState, error) {
	judgments, err := ledger.ListReJudgments(project, reviewID, consensusLocalID)
	if err != nil {
		return nil, err
	}
	out := make(map[domain.Reviewer]domain.ReJudgmentState, len(judgments))
	for _, judgment := range judgments {
		out[judgment.Reviewer] = judgment.State
	}
	return out, nil
}
