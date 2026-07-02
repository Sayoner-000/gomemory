package ports

import "mem/domain"

type MemoryRepository interface {
	Insert(m *domain.Memory) (int64, error)
	List(project string, limit int) ([]domain.Memory, error)
	Search(project, query string, limit int) ([]domain.Memory, error)
	Delete(project string, id int64) (bool, error)
}
