package ports

import "mem/domain"

type MemoryLister interface {
	List(project string, limit int) ([]domain.Memory, error)
}

type SessionQuerier interface {
	Active(project string) (*domain.Session, error)
	Recent(project string, limit int) ([]domain.Session, error)
}

type ContextBuilder interface {
	Build() (string, error)
	WriteFile() error
}
