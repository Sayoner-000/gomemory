package ports

import "mem/domain"

type RelationRepository interface {
	Insert(r *domain.Relation) (int64, error)
	Update(id int64, relation domain.RelationType, confidence float64, reasoning string) error
	Get(project string, id int64) (*domain.Relation, error)
	GetByPair(project string, memIDA, memIDB int64) (*domain.Relation, error)
	List(project string, limit int) ([]domain.Relation, error)
}
