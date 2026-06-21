package ports

import "mem/domain"

type SessionRepository interface {
	Start(project string) (*domain.Session, error)
	End(id, summary string) error
	Active(project string) (*domain.Session, error)
	Recent(project string, limit int) ([]domain.Session, error)
}
