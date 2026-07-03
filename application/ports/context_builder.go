package ports

import "mem/domain"

type MemoryLister interface {
	List(project string, limit int) ([]domain.Memory, error)
}

type SessionQuerier interface {
	Active(project string) (*domain.Session, error)
	Recent(project string, limit int) ([]domain.Session, error)
}

type RelationLister interface {
	List(project string, limit int) ([]domain.Relation, error)
}

// GraphStatusQuerier es la porción de CodeGraphRepository que necesita
// build_context.go para resumir el índice de código en get_context, sin
// acoplarse a la interfaz completa de escritura/consulta del grafo.
type GraphStatusQuerier interface {
	Status(project string) (domain.GraphStatus, error)
}

type ContextBuilder interface {
	Build() (string, error)
	WriteFile() error
}
