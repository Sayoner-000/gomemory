package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"mem/application/ports"
	"mem/domain"
)

// PrivacyMaintenanceStore implementa el saneamiento histórico sobre SQLite
// (spec 037, US2): lectura del texto original, transacción inmediata con
// secure_delete y compactación con checkpoint verificado. Las sentencias son
// fijas por categoría; los valores van siempre como parámetros.
type PrivacyMaintenanceStore struct {
	db *sql.DB
}

var (
	_ ports.PrivacyMaintenanceRepository = (*PrivacyMaintenanceStore)(nil)
	_ ports.PrivacyMaintenanceUnitOfWork = (*PrivacyMaintenanceStore)(nil)
)

func NewPrivacyMaintenanceStore(db *sql.DB) *PrivacyMaintenanceStore {
	return &PrivacyMaintenanceStore{db: db}
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Candidates lee los candidatos fuera de transacción.
func (s *PrivacyMaintenanceStore) Candidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error) {
	return readSanitizeCandidates(ctx, s.db, project)
}

// readSanitizeCandidates NO usa los lectores depurados del paquete: estos
// ocultarían justamente el texto que hay que sanear.
func readSanitizeCandidates(ctx context.Context, q queryer, project string) ([]domain.SanitizeCandidate, error) {
	var out []domain.SanitizeCandidate
	add := func(cat domain.SanitizeCategory, field, id, text string) {
		out = append(out, domain.SanitizeCandidate{Category: cat, RecordID: id, Project: project, Field: field, Text: text})
	}

	err := scanRows(ctx, q, `SELECT id, COALESCE(summary,''), COALESCE(last_prompt,'') FROM sessions WHERE project = ?`, project, func(r *sql.Rows) error {
		var id, text, prompt string
		if err := r.Scan(&id, &text, &prompt); err != nil {
			return err
		}
		if domain.NeedsSanitize(text) {
			add(domain.SanitizeSession, "summary", id, text)
		}
		if domain.NeedsSanitize(prompt) {
			add(domain.SanitizeSessionPrompt, "last_prompt", id, prompt)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("candidatos de sesiones: %w", err)
	}

	err = scanRows(ctx, q, `SELECT id, reasoning FROM memory_relations WHERE project = ?`, project, func(r *sql.Rows) error {
		var id int64
		var text string
		if err := r.Scan(&id, &text); err != nil {
			return err
		}
		if domain.NeedsSanitize(text) {
			add(domain.SanitizeRelation, "reasoning", strconv.FormatInt(id, 10), text)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("candidatos de relaciones: %w", err)
	}

	err = scanRows(ctx, q, `SELECT agent, scope, kind, last_error FROM channel_activity WHERE project = ?`, project, func(r *sql.Rows) error {
		var agent, scope, kind, text string
		if err := r.Scan(&agent, &scope, &kind, &text); err != nil {
			return err
		}
		if domain.NeedsSanitize(text) {
			add(domain.SanitizeChannelError, "last_error", channelRecordID(agent, scope, kind), text)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("candidatos de diagnósticos: %w", err)
	}

	err = scanRows(ctx, q, `SELECT ref, content_gz FROM compression_originals WHERE project = ?`, project, func(r *sql.Rows) error {
		var ref string
		var gz []byte
		if err := r.Scan(&ref, &gz); err != nil {
			return err
		}
		text, err := gunzipBytes(gz)
		if err != nil {
			return err
		}
		if domain.IsPrivateContent(text) {
			add(domain.SanitizeOriginal, "content", ref, text)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("candidatos de originales: %w", err)
	}

	if err := readSocraticaCandidates(ctx, q, project, add); err != nil {
		return nil, err
	}
	if err := readLedgerCandidates(ctx, q, project, add); err != nil {
		return nil, err
	}

	// Memorias: siempre se guardaron depuradas, pero las anteriores a la
	// política de credenciales o con remanentes de bloques anidados (cierre
	// huérfano) no se pueden reescribir con seguridad. Solo se reportan.
	err = scanRows(ctx, q, `SELECT id, title, content, COALESCE(origin_prompt,'') FROM memories WHERE project = ?`, project, func(r *sql.Rows) error {
		var id int64
		var title, content, origin string
		if err := r.Scan(&id, &title, &content, &origin); err != nil {
			return err
		}
		for _, text := range []string{title, content, origin} {
			if domain.NeedsSanitize(text) || domain.HasOrphanPrivateClose(text) {
				add(domain.SanitizeMemoryReview, "content", strconv.FormatInt(id, 10), content)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("candidatos de memorias: %w", err)
	}
	return out, nil
}

// ledgerColumn describe una columna sensible del ledger de revisión. Las
// tablas no tienen columna project: el alcance sale de reviews.project.
type ledgerColumn struct {
	cat    domain.SanitizeCategory
	table  string
	column string
	scope  string // condición SQL sobre review_id/reviewer_result_id que fija el proyecto
	list   bool   // columna JSON con una lista de textos
}

const (
	scopeByReview = `review_id IN (SELECT id FROM reviews WHERE project = ?)`
	scopeByResult = `reviewer_result_id IN (SELECT rr.id FROM reviewer_results rr
		JOIN reviews rv ON rv.id = rr.review_id WHERE rv.project = ?)`
)

var ledgerColumns = []ledgerColumn{
	{domain.SanitizeReviewFinding, "findings", "claim", scopeByResult, false},
	{domain.SanitizeReviewFindingEvidence, "findings", "evidence", scopeByResult, true},
	{domain.SanitizeReviewConsensus, "consensus_findings", "claim", scopeByReview, false},
	{domain.SanitizeReviewRejudgment, "rejudgments", "evidence", scopeByReview, true},
	{domain.SanitizeReviewFixVerification, "fix_rounds", "verification", scopeByReview, true},
	// El alcance de la revisión lo aporta quien llama a review_start (H-04).
	{domain.SanitizeReviewTargetScope, "reviews", "target_scope", `project = ?`, true},
}

func ledgerColumnFor(cat domain.SanitizeCategory) (ledgerColumn, bool) {
	for _, col := range ledgerColumns {
		if col.cat == cat {
			return col, true
		}
	}
	return ledgerColumn{}, false
}

func readLedgerCandidates(ctx context.Context, q queryer, project string,
	add func(domain.SanitizeCategory, string, string, string)) error {
	for _, col := range ledgerColumns {
		query := `SELECT id, ` + col.column + ` FROM ` + col.table + ` WHERE ` + col.scope
		err := scanRows(ctx, q, query, project, func(r *sql.Rows) error {
			var id int64
			var text string
			if err := r.Scan(&id, &text); err != nil {
				return err
			}
			needs := domain.NeedsSanitize(text)
			if col.list {
				var err error
				if _, needs, err = protectJSONList(text); err != nil {
					return err
				}
			}
			if needs {
				add(col.cat, col.column, strconv.FormatInt(id, 10), text)
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("candidatos de %s.%s: %w", col.table, col.column, err)
		}
	}
	return nil
}

// protectJSONList protege cada elemento de una lista JSON y la vuelve a
// serializar como la escriben los adaptadores (json.Marshal). Proteger el JSON
// como texto no basta: json.Marshal escapa `<` como `\u003c`, así que un bloque
// privado no se ve, y una región que cruce elementos rompería la estructura.
func protectJSONList(raw string) (string, bool, error) {
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return "", false, fmt.Errorf("%w: lista JSON ilegible en el ledger", domain.ErrValidation)
	}
	changed := false
	for i := range items {
		if domain.NeedsSanitize(items[i]) {
			items[i] = domain.ProtectText(items[i])
			changed = true
		}
	}
	if !changed {
		return raw, false, nil
	}
	out, err := json.Marshal(items)
	return string(out), true, err
}

func scanRows(ctx context.Context, q queryer, query, project string, each func(*sql.Rows) error) error {
	rows, err := q.QueryContext(ctx, query, project)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

// channelRecordID codifica la identidad compuesta de channel_activity de
// forma reversible y legible en el informe.
func channelRecordID(agent, scope, kind string) string {
	return url.PathEscape(agent) + "/" + url.PathEscape(scope) + "/" + url.PathEscape(kind)
}

func parseChannelRecordID(id string) (agent, scope, kind string, err error) {
	parts := strings.Split(id, "/")
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("%w: identidad de diagnóstico inválida", domain.ErrValidation)
	}
	out := make([]string, 3)
	for i, p := range parts {
		if out[i], err = url.PathUnescape(p); err != nil {
			return "", "", "", fmt.Errorf("%w: identidad de diagnóstico inválida", domain.ErrValidation)
		}
	}
	return out[0], out[1], out[2], nil
}

// WithTransaction ejecuta fn en una transacción inmediata sobre una conexión
// dedicada con secure_delete activo: las páginas liberadas se sobrescriben
// en vez de quedar con el texto anterior.
func (s *PrivacyMaintenanceStore) WithTransaction(ctx context.Context, project string, fn func(ports.PrivacyTransaction) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return classifySQLite(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete = ON`); err != nil {
		return classifySQLite(err)
	}
	defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA secure_delete = OFF`) }()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return classifySQLite(err)
	}
	if err := fn(&privacyTx{tx: tx, project: project}); err != nil {
		_ = tx.Rollback()
		return classifySQLite(err)
	}
	if err := ctx.Err(); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return classifySQLite(err)
	}
	return nil
}

type privacyTx struct {
	tx      *sql.Tx
	project string
}

func (t *privacyTx) Candidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error) {
	return readSanitizeCandidates(ctx, t.tx, project)
}

// Apply escribe solo si el valor actual sigue siendo c.Text: un escritor que
// lo cambió entre la lectura y la escritura no se pisa.
func (t *privacyTx) Apply(ctx context.Context, c domain.SanitizeCandidate, text string) (bool, error) {
	if err := c.Validate(t.project); err != nil {
		return false, err
	}
	if col, ok := socraticaSanitizeColumn(c.Category); ok {
		return applySocraticaCandidate(ctx, t.tx, col, c)
	}
	var res sql.Result
	var err error
	switch c.Category {
	case domain.SanitizeSession:
		res, err = t.tx.ExecContext(ctx,
			`UPDATE sessions SET summary = ? WHERE id = ? AND project = ? AND COALESCE(summary,'') = ?`,
			text, c.RecordID, c.Project, c.Text)
	case domain.SanitizeSessionPrompt:
		res, err = t.tx.ExecContext(ctx,
			`UPDATE sessions SET last_prompt = ? WHERE id = ? AND project = ? AND COALESCE(last_prompt,'') = ?`,
			text, c.RecordID, c.Project, c.Text)
	case domain.SanitizeRelation:
		id, perr := strconv.ParseInt(c.RecordID, 10, 64)
		if perr != nil {
			return false, fmt.Errorf("%w: id de relación inválido", domain.ErrValidation)
		}
		res, err = t.tx.ExecContext(ctx,
			`UPDATE memory_relations SET reasoning = ? WHERE id = ? AND project = ? AND reasoning = ?`,
			text, id, c.Project, c.Text)
	case domain.SanitizeChannelError:
		agent, scope, kind, perr := parseChannelRecordID(c.RecordID)
		if perr != nil {
			return false, perr
		}
		res, err = t.tx.ExecContext(ctx,
			`UPDATE channel_activity SET last_error = ? WHERE project = ? AND agent = ? AND scope = ? AND kind = ? AND last_error = ?`,
			text, c.Project, agent, scope, kind, c.Text)
	case domain.SanitizeOriginal:
		// Un original prometía recuperación exacta: no se reescribe, se borra.
		res, err = t.tx.ExecContext(ctx,
			`DELETE FROM compression_originals WHERE ref = ? AND project = ?`, c.RecordID, c.Project)
	default:
		col, ok := ledgerColumnFor(c.Category)
		if !ok {
			return false, fmt.Errorf("%w: categoría solo informativa", domain.ErrValidation)
		}
		id, perr := strconv.ParseInt(c.RecordID, 10, 64)
		if perr != nil {
			return false, fmt.Errorf("%w: id del ledger inválido", domain.ErrValidation)
		}
		if col.list {
			// La protección por elemento se recalcula aquí: text viene de
			// proteger el JSON serializado, que no ve \u003cprivate\u003e.
			if text, _, err = protectJSONList(c.Text); err != nil {
				return false, err
			}
		}
		res, err = t.tx.ExecContext(ctx,
			`UPDATE `+col.table+` SET `+col.column+` = ? WHERE id = ? AND `+col.column+` = ? AND `+col.scope,
			text, id, c.Text, c.Project)
	}
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Compact reconstruye la base y vacía el WAL. Nunca borra -wal/-shm por
// operaciones de archivo: el checkpoint TRUNCATE lo deja en cero bytes
// cuando no hay lectores ni escritores, y si los hay lo informa como BUSY.
func (s *PrivacyMaintenanceStore) Compact(ctx context.Context) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return classifySQLite(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
		return classifySQLite(err)
	}
	var busy, logFrames, checkpointed int
	if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return classifySQLite(err)
	}
	if busy != 0 {
		return domain.NewSanitizeError(domain.SanitizeCodeBusy, errors.New("checkpoint bloqueado por otra conexión"))
	}
	return nil
}

// classifySQLite convierte un bloqueo de SQLite en SANITIZE_BUSY; el resto
// se devuelve tal cual para que el caso de uso lo clasifique.
func classifySQLite(err error) error {
	if err != nil && isSQLiteBusy(err) {
		return domain.NewSanitizeError(domain.SanitizeCodeBusy, err)
	}
	return err
}
