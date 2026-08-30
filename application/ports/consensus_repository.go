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
	ExpectedBaseDigest string
	// ExpectedStatus es el estado que el caso de uso comprobó mutable antes de la
	// transacción. Se revalida dentro, y cierra la dirección que el digest no cubre:
	// finalizar NO cambia el target, así que una corrección que llegaba tarde pasaba
	// la comprobación de digest y reabría una revisión ya terminal.
	ExpectedStatus      domain.ReviewStatus
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

	// ReplaceConsensusRound persiste la clasificación COMPLETA de una ronda en una
	// sola transacción, o no persiste nada.
	//
	// Reemplaza al par «listar y luego insertar fila a fila», que era un
	// check-then-write sin transacción: dos llamadas concurrentes veían el ledger
	// vacío, las dos escribían y la ronda quedaba mezclada; y un fallo a mitad del
	// bucle dejaba media clasificación persistida, que es justo lo que el veredicto
	// tiene que atrapar después.
	//
	// Devuelve las filas ya existentes con idempotente=true cuando la ronda
	// registrada tiene exactamente la misma huella. Si la huella difiere, rechaza:
	// reclasificar un confirmado como informativo justo antes de finalizar es una
	// aprobación falsa por otra puerta (FR-005).
	ReplaceConsensusRound(
		project, reviewID string, expectedRound int, fingerprint string,
		findings []domain.ConsensusFinding,
	) (existentes []domain.ConsensusFinding, idempotente bool, err error)

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
