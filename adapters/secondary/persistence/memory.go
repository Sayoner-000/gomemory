package persistence

import (
	"database/sql"
	"fmt"
	"strings"

	"mem/domain"
)

func InsertMemory(db *sql.DB, m *domain.Memory) (int64, error) {
	title := domain.RedactPrivate(m.Title)
	content := domain.RedactPrivate(m.Content)
	if strings.TrimSpace(content) == "" {
		return 0, fmt.Errorf("insert memory: contenido vacío tras redactar <private>")
	}

	res, err := db.Exec(
		`INSERT INTO memories (project, session_id, type, title, content, filepath, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, `+Now+`, `+Now+`)`,
		m.Project, m.SessionID, string(m.Type), title, content, m.Filepath,
	)
	if err != nil {
		return 0, fmt.Errorf("insert memory: %w", err)
	}
	return res.LastInsertId()
}

func ListMemories(db *sql.DB, project string, limit int) ([]domain.Memory, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), created_at, updated_at
		 FROM memories WHERE project = ? ORDER BY created_at DESC LIMIT ?`,
		project, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()

	var mems []domain.Memory
	for rows.Next() {
		var m domain.Memory
		var memType string
		err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = domain.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []domain.Memory{}
	}
	return mems, rows.Err()
}

func DeleteMemory(db *sql.DB, project string, id int64) (bool, error) {
	res, err := db.Exec(`DELETE FROM memories WHERE id = ? AND project = ?`, id, project)
	if err != nil {
		return false, fmt.Errorf("delete memory: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete memory: %w", err)
	}
	return affected > 0, nil
}

func SearchMemories(db *sql.DB, project, query string, limit int) ([]domain.Memory, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	like := "%" + query + "%"
	rows, err := db.Query(
		`SELECT id, project, COALESCE(session_id,''), type, COALESCE(title,''), content,
		        COALESCE(filepath,''), created_at, updated_at
		 FROM memories WHERE project = ? AND (content LIKE ? OR title LIKE ?)
		 ORDER BY
		   CASE
		     WHEN title LIKE ? THEN 0
		     WHEN content LIKE ? THEN 1
		     ELSE 2
		   END,
		   created_at DESC
		 LIMIT ?`,
		project, like, like, like, like, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer rows.Close()

	var mems []domain.Memory
	for rows.Next() {
		var m domain.Memory
		var memType string
		err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = domain.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []domain.Memory{}
	}
	return mems, rows.Err()
}
