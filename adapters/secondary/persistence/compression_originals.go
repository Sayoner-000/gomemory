package persistence

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// tsLayout es de ancho fijo y en UTC para que las marcas de tiempo se ordenen
// lexicográficamente igual que cronológicamente (TTL y LRU se resuelven en SQL).
const tsLayout = "2006-01-02T15:04:05.000000000Z"

func formatTS(t time.Time) string { return t.UTC().Format(tsLayout) }

func parseTS(s string) time.Time {
	t, _ := time.Parse(tsLayout, s)
	return t
}

// OriginalStoreRepository implementa ports.OriginalStoreRepository sobre la
// tabla compression_originals (feature 033, research.md R9): gzip, dedup por
// sha256, caducidad desde el último acceso y tope de espacio con expulsión LRU.
type OriginalStoreRepository struct {
	db       *sql.DB
	clock    ports.ClockPort
	ttl      time.Duration
	maxBytes int64
}

// NewOriginalStoreRepository construye el almacén. ttlDays y maxMB <= 0 toman
// los valores de fábrica de domain/compression_policy.go.
func NewOriginalStoreRepository(db *sql.DB, clock ports.ClockPort, ttlDays, maxMB int) *OriginalStoreRepository {
	ttl := time.Duration(domain.CompressionOriginalsTTLDays) * 24 * time.Hour
	if ttlDays > 0 {
		ttl = time.Duration(ttlDays) * 24 * time.Hour
	}
	maxBytes := int64(domain.CompressionOriginalsMaxBytes)
	if maxMB > 0 {
		maxBytes = int64(maxMB) << 20
	}
	return &OriginalStoreRepository{db: db, clock: clock, ttl: ttl, maxBytes: maxBytes}
}

func (r *OriginalStoreRepository) Put(ctx context.Context, project, content, contentType, compressor string) (string, error) {
	sum := sha256.Sum256([]byte(content))
	hash := hex.EncodeToString(sum[:])
	now := r.clock.Now()

	// Reutiliza la fila si el mismo contenido ya estaba guardado (dedup).
	var existing string
	err := r.db.QueryRowContext(ctx, `SELECT ref FROM compression_originals WHERE hash = ?`, hash).Scan(&existing)
	switch {
	case err == nil:
		_, err = r.db.ExecContext(ctx,
			`UPDATE compression_originals SET last_access_at = ?, expires_at = ? WHERE ref = ?`,
			formatTS(now), formatTS(now.Add(r.ttl)), existing)
		if err != nil {
			return "", fmt.Errorf("renovar original: %w", err)
		}
		return existing, nil
	case !errors.Is(err, sql.ErrNoRows):
		return "", fmt.Errorf("buscar original: %w", err)
	}

	ref, err := r.freeRef(ctx, hash)
	if err != nil {
		return "", err
	}
	gz, err := gzipBytes(content)
	if err != nil {
		return "", err
	}
	// A single original that cannot fit would be inserted and immediately
	// evicted by Purge, leaving the caller with an unrecoverable reference.
	if int64(len(gz)) > r.maxBytes {
		return "", fmt.Errorf("original comprimido excede el tope del almacén: %d > %d bytes", len(gz), r.maxBytes)
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO compression_originals (ref, hash, project, content_gz, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ref, hash, project, gz, len(content), contentType, compressor, formatTS(now), formatTS(now), formatTS(now.Add(r.ttl)))
	if err != nil {
		return "", fmt.Errorf("guardar original: %w", err)
	}
	if _, _, err := r.purge(ctx, ref); err != nil {
		return "", err
	}
	return ref, nil
}

// freeRef devuelve el prefijo corto de la huella, o el largo si el corto ya
// pertenece a otro contenido (colisión).
func (r *OriginalStoreRepository) freeRef(ctx context.Context, hash string) (string, error) {
	for _, n := range []int{domain.RefLen, domain.RefLenCollision, len(hash)} {
		ref := domain.RefFromHash(hash, n)
		var taken int
		if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM compression_originals WHERE ref = ?`, ref).Scan(&taken); err != nil {
			return "", fmt.Errorf("comprobar ref: %w", err)
		}
		if taken == 0 {
			return ref, nil
		}
	}
	return "", fmt.Errorf("sin ref libre para %s", hash)
}

func (r *OriginalStoreRepository) Get(ctx context.Context, ref string) (string, ports.OriginalMeta, bool, error) {
	now := r.clock.Now()
	var (
		gz                                        []byte
		meta                                      ports.OriginalMeta
		created, lastAccess, expires, contentType string
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT content_gz, project, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at
		 FROM compression_originals WHERE ref = ? AND expires_at > ?`, ref, formatTS(now)).
		Scan(&gz, &meta.Project, &meta.RawBytes, &contentType, &meta.Compressor, &created, &lastAccess, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ports.OriginalMeta{}, false, nil
	}
	if err != nil {
		return "", ports.OriginalMeta{}, false, fmt.Errorf("leer original: %w", err)
	}
	content, err := gunzipBytes(gz)
	if err != nil {
		return "", ports.OriginalMeta{}, false, err
	}
	if _, err := r.db.ExecContext(ctx,
		`UPDATE compression_originals SET last_access_at = ?, expires_at = ? WHERE ref = ?`,
		formatTS(now), formatTS(now.Add(r.ttl)), ref); err != nil {
		return "", ports.OriginalMeta{}, false, fmt.Errorf("renovar original: %w", err)
	}
	meta.Ref = ref
	meta.ContentType = contentType
	meta.CreatedAt = parseTS(created)
	meta.LastAccessAt = now
	meta.ExpiresAt = now.Add(r.ttl)
	return content, meta, true, nil
}

// Purge borra los caducados y, si el almacén supera el tope, los de acceso más
// antiguo hasta volver a caber.
func (r *OriginalStoreRepository) Purge(ctx context.Context) (int, int64, error) {
	return r.purge(ctx, "")
}

// purge conserva keepRef mientras aplica el tope. Put lo usa para garantizar
// que una referencia que acaba de devolver siga siendo recuperable.
func (r *OriginalStoreRepository) purge(ctx context.Context, keepRef string) (int, int64, error) {
	now := formatTS(r.clock.Now())
	var freed int64
	_ = r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(content_gz)), 0) FROM compression_originals WHERE expires_at <= ?`, now).Scan(&freed)
	res, err := r.db.ExecContext(ctx, `DELETE FROM compression_originals WHERE expires_at <= ?`, now)
	if err != nil {
		return 0, 0, fmt.Errorf("purgar caducados: %w", err)
	}
	n64, _ := res.RowsAffected()
	removed := int(n64)

	total, _, err := r.Usage(ctx)
	if err != nil {
		return removed, freed, err
	}
	for total > r.maxBytes {
		var ref string
		var size int64
		err := r.db.QueryRowContext(ctx,
			`SELECT ref, length(content_gz) FROM compression_originals WHERE ref <> ? ORDER BY last_access_at ASC, ref ASC LIMIT 1`, keepRef).Scan(&ref, &size)
		if errors.Is(err, sql.ErrNoRows) {
			break
		}
		if err != nil {
			return removed, freed, fmt.Errorf("elegir original a expulsar: %w", err)
		}
		if _, err := r.db.ExecContext(ctx, `DELETE FROM compression_originals WHERE ref = ?`, ref); err != nil {
			return removed, freed, fmt.Errorf("expulsar original: %w", err)
		}
		removed++
		freed += size
		total -= size
	}
	return removed, freed, nil
}

func (r *OriginalStoreRepository) Usage(ctx context.Context) (int64, int, error) {
	var bytes int64
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(content_gz)), 0), COUNT(*) FROM compression_originals`).Scan(&bytes, &count)
	if err != nil {
		return 0, 0, fmt.Errorf("uso del almacén de originales: %w", err)
	}
	return bytes, count, nil
}

func gzipBytes(s string) ([]byte, error) {
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := io.WriteString(w, s); err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("gzip: %w", err)
	}
	return buf.Bytes(), nil
}

func gunzipBytes(b []byte) (string, error) {
	r, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return "", fmt.Errorf("gunzip: %w", err)
	}
	defer func() { _ = r.Close() }()
	out, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("gunzip: %w", err)
	}
	return string(out), nil
}
