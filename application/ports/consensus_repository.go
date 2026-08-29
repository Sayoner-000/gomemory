package ports

import "mem/domain"

type ConsensusRepository interface {
	UpsertConsensusFinding(project, reviewID string, finding *domain.ConsensusFinding) error
	GetConsensusFinding(project, reviewID, consensusLocalID string) (*domain.ConsensusFinding, error)
	ListConsensusFindings(project, reviewID string, round int) ([]domain.ConsensusFinding, error)
	// ListAllConsensusFindings devuelve los hallazgos de TODAS las rondas.
	//
	// Existe porque el veredicto no se deriva de la ronda en curso sino del
	// estado actual de todo lo confirmado: un hallazgo nace en la ronda del
	// consenso y su resolución llega en una posterior. Derivar el veredicto con
	// el listado por ronda hacía que, tras la primera corrección, no se viera
	// ningún hallazgo y la revisión se aprobara con su defecto intacto.
	ListAllConsensusFindings(project, reviewID string) ([]domain.ConsensusFinding, error)

	UpsertFixDelta(project, reviewID string, delta *domain.FixDelta) error
	ListFixDeltas(project, reviewID string) ([]domain.FixDelta, error)
}
