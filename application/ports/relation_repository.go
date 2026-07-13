package ports

import "mem/domain"

type RelationRepository interface {
	Insert(r *domain.Relation) (int64, error)
	Update(id int64, relation domain.RelationType, confidence float64, reasoning string) error
	Get(project string, id int64) (*domain.Relation, error)
	GetByPair(project string, memIDA, memIDB int64) (*domain.Relation, error)
	List(project string, limit int) ([]domain.Relation, error)
	// ListAll devuelve TODAS las relaciones del proyecto (sin tope). Usado por
	// el export; List está acotado a 50.
	ListAll(project string) ([]domain.Relation, error)
	// ImportRelation inserta una relación preservando su created_at de origen.
	// Usado por el import (los ids de memoria ya vienen remapeados al destino).
	ImportRelation(r *domain.Relation) (int64, error)
}
