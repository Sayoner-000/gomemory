package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"mem/application/ports"
	"mem/domain"
)

// CompressionStatsRepository implementa ports.CompressionStatsRepository sobre
// compression_stats (feature 033, FR-026). Solo cifras: nunca contenido.
type CompressionStatsRepository struct {
	db    *sql.DB
	clock ports.ClockPort
}

func NewCompressionStatsRepository(db *sql.DB, clock ports.ClockPort) *CompressionStatsRepository {
	return &CompressionStatsRepository{db: db, clock: clock}
}

func orUnknown(s string) string {
	if s == "" {
		return string(domain.ContentUnknown)
	}
	return s
}

func (r *CompressionStatsRepository) Record(ctx context.Context, project string, res ports.CompressionResult) error {
	fallback := 0
	if res.FallbackReason != "" {
		fallback = 1
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO compression_stats (project, compressor, content_type, uses, raw_tokens, structural_tokens, final_tokens, omissions, retrievals, fallbacks, latency_us_total, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?, ?, 0, ?, ?, ?)
		ON CONFLICT(project, compressor, content_type) DO UPDATE SET
			uses = uses + 1,
			raw_tokens = raw_tokens + excluded.raw_tokens,
			structural_tokens = structural_tokens + excluded.structural_tokens,
			final_tokens = final_tokens + excluded.final_tokens,
			omissions = omissions + excluded.omissions,
			fallbacks = fallbacks + excluded.fallbacks,
			latency_us_total = latency_us_total + excluded.latency_us_total,
			updated_at = excluded.updated_at`,
		project, orUnknown(res.Compressor), orUnknown(res.ContentType), res.RawTokens, res.StructuralTokens, res.Tokens,
		res.Omissions, fallback, res.LatencyMicros, formatTS(r.clock.Now()))
	if err != nil {
		return fmt.Errorf("registrar estadística de compresión: %w", err)
	}
	return nil
}

func (r *CompressionStatsRepository) RecordRetrieval(ctx context.Context, project, compressor, contentType string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO compression_stats (project, compressor, content_type, retrievals, updated_at)
		VALUES (?, ?, ?, 1, ?)
		ON CONFLICT(project, compressor, content_type) DO UPDATE SET
			retrievals = retrievals + 1, updated_at = excluded.updated_at`,
		project, orUnknown(compressor), orUnknown(contentType), formatTS(r.clock.Now()))
	if err != nil {
		return fmt.Errorf("registrar recuperación: %w", err)
	}
	return nil
}

func (r *CompressionStatsRepository) Summary(ctx context.Context, project string) ([]ports.CompressorStats, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT compressor, content_type, uses, raw_tokens, structural_tokens, final_tokens, omissions, retrievals, fallbacks, latency_us_total
		FROM compression_stats WHERE project = ? ORDER BY compressor, content_type`, project)
	if err != nil {
		return nil, fmt.Errorf("leer estadísticas de compresión: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ports.CompressorStats
	for rows.Next() {
		var s ports.CompressorStats
		if err := rows.Scan(&s.Compressor, &s.ContentType, &s.Uses, &s.RawTokens, &s.StructuralTokens, &s.FinalTokens,
			&s.Omissions, &s.Retrievals, &s.Fallbacks, &s.LatencyMicrosTotal); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// CompressionTuningRepository implementa ports.CompressionTuningRepository
// (FR-027). Sin fila, la agresividad es la máxima.
type CompressionTuningRepository struct {
	db    *sql.DB
	clock ports.ClockPort
}

func NewCompressionTuningRepository(db *sql.DB, clock ports.ClockPort) *CompressionTuningRepository {
	return &CompressionTuningRepository{db: db, clock: clock}
}

func (r *CompressionTuningRepository) Get(ctx context.Context, project, contentType string) (domain.Aggressiveness, error) {
	var a int
	err := r.db.QueryRowContext(ctx, `SELECT aggressiveness FROM compression_tuning WHERE project = ? AND content_type = ?`, project, contentType).Scan(&a)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AggressivenessMax, nil
	}
	if err != nil {
		return domain.AggressivenessMax, fmt.Errorf("leer ajuste de compresión: %w", err)
	}
	return domain.Aggressiveness(a), nil
}

// counts devuelve las omisiones y recuperaciones acumuladas de un tipo.
func (r *CompressionTuningRepository) counts(ctx context.Context, project, contentType string) (int, int, error) {
	var om, ret int
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(omissions), 0), COALESCE(SUM(retrievals), 0)
		FROM compression_stats WHERE project = ? AND content_type = ?`, project, contentType).Scan(&om, &ret)
	return om, ret, err
}

// SinceLastAdjustment devuelve omisiones y recuperaciones de un tipo desde su
// último ajuste (o desde siempre, si no lo hubo).
func (r *CompressionTuningRepository) SinceLastAdjustment(ctx context.Context, project, contentType string) (int, int, error) {
	om, ret, err := r.counts(ctx, project, contentType)
	if err != nil {
		return 0, 0, fmt.Errorf("leer omisiones: %w", err)
	}
	var omB, retB int
	err = r.db.QueryRowContext(ctx, `SELECT omissions_base, retrievals_base FROM compression_tuning WHERE project = ? AND content_type = ?`,
		project, contentType).Scan(&omB, &retB)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, fmt.Errorf("leer línea base: %w", err)
	}
	return om - omB, ret - retB, nil
}

func (r *CompressionTuningRepository) Lower(ctx context.Context, project, contentType, reason string) error {
	cur, err := r.Get(ctx, project, contentType)
	if err != nil {
		return err
	}
	next := cur - 1
	if next < domain.AggressivenessOff {
		next = domain.AggressivenessOff
	}
	om, ret, err := r.counts(ctx, project, contentType)
	if err != nil {
		return fmt.Errorf("leer omisiones: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO compression_tuning (project, content_type, aggressiveness, reason, omissions_base, retrievals_base, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(project, content_type) DO UPDATE SET
			aggressiveness = excluded.aggressiveness, reason = excluded.reason,
			omissions_base = excluded.omissions_base, retrievals_base = excluded.retrievals_base,
			updated_at = excluded.updated_at`,
		project, contentType, int(next), reason, om, ret, formatTS(r.clock.Now()))
	if err != nil {
		return fmt.Errorf("bajar agresividad: %w", err)
	}
	return nil
}

func (r *CompressionTuningRepository) Reset(ctx context.Context, project, contentType string) error {
	var err error
	if contentType == "" {
		_, err = r.db.ExecContext(ctx, `DELETE FROM compression_tuning WHERE project = ?`, project)
	} else {
		_, err = r.db.ExecContext(ctx, `DELETE FROM compression_tuning WHERE project = ? AND content_type = ?`, project, contentType)
	}
	if err != nil {
		return fmt.Errorf("restablecer ajuste: %w", err)
	}
	return nil
}

func (r *CompressionTuningRepository) List(ctx context.Context, project string) ([]ports.TuningEntry, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT content_type, aggressiveness, reason, updated_at FROM compression_tuning WHERE project = ? ORDER BY content_type`, project)
	if err != nil {
		return nil, fmt.Errorf("listar ajustes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []ports.TuningEntry
	for rows.Next() {
		var e ports.TuningEntry
		var a int
		var ts string
		if err := rows.Scan(&e.ContentType, &a, &e.Reason, &ts); err != nil {
			return nil, err
		}
		e.Aggressiveness = domain.Aggressiveness(a)
		e.UpdatedAt = parseTS(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}
