package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"mem/application/ports"
)

// DeliveredBlocksRepository implementa ports.DeliveredBlocksRepository sobre la
// tabla delivered_blocks (feature 033). Igual que DeliveryLogRepository,
// resuelve la sesión activa en CADA llamada: una sesión puede abrirse o
// cerrarse durante la vida del proceso.
type DeliveredBlocksRepository struct {
	db      *sql.DB
	project string
	clock   ports.ClockPort
}

func NewDeliveredBlocksRepository(db *sql.DB, project string, clock ports.ClockPort) *DeliveredBlocksRepository {
	return &DeliveredBlocksRepository{db: db, project: project, clock: clock}
}

func (r *DeliveredBlocksRepository) sessionID() string {
	s, err := ActiveSession(r.db, r.project)
	if err != nil || s == nil {
		return ""
	}
	return s.ID
}

func (r *DeliveredBlocksRepository) Seen(ctx context.Context, hashes []string) (map[string]bool, error) {
	out := map[string]bool{}
	sid := r.sessionID()
	if sid == "" || len(hashes) == 0 {
		return out, nil
	}
	// Por lotes: SQLite limita el número de parámetros por sentencia.
	const batch = 400
	for start := 0; start < len(hashes); start += batch {
		end := min(start+batch, len(hashes))
		chunk := hashes[start:end]
		args := make([]any, 0, len(chunk)+1)
		args = append(args, sid)
		for _, h := range chunk {
			args = append(args, h)
		}
		q := `SELECT block_hash FROM delivered_blocks WHERE session_id = ? AND block_hash IN (?` +
			strings.Repeat(",?", len(chunk)-1) + `)`
		rows, err := r.db.QueryContext(ctx, q, args...)
		if err != nil {
			return nil, fmt.Errorf("leer bloques entregados: %w", err)
		}
		for rows.Next() {
			var h string
			if err := rows.Scan(&h); err != nil {
				_ = rows.Close()
				return nil, err
			}
			out[h] = true
		}
		_ = rows.Close()
	}
	return out, nil
}

func (r *DeliveredBlocksRepository) Mark(ctx context.Context, hashes []string) error {
	sid := r.sessionID()
	if sid == "" || len(hashes) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("registrar bloques entregados: %w", err)
	}
	now := formatTS(r.clock.Now())
	for _, h := range hashes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO delivered_blocks (session_id, block_hash, delivered_at) VALUES (?, ?, ?)
			 ON CONFLICT(session_id, block_hash) DO UPDATE SET delivered_at = excluded.delivered_at`, sid, h, now); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("registrar bloque entregado: %w", err)
		}
	}
	return tx.Commit()
}

func (r *DeliveredBlocksRepository) Reset(ctx context.Context) error {
	sid := r.sessionID()
	if sid == "" {
		return nil
	}
	if _, err := r.db.ExecContext(ctx, `DELETE FROM delivered_blocks WHERE session_id = ?`, sid); err != nil {
		return fmt.Errorf("olvidar bloques entregados: %w", err)
	}
	return nil
}

// ResetContextDeliveries borra lo que el registro por canal (context,
// plan_context) anotó en una sesión. Tras compactar, la supresión de
// get_plan_context no debe creer que el agente conserva el contexto.
func ResetContextDeliveries(db *sql.DB, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if _, err := db.Exec(`DELETE FROM context_deliveries WHERE session_id = ?`, sessionID); err != nil {
		return fmt.Errorf("olvidar entregas de contexto: %w", err)
	}
	return nil
}

// Reset implementa ports.DeliveryLog.Reset sobre la sesión activa.
func (r *DeliveryLogRepository) Reset() error {
	return ResetContextDeliveries(r.db, r.sessionID())
}
