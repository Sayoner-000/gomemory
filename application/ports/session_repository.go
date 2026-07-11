package ports

import "mem/domain"

type SessionRepository interface {
	Start(project string) (*domain.Session, error)
	End(id, summary string) error
	Active(project string) (*domain.Session, error)
	Recent(project string, limit int) ([]domain.Session, error)
	// SetLastPrompt guarda el prompt del usuario del turno en curso en la sesión
	// activa, para adjuntarlo como provenance a las memorias que se guarden.
	SetLastPrompt(project, prompt string) error
}
