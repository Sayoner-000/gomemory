package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"mem/application/ports"
	"mem/domain"
)

const instinctSchema = `
CREATE TABLE IF NOT EXISTS instinct_stats (
project TEXT NOT NULL,slug TEXT NOT NULL,kind TEXT NOT NULL,description TEXT NOT NULL,
observations INTEGER NOT NULL DEFAULT 0,corrections INTEGER NOT NULL DEFAULT 0,
confidence REAL NOT NULL DEFAULT 0.3,scope TEXT NOT NULL DEFAULT 'project',status TEXT NOT NULL DEFAULT 'proposed',
first_seen TEXT NOT NULL,last_seen TEXT NOT NULL,PRIMARY KEY(project,slug));
CREATE TABLE IF NOT EXISTS instinct_folded(project TEXT NOT NULL,obs_id TEXT NOT NULL,folded_at TEXT NOT NULL,PRIMARY KEY(project,obs_id));`

type InstinctStore struct {
	db      *sql.DB
	project string
}

func NewInstinctRepository(db *sql.DB, project string) *InstinctStore {
	return &InstinctStore{db, project}
}

func (s *InstinctStore) Fold(ctx context.Context, batch []domain.Observation, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	stamp := now.UTC().Format(time.RFC3339Nano)
	for _, obs := range batch {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if obs.ID == "" || obs.Key == "" {
			continue
		}
		res, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO instinct_folded(project,obs_id,folded_at) VALUES(?,?,?)`, s.project, obs.ID, stamp)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			continue
		}
		// Una observación pendiente anterior a la protección de claves puede
		// traer la credencial en la clave: se pliega sobre su forma protegida.
		key := domain.ProtectObservationKey(obs.Key)
		corrections := 0
		if obs.Kind == "correction" {
			corrections = 1
		}
		desc := domain.TruncateRunes(domain.StripTerminalControls(domain.RedactSecrets(domain.RedactPrivate(obs.Sample))), 200)
		_, err = tx.ExecContext(ctx, `INSERT INTO instinct_stats(project,slug,kind,description,observations,corrections,first_seen,last_seen) VALUES(?,?,?,?,1,?,?,?)
ON CONFLICT(project,slug) DO UPDATE SET observations=instinct_stats.observations+1,corrections=instinct_stats.corrections+excluded.corrections,last_seen=excluded.last_seen`, s.project, key, obs.Kind, desc, corrections, stamp, stamp)
		if err != nil {
			return err
		}
		var total, corr int
		if err := tx.QueryRowContext(ctx, `SELECT observations,corrections FROM instinct_stats WHERE project=? AND slug=?`, s.project, key).Scan(&total, &corr); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE instinct_stats SET confidence=? WHERE project=? AND slug=?`, domain.InstinctConfidence(total, corr), s.project, key); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *InstinctStore) PurgeFolded(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM instinct_folded WHERE project=? AND folded_at<?`, s.project, before.UTC().Format(time.RFC3339Nano))
	return err
}
func (s *InstinctStore) List(ctx context.Context, all bool) ([]domain.Instinct, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug,kind,description,observations,corrections,confidence,scope,status FROM instinct_stats WHERE project=? AND (? OR status!='rejected') ORDER BY confidence DESC,slug`, s.project, all)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Instinct{}
	for rows.Next() {
		var i domain.Instinct
		if err := rows.Scan(&i.Slug, &i.Kind, &i.Description, &i.Observations, &i.Corrections, &i.Confidence, &i.Scope, &i.Status); err != nil {
			return nil, err
		}
		// Un slug histórico aún sin sanear no sale con su credencial (A-001):
		// se muestra su forma protegida y SetStatus la resuelve.
		i.Slug = domain.ProtectObservationKey(i.Slug)
		i.Description = domain.StripTerminalControls(domain.RedactSecrets(domain.RedactPrivate(i.Description)))
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *InstinctStore) ForContext(ctx context.Context, minConfidence float64) ([]domain.Instinct, error) {
	all, err := s.List(ctx, false)
	out := []domain.Instinct{}
	for _, i := range all {
		if i.Confidence+1e-9 >= minConfidence {
			out = append(out, i)
		}
	}
	return out, err
}
func (s *InstinctStore) SetStatus(ctx context.Context, slug, status string) error {
	if status != "confirmed" && status != "rejected" {
		return errors.New("invalid instinct status")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// List muestra la forma protegida del slug; la revisión alcanza también
	// a las filas históricas cuyo slug guardado se reduce a ella (A-001).
	rows, err := tx.QueryContext(ctx, `SELECT slug FROM instinct_stats WHERE project=? AND (status='proposed' OR status=?)`, s.project, status)
	if err != nil {
		return err
	}
	var targets []string
	for rows.Next() {
		var stored string
		if err := rows.Scan(&stored); err != nil {
			_ = rows.Close()
			return err
		}
		if stored == slug || domain.ProtectObservationKey(stored) == slug {
			targets = append(targets, stored)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return errors.New("instinct not found or status is final")
	}
	for _, stored := range targets {
		if _, err := tx.ExecContext(ctx, `UPDATE instinct_stats SET status=? WHERE project=? AND slug=?`, status, s.project, stored); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Adopt inserta la fila local de una propuesta global con el estado
// revisado. Empieza sin observaciones y con la confianza inicial: el
// proyecto no la ha observado, así que no debe contarse como observador al
// registrar el patrón global. Si la fila ya existe, no cambia nada.
func (s *InstinctStore) Adopt(ctx context.Context, i domain.Instinct, status string, now time.Time) error {
	if status != "confirmed" && status != "rejected" {
		return errors.New("invalid instinct status")
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	_, err := s.db.ExecContext(ctx, `INSERT INTO instinct_stats(project,slug,kind,description,scope,status,first_seen,last_seen) VALUES(?,?,?,?,'global',?,?,?) ON CONFLICT(project,slug) DO NOTHING`,
		s.project, i.Slug, i.Kind, domain.ProtectText(i.Description), status, stamp, stamp)
	return err
}

type InstinctGlobalStore struct {
	db   *sql.DB
	path string
}

func OpenInstinctGlobal(dataHome string) (*InstinctGlobalStore, error) {
	if err := os.MkdirAll(dataHome, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataHome, "instincts.db")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(100)&_pragma=journal_mode(WAL)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA secure_delete=ON;
CREATE TABLE IF NOT EXISTS instinct_global(slug TEXT PRIMARY KEY,kind TEXT NOT NULL,description TEXT NOT NULL,projects TEXT NOT NULL DEFAULT '[]',confidence REAL NOT NULL DEFAULT 0.3,updated_at TEXT NOT NULL);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &InstinctGlobalStore{db: db, path: path}, nil
}
func (s *InstinctGlobalStore) Close() error { return s.db.Close() }

// UnavailableInstinctGlobal sustituye al store global cuando instincts.db no
// abre (directorio, archivo corrupto, disco lleno). Es un store auxiliar: su
// fallo no puede impedir guardar o leer memorias, pero quien sí lo necesita
// (mem expand instincts, mem sanitize) debe fallar de forma explícita en vez
// de saltárselo en silencio (H-04 de la auditoría de 040).
type UnavailableInstinctGlobal struct{ Err error }

var _ ports.InstinctGlobalRepository = UnavailableInstinctGlobal{}

func (u UnavailableInstinctGlobal) err() error {
	return fmt.Errorf("instincts globales no disponibles: %w", u.Err)
}
func (u UnavailableInstinctGlobal) SanitizeCandidates(context.Context, string) ([]domain.SanitizeCandidate, error) {
	return nil, u.err()
}
func (u UnavailableInstinctGlobal) RecordProject(context.Context, string, domain.Instinct, time.Time) error {
	return u.err()
}
func (u UnavailableInstinctGlobal) GlobalFor(context.Context, float64) ([]domain.Instinct, error) {
	return nil, u.err()
}
func (u UnavailableInstinctGlobal) Sanitize(context.Context, func(string) string) (int, error) {
	return 0, u.err()
}

func (s *InstinctGlobalStore) SanitizeCandidates(ctx context.Context, project string) ([]domain.SanitizeCandidate, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug,description FROM instinct_global`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.SanitizeCandidate{}
	for rows.Next() {
		var slug, text string
		if err := rows.Scan(&slug, &text); err != nil {
			return nil, err
		}
		if domain.NeedsSanitize(text) {
			out = append(out, domain.SanitizeCandidate{Category: domain.SanitizeCategory("instinct_global"), RecordID: slug, Project: project, Field: "description", Text: text})
		}
		if domain.ProtectObservationKey(slug) != slug {
			out = append(out, domain.SanitizeCandidate{Category: domain.SanitizeCategory("instinct_global_slug"), RecordID: slug, Project: project, Field: "slug", Text: slug})
		}
	}
	return out, rows.Err()
}
func (s *InstinctGlobalStore) RecordProject(ctx context.Context, project string, i domain.Instinct, now time.Time) error {
	if i.Confidence+1e-9 < .5 || i.Status == "rejected" {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var raw string
	err = tx.QueryRowContext(ctx, `SELECT projects FROM instinct_global WHERE slug=?`, i.Slug).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	projects := []string{}
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &projects); err != nil {
			return fmt.Errorf("decode instinct projects: %w", err)
		}
	}
	if !slices.Contains(projects, project) {
		projects = append(projects, project)
	}
	slices.Sort(projects)
	encoded, err := json.Marshal(projects)
	if err != nil {
		return err
	}
	desc := domain.TruncateRunes(domain.StripTerminalControls(domain.RedactSecrets(domain.RedactPrivate(i.Description))), 200)
	_, err = tx.ExecContext(ctx, `INSERT INTO instinct_global(slug,kind,description,projects,confidence,updated_at) VALUES(?,?,?,?,?,?) ON CONFLICT(slug) DO UPDATE SET projects=excluded.projects,confidence=MAX(instinct_global.confidence,excluded.confidence),description=excluded.description,updated_at=excluded.updated_at`, i.Slug, i.Kind, desc, string(encoded), i.Confidence, now.UTC().Format(time.RFC3339Nano))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *InstinctGlobalStore) GlobalFor(ctx context.Context, minConfidence float64) ([]domain.Instinct, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug,kind,description,projects,confidence FROM instinct_global WHERE confidence>=? ORDER BY confidence DESC,slug`, minConfidence-1e-9)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Instinct{}
	for rows.Next() {
		var i domain.Instinct
		var raw string
		if err := rows.Scan(&i.Slug, &i.Kind, &i.Description, &raw, &i.Confidence); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &i.Projects); err != nil {
			return nil, err
		}
		if len(i.Projects) < 2 {
			continue
		}
		i.Slug = domain.ProtectObservationKey(i.Slug)
		i.Description = domain.StripTerminalControls(domain.RedactSecrets(domain.RedactPrivate(i.Description)))
		i.Scope = "global"
		i.Status = "proposed"
		out = append(out, i)
	}
	return out, rows.Err()
}
func (s *InstinctGlobalStore) Sanitize(ctx context.Context, redact func(string) string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.QueryContext(ctx, `SELECT slug,description FROM instinct_global`)
	if err != nil {
		return 0, err
	}
	changes := map[string]string{}
	var slugs []string
	for rows.Next() {
		var slug, text string
		if err := rows.Scan(&slug, &text); err != nil {
			_ = rows.Close()
			return 0, err
		}
		if clean := redact(text); clean != text {
			changes[slug] = clean
		}
		if domain.ProtectObservationKey(slug) != slug {
			slugs = append(slugs, slug)
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return 0, err
	}
	for slug, text := range changes {
		if _, err := tx.ExecContext(ctx, `UPDATE instinct_global SET description=? WHERE slug=?`, text, slug); err != nil {
			return 0, err
		}
	}
	changed := len(changes)
	for _, slug := range slugs {
		if err := renameGlobalSlug(ctx, tx, slug, domain.ProtectObservationKey(slug)); err != nil {
			return 0, err
		}
		if _, counted := changes[slug]; !counted {
			changed++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	// El checkpoint se ejecuta siempre, aunque esta pasada no cambie nada:
	// si la anterior lo encontró ocupado, el texto antiguo sigue en el WAL y
	// repetir mem sanitize --apply debe terminar de retirarlo (A-001).
	var busy, logFrames, checkpointed int
	if err := s.db.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
		return changed, classifySQLite(err)
	}
	if busy != 0 {
		return changed, domain.NewSanitizeError(domain.SanitizeCodeBusy, errors.New("checkpoint de instincts.db bloqueado por otra conexión"))
	}
	return changed, nil
}

// renameGlobalSlug sustituye un slug global con credencial por su forma
// protegida; si ya existe, se fusionan: unión de proyectos y confianza
// máxima, conservando la descripción de la fila destino (B-001).
func renameGlobalSlug(ctx context.Context, tx *sql.Tx, from, to string) error {
	var rawFrom string
	var confFrom float64
	err := tx.QueryRowContext(ctx, `SELECT projects,confidence FROM instinct_global WHERE slug=?`, from).Scan(&rawFrom, &confFrom)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var rawTo string
	err = tx.QueryRowContext(ctx, `SELECT projects FROM instinct_global WHERE slug=?`, to).Scan(&rawTo)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = tx.ExecContext(ctx, `UPDATE instinct_global SET slug=? WHERE slug=?`, to, from)
		return err
	}
	if err != nil {
		return err
	}
	var a, b []string
	if err := json.Unmarshal([]byte(rawFrom), &a); err != nil {
		return fmt.Errorf("decode instinct projects: %w", err)
	}
	if err := json.Unmarshal([]byte(rawTo), &b); err != nil {
		return fmt.Errorf("decode instinct projects: %w", err)
	}
	merged := append(b, a...)
	slices.Sort(merged)
	encoded, err := json.Marshal(slices.Compact(merged))
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE instinct_global SET projects=?,confidence=MAX(confidence,?) WHERE slug=?`, string(encoded), confFrom, to); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM instinct_global WHERE slug=?`, from)
	return err
}
