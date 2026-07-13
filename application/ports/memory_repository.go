package ports

import "mem/domain"

type MemoryRepository interface {
	Insert(m *domain.Memory) (int64, error)
	List(project string, limit int) ([]domain.Memory, error)
	// ListAll devuelve TODAS las memorias del proyecto (sin tope), en orden
	// estable por id. Usado por el export; List está acotado a 200.
	ListAll(project string) ([]domain.Memory, error)
	// ImportMemory inserta una memoria preservando sus timestamps de origen y
	// SIN formar la sinapsis automática (las relaciones se importan aparte).
	// Redacta <private> igual que Insert (idempotente). Usado por el import.
	ImportMemory(m *domain.Memory) (int64, error)
	Search(project, query string, limit int) ([]domain.Memory, error)
	Delete(project string, id int64) (bool, error)
	// SecondsSinceLastSave indica cuántos segundos pasaron desde la última
	// memoria real del proyecto (excluye checkpoints automáticos). El bool es
	// false si aún no hay ninguna. Lo usa el recordatorio de guardado por turno.
	SecondsSinceLastSave(project string) (int64, bool, error)
}
