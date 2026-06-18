package store

import (
	"database/sql"
	"fmt"

	"mem/types"
)

func InsertMemory(db *sql.DB, m *types.Memory) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO memories (project, session_id, type, title, content, filepath, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, `+Now+`, `+Now+`)`,
		m.Project, m.SessionID, string(m.Type), m.Title, m.Content, m.Filepath,
	)
	if err != nil {
		return 0, fmt.Errorf("insert memory: %w", err)
	}
	return res.LastInsertId()
}

func ListMemories(db *sql.DB, project string, limit int) ([]types.Memory, error) {
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

	var mems []types.Memory
	for rows.Next() {
		var m types.Memory
		var memType string
		err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = types.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []types.Memory{}
	}
	return mems, rows.Err()
}

func SearchMemories(db *sql.DB, project, query string, limit int) ([]types.Memory, error) {
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

	var mems []types.Memory
	for rows.Next() {
		var m types.Memory
		var memType string
		err := rows.Scan(&m.ID, &m.Project, &m.SessionID, &memType, &m.Title,
			&m.Content, &m.Filepath, &m.CreatedAt, &m.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan memory: %w", err)
		}
		m.Type = types.MemoryType(memType)
		mems = append(mems, m)
	}
	if mems == nil {
		mems = []types.Memory{}
	}
	return mems, rows.Err()
}
