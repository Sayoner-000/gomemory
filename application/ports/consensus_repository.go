package ports

import "mem/domain"

// FixTransition es la escritura indivisible de una ronda de corrección.
//
// Existe como un solo objeto porque sus cuatro efectos —insertar el delta, avanzar
// la ronda, mover el estado y sustituir el target vigente— tienen que ocurrir juntos
// o no ocurrir. Cuando eran cuatro llamadas sueltas, dos procesos concurrentes leían
// el mismo número de rondas registradas, derivaban la misma ronda y el segundo
// sobrescribía la corrección del primero sin que nadie se enterara.
type FixTransition struct {
	Delta *domain.FixDelta
	// ExpectedRounds son las rondas que el caso de uso vio al derivar el número de
	// ronda. Si al abrir la transacción hay otras, alguien se adelantó.
	ExpectedRounds int
	// ExpectedBaseDigest es el target vigente que el caso de uso leyó ANTES de la
	// transacción. Se vuelve a comprobar dentro, y no es redundante: el caso de uso
	// lee la revisión fuera, así que dos correcciones concurrentes pueden ver el
	// mismo target vigente, derivar rondas distintas y colarse las dos. Sin esta
	// comprobación, un test de 100 escrituras simultáneas registra dos.
	ExpectedBaseDigest  string
	NextRound           int
	NextStatus          domain.ReviewStatus
	CurrentTargetDigest string
}

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
	// RecordFixAtomically aplica la transición completa de una ronda de corrección
	// en una única transacción. Devuelve error si otra corrección ganó la carrera.
	RecordFixAtomically(project, reviewID string, transition FixTransition) error

	// UpsertReJudgment registra el re-juicio de UN revisor sobre UN hallazgo y
	// recalcula el estado agregado del hallazgo en la misma transacción, para que
	// la columna derivada nunca contradiga a la tabla de la que se deriva.
	UpsertReJudgment(project, reviewID string, judgment *domain.ReJudgment) error
	ListReJudgments(project, reviewID, consensusLocalID string) ([]domain.ReJudgment, error)
}
