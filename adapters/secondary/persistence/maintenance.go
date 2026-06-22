package persistence

import (
	"database/sql"
	"fmt"
	"os"

	"mem/application/ports"
)

func StatsQuery(db *sql.DB, dbPath, project string) (projectCount, totalCount int64, err error) {
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories WHERE project = ?`, project).Scan(&projectCount); err != nil {
		return 0, 0, fmt.Errorf("count project memories: %w", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM memories`).Scan(&totalCount); err != nil {
		return 0, 0, fmt.Errorf("count total memories: %w", err)
	}
	return projectCount, totalCount, nil
}

func FileSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, fmt.Errorf("stat %s: %w", path, err)
	}
	return info.Size(), nil
}

// PurgeMemories borra memorias según filter, dentro de una sola transacción,
// y limpia cualquier memory_relations que haya quedado huérfana (no hay
// ON DELETE CASCADE declarado en el esquema — ver research.md punto 4).
func PurgeMemories(db *sql.DB, filter ports.PurgeFilter) (int64, error) {
	if !filter.All && filter.Project == "" {
		return 0, fmt.Errorf("purge: alcance ambiguo — se requiere Project o All=true")
	}

	tx, err := db.Begin()
	if err != nil {
		return 0, fmt.Errorf("purge: begin tx: %w", err)
	}
	defer tx.Rollback()

	query := `DELETE FROM memories WHERE 1=1`
	args := []interface{}{}

	if !filter.All {
		query += ` AND project = ?`
		args = append(args, filter.Project)
	}
	if filter.Type != "" {
		query += ` AND type = ?`
		args = append(args, filter.Type)
	}
	if filter.OlderThanDays > 0 {
		query += fmt.Sprintf(` AND created_at < datetime('now', '-%d days')`, filter.OlderThanDays)
	}

	res, err := tx.Exec(query, args...)
	if err != nil {
		return 0, fmt.Errorf("purge: delete memories: %w", err)
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("purge: rows affected: %w", err)
	}

	if _, err := tx.Exec(`DELETE FROM memory_relations
		WHERE memory_id_a NOT IN (SELECT id FROM memories)
		   OR memory_id_b NOT IN (SELECT id FROM memories)`); err != nil {
		return 0, fmt.Errorf("purge: clean orphan relations: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("purge: commit: %w", err)
	}

	return deleted, nil
}

// CompactDB ejecuta VACUUM para reclamar el espacio dejado por filas
// borradas (FR-006) — nunca toca ni pierde filas sobrevivientes.
func CompactDB(db *sql.DB, dbPath string) (beforeBytes, afterBytes int64, err error) {
	before, err := FileSize(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("compact: %w", err)
	}

	if _, err := db.Exec(`VACUUM;`); err != nil {
		return 0, 0, fmt.Errorf("compact: vacuum: %w", err)
	}

	after, err := FileSize(dbPath)
	if err != nil {
		return 0, 0, fmt.Errorf("compact: %w", err)
	}

	return before, after, nil
}
