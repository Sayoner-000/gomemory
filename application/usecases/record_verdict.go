package usecases

import (
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// RecordVerdict persiste el veredicto sobre la relación entre dos memorias:
// actualiza la relación existente del par si ya había una (compare/judge son
// siempre el estado más reciente, no un historial), o crea una nueva.
// Lo comparten `mem compare` (CLI, veredicto humano) y la tool MCP
// `judge_memories` (veredicto del agente) para no duplicar la lógica.
func RecordVerdict(relRepo ports.RelationRepository, project string, idA, idB int64, relation domain.RelationType, confidence float64, reasoning string) (*domain.Relation, bool, error) {
	if idA == idB {
		return nil, false, fmt.Errorf("no se puede comparar una memoria consigo misma")
	}

	existing, _ := relRepo.GetByPair(project, idA, idB)
	if existing != nil {
		if err := relRepo.Update(existing.ID, relation, confidence, reasoning); err != nil {
			return nil, false, fmt.Errorf("actualizar relación: %w", err)
		}
		existing.Relation = relation
		existing.Confidence = confidence
		existing.Reasoning = reasoning
		return existing, true, nil
	}

	rel := &domain.Relation{
		Project:    project,
		MemoryIDA:  idA,
		MemoryIDB:  idB,
		Relation:   relation,
		Confidence: confidence,
		Reasoning:  reasoning,
	}
	id, err := relRepo.Insert(rel)
	if err != nil {
		return nil, false, fmt.Errorf("guardar relación: %w", err)
	}
	rel.ID = id
	return rel, false, nil
}
