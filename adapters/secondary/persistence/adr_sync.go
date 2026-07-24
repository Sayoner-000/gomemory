package persistence

import (
	"database/sql"
	"fmt"

	"mem/domain"
)

// InsertADRSyncRecord persiste un registro de sincronización memoria↔bloque
// ADR (feature 010, Historia 2). Los índices únicos (project, memory_id) y
// (project, provider, block_key) hacen que un segundo INSERT para el mismo
// par falle — el llamador decide si eso significa "ya sincronizado, hacer
// update" (ver findDuplicate/dedup en memory.go para el mismo patrón).
func InsertADRSyncRecord(db *sql.DB, r *domain.ADRSyncRecord) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO adr_sync_records (project, memory_id, provider, section, block_key, origin, status, content_hash, last_synced_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, `+Now+`, `+Now+`)`,
		r.Project, nullableMemoryID(r.MemoryID), r.Provider, r.Section, r.BlockKey,
		string(r.Origin), string(r.Status), r.ContentHash,
	)
	if err != nil {
		return 0, fmt.Errorf("insert adr sync record: %w", err)
	}
	return res.LastInsertId()
}

// GetADRSyncByMemory busca el registro de sincronización de una memoria
// (por definición hay a lo sumo uno, por el índice único). nil, nil si no
// existe.
func GetADRSyncByMemory(db *sql.DB, project string, memoryID int64) (*domain.ADRSyncRecord, error) {
	return scanADRSyncRow(db.QueryRow(
		`SELECT id, project, memory_id, provider, section, block_key, origin, status, content_hash, last_synced_at, created_at
		 FROM adr_sync_records WHERE project = ? AND memory_id = ?`,
		project, memoryID,
	))
}

// GetADRSyncByBlockKey busca el registro por su identidad del lado del
// proveedor (block_key). nil, nil si no existe.
func GetADRSyncByBlockKey(db *sql.DB, project, provider, blockKey string) (*domain.ADRSyncRecord, error) {
	return scanADRSyncRow(db.QueryRow(
		`SELECT id, project, memory_id, provider, section, block_key, origin, status, content_hash, last_synced_at, created_at
		 FROM adr_sync_records WHERE project = ? AND provider = ? AND block_key = ?`,
		project, provider, blockKey,
	))
}

// UpdateADRSyncStatus actualiza el resultado del último intento de
// sincronización (en cualquier sentido) de un registro existente.
func UpdateADRSyncStatus(db *sql.DB, id int64, status domain.SyncStatus, contentHash string) error {
	res, err := db.Exec(
		`UPDATE adr_sync_records SET status = ?, content_hash = ?, last_synced_at = `+Now+` WHERE id = ?`,
		string(status), contentHash, id,
	)
	if err != nil {
		return fmt.Errorf("update adr sync status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("adr sync record %d not found", id)
	}
	return nil
}

// ListADRSyncRecords devuelve todos los registros de sincronización de un
// proyecto, más recientes primero. Usado por `mem adr-sync status`.
func ListADRSyncRecords(db *sql.DB, project string) ([]domain.ADRSyncRecord, error) {
	rows, err := db.Query(
		`SELECT id, project, memory_id, provider, section, block_key, origin, status, content_hash, last_synced_at, created_at
		 FROM adr_sync_records WHERE project = ? ORDER BY last_synced_at DESC`,
		project,
	)
	if err != nil {
		return nil, fmt.Errorf("list adr sync records: %w", err)
	}
	defer rows.Close()

	var recs []domain.ADRSyncRecord
	for rows.Next() {
		var r domain.ADRSyncRecord
		var memID sql.NullInt64
		var origin, status string
		if err := rows.Scan(&r.ID, &r.Project, &memID, &r.Provider, &r.Section, &r.BlockKey, &origin, &status, &r.ContentHash, &r.LastSyncedAt, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan adr sync record: %w", err)
		}
		if memID.Valid {
			r.MemoryID = &memID.Int64
		}
		r.Origin = domain.ValidSyncOrigin(origin)
		r.Status = domain.ValidSyncStatus(status)
		recs = append(recs, r)
	}
	if recs == nil {
		recs = []domain.ADRSyncRecord{}
	}
	return recs, rows.Err()
}

func scanADRSyncRow(row *sql.Row) (*domain.ADRSyncRecord, error) {
	var r domain.ADRSyncRecord
	var memID sql.NullInt64
	var origin, status string
	err := row.Scan(&r.ID, &r.Project, &memID, &r.Provider, &r.Section, &r.BlockKey, &origin, &status, &r.ContentHash, &r.LastSyncedAt, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get adr sync record: %w", err)
	}
	if memID.Valid {
		r.MemoryID = &memID.Int64
	}
	r.Origin = domain.ValidSyncOrigin(origin)
	r.Status = domain.ValidSyncStatus(status)
	return &r, nil
}

func nullableMemoryID(id *int64) any {
	if id == nil {
		return nil
	}
	return *id
}
