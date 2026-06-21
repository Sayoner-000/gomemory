package mcp

import (
	"database/sql"

	"mem/adapters/secondary/persistence"
)

func New(db *sql.DB, project string, port int) *Server {
	return NewWithRepos(
		persistence.NewMemoryRepository(db),
		persistence.NewSessionRepository(db),
		project, port,
	)
}
