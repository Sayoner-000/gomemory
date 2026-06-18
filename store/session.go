package store

import (
	"crypto/rand"
	"database/sql"
	"fmt"

	"mem/types"
)

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func StartSession(db *sql.DB, project string) (*types.Session, error) {
	id := newID()
	_, err := db.Exec(
		`INSERT INTO sessions (id, project, created_at) VALUES (?, ?, `+Now+`)`,
		id, project,
	)
	if err != nil {
		return nil, fmt.Errorf("start session: %w", err)
	}
	return &types.Session{
		ID:        id,
		Project:   project,
		CreatedAt: "",
	}, nil
}

func EndSession(db *sql.DB, id, summary string) error {
	res, err := db.Exec(
		`UPDATE sessions SET ended_at = `+Now+`, summary = ? WHERE id = ? AND ended_at IS NULL`,
		summary, id,
	)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("session %s not found or already ended", id)
	}
	return nil
}

func ActiveSession(db *sql.DB, project string) (*types.Session, error) {
	var s types.Session
	var endedAt sql.NullString
	err := db.QueryRow(
		`SELECT id, project, COALESCE(summary,''), created_at, ended_at FROM sessions
		 WHERE project = ? AND ended_at IS NULL ORDER BY created_at DESC LIMIT 1`,
		project,
	).Scan(&s.ID, &s.Project, &s.Summary, &s.CreatedAt, &endedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("active session: %w", err)
	}
	if endedAt.Valid {
		s.EndedAt = &endedAt.String
	}
	return &s, nil
}

func RecentSessions(db *sql.DB, project string, limit int) ([]types.Session, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	rows, err := db.Query(
		`SELECT id, project, COALESCE(summary,''), created_at, COALESCE(ended_at,'') FROM sessions
		 WHERE project = ? ORDER BY created_at DESC LIMIT ?`,
		project, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []types.Session
	for rows.Next() {
		var s types.Session
		var endedAt string
		err := rows.Scan(&s.ID, &s.Project, &s.Summary, &s.CreatedAt, &endedAt)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if endedAt != "" {
			s.EndedAt = &endedAt
		}
		sessions = append(sessions, s)
	}
	if sessions == nil {
		sessions = []types.Session{}
	}
	return sessions, rows.Err()
}
