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

// MemoryFullLister evita ensanchar MemoryRepository para lecturas sin tope.
type MemoryFullLister interface {
	ListAll(project string) ([]domain.Memory, error)
}

// RelationFullLister evita ensanchar RelationLister para lecturas sin recorte.
type RelationFullLister interface {
	ListAll(project string) ([]domain.Relation, error)
}

// IndexedFilesQuerier aísla la consulta de archivos indexados del grafo completo.
type IndexedFilesQuerier interface {
	FileHashes(project string) (map[string]string, error)
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
