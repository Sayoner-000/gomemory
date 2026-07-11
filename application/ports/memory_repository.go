package ports

import "mem/domain"

type MemoryRepository interface {
	Insert(m *domain.Memory) (int64, error)
	List(project string, limit int) ([]domain.Memory, error)
	Search(project, query string, limit int) ([]domain.Memory, error)
	Delete(project string, id int64) (bool, error)
	// SecondsSinceLastSave indica cuántos segundos pasaron desde la última
	// memoria real del proyecto (excluye checkpoints automáticos). El bool es
	// false si aún no hay ninguna. Lo usa el recordatorio de guardado por turno.
	SecondsSinceLastSave(project string) (int64, bool, error)
}
