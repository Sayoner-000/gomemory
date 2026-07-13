package persistence

import (
	"database/sql"
	"fmt"

	"mem/domain"
)

func InsertRelation(db *sql.DB, r *domain.Relation) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, `+Now+`)`,
		r.Project, r.MemoryIDA, r.MemoryIDB, string(r.Relation), r.Confidence, r.Reasoning,
	)
	if err != nil {
		return 0, fmt.Errorf("insert relation: %w", err)
	}
	return res.LastInsertId()
}

func UpdateRelation(db *sql.DB, id int64, relation domain.RelationType, confidence float64, reasoning string) error {
	res, err := db.Exec(
		`UPDATE memory_relations SET relation = ?, confidence = ?, reasoning = ? WHERE id = ?`,
		string(relation), confidence, reasoning, id,
	)
	if err != nil {
		return fmt.Errorf("update relation: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("relation %d not found", id)
	}
	return nil
}

func GetRelation(db *sql.DB, project string, id int64) (*domain.Relation, error) {
	var r domain.Relation
	err := db.QueryRow(
		`SELECT id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at
		 FROM memory_relations WHERE id = ? AND project = ?`,
		id, project,
	).Scan(&r.ID, &r.Project, &r.MemoryIDA, &r.MemoryIDB, &r.Relation, &r.Confidence, &r.Reasoning, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relation: %w", err)
	}
	return &r, nil
}

func GetRelationByPair(db *sql.DB, project string, memIDA, memIDB int64) (*domain.Relation, error) {
	var r domain.Relation
	err := db.QueryRow(
		`SELECT id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at
		 FROM memory_relations
		 WHERE project = ? AND ((memory_id_a = ? AND memory_id_b = ?) OR (memory_id_a = ? AND memory_id_b = ?))
		 LIMIT 1`,
		project, memIDA, memIDB, memIDB, memIDA,
	).Scan(&r.ID, &r.Project, &r.MemoryIDA, &r.MemoryIDB, &r.Relation, &r.Confidence, &r.Reasoning, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get relation by pair: %w", err)
	}
	return &r, nil
}

func ListRelations(db *sql.DB, project string, limit int) ([]domain.Relation, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at
		 FROM memory_relations WHERE project = ? ORDER BY created_at DESC LIMIT ?`,
		project, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list relations: %w", err)
	}
	defer rows.Close()

	var rels []domain.Relation
	for rows.Next() {
		var r domain.Relation
		err := rows.Scan(&r.ID, &r.Project, &r.MemoryIDA, &r.MemoryIDB, &r.Relation, &r.Confidence, &r.Reasoning, &r.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		rels = append(rels, r)
	}
	if rels == nil {
		rels = []domain.Relation{}
	}
	return rels, rows.Err()
}

// ListAllRelations devuelve TODAS las relaciones del proyecto (sin tope), para
// el export. ListRelations está acotado a 50.
func ListAllRelations(db *sql.DB, project string) ([]domain.Relation, error) {
	rows, err := db.Query(
		`SELECT id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at
		 FROM memory_relations WHERE project = ? ORDER BY id ASC`,
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("list all relations: %w", err)
	}
	defer rows.Close()

	var rels []domain.Relation
	for rows.Next() {
		var r domain.Relation
		if err := rows.Scan(&r.ID, &r.Project, &r.MemoryIDA, &r.MemoryIDB, &r.Relation, &r.Confidence, &r.Reasoning, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		rels = append(rels, r)
	}
	if rels == nil {
		rels = []domain.Relation{}
	}
	return rels, rows.Err()
}

// ImportRelation inserta una relación PRESERVANDO su created_at de origen (si
// viene vacío, usa el reloj local). Los ids de memoria ya vienen remapeados al
// proyecto destino por el use case de import.
func ImportRelation(db *sql.DB, r *domain.Relation) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, COALESCE(NULLIF(?,''), `+Now+`))`,
		r.Project, r.MemoryIDA, r.MemoryIDB, string(r.Relation), r.Confidence, r.Reasoning, r.CreatedAt,
	)
	if err != nil {
		return 0, fmt.Errorf("import relation: %w", err)
	}
	return res.LastInsertId()
}
