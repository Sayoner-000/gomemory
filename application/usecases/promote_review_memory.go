package usecases

import (
	"fmt"
	"sort"

	"mem/application/ports"
	"mem/domain"
)

// ReviewMemoryWriter es la porción del repositorio de memorias que la promoción
// necesita: insertar.
//
// Se declara aquí y no se usa ports.MemoryRepository entero porque este caso de
// uso no lee, ni borra, ni busca. Pedir las doce operaciones del repositorio
// para usar una obligaría a cualquier prueba a implementarlas todas, y dejaría
// la puerta abierta a que mañana esta promoción empiece a leer o borrar sin que
// su firma lo delate.
type ReviewMemoryWriter interface {
	Insert(m *domain.Memory) (int64, error)
}

type PromoteReviewMemoryInput struct {
	Project  string
	ReviewID string
	// Learnings mapea el identificador local de consenso al conocimiento
	// destilado que el agente propone conservar. gomemory no lo redacta: valida
	// que el hallazgo tuviera derecho a producirlo.
	Learnings map[string]domain.ReviewLearning
}

// PromoteReviewMemory convierte defectos confirmados y resueltos en memoria
// reutilizable (FR-032..035).
//
// La condición es CONFIRMED + RESOLVED, y las dos mitades importan por motivos
// distintos: sin corroboración el aprendizaje podría venir de un falso positivo
// de un solo revisor; sin resolución se guardaría como conocimiento algo que
// todavía no se sabe arreglar. Cualquiera de las dos por su cuenta produce una
// memoria que se servirá en todas las sesiones futuras diciendo algo falso.
//
// La deduplicación no se implementa aquí: la clave de tópico del aprendizaje
// agrupa por patrón de fallo y el upsert por `topic_key` que ya tiene la
// persistencia (feature 008) hace que la segunda revisión del mismo patrón
// actualice la memoria de la primera.
func PromoteReviewMemory(
	reviews ports.ReviewRepository,
	ledger ports.ConsensusRepository,
	memories ReviewMemoryWriter,
	input PromoteReviewMemoryInput,
) ([]domain.Memory, error) {
	review, err := reviews.GetReview(input.Project, input.ReviewID)
	if err != nil {
		return nil, err
	}
	if review == nil {
		return nil, fmt.Errorf("review %s not found", input.ReviewID)
	}
	if len(input.Learnings) == 0 {
		return nil, nil
	}

	localIDs := make([]string, 0, len(input.Learnings))
	for localID := range input.Learnings {
		localIDs = append(localIDs, localID)
	}
	sort.Strings(localIDs)

	// Validación completa antes de escribir: una promoción a medias dejaría
	// parte del conocimiento guardado y parte no, sin forma de saber cuál.
	pendientes := make([]domain.Memory, 0, len(localIDs))
	for _, localID := range localIDs {
		finding, err := ledger.GetConsensusFinding(input.Project, input.ReviewID, localID)
		if err != nil {
			return nil, err
		}
		if finding == nil {
			return nil, fmt.Errorf("el hallazgo de consenso %s no existe en esta revisión", localID)
		}
		if finding.Status != domain.ConsensusConfirmed {
			return nil, fmt.Errorf(
				"el hallazgo %s es %s: sin corroboración independiente su aprendizaje podría ser un falso positivo",
				localID, finding.Status,
			)
		}
		if finding.RejudgmentState != domain.ReJudgmentResolved {
			return nil, fmt.Errorf(
				"el hallazgo %s no está resuelto (%s): promoverlo guardaría como conocimiento algo que aún no se sabe arreglar",
				localID, finding.RejudgmentState,
			)
		}

		learning := input.Learnings[localID]
		learning.ReviewID = review.ID
		memoria, err := learning.Memory(input.Project)
		if err != nil {
			return nil, fmt.Errorf("hallazgo %s: %w", localID, err)
		}
		memoria.SourceReviewID = review.ID
		pendientes = append(pendientes, memoria)
	}

	out := make([]domain.Memory, 0, len(pendientes))
	for i := range pendientes {
		id, err := memories.Insert(&pendientes[i])
		if err != nil {
			return nil, err
		}
		pendientes[i].ID = id
		out = append(out, pendientes[i])
	}
	return out, nil
}
