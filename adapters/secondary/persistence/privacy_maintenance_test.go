package persistence

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"mem/application/ports"
	"mem/domain"
)

// seedHistorical037 inserta filas anteriores a la feature 037, guardadas en
// crudo, más filas limpias y de otro proyecto que no deben aparecer.
func seedHistorical037(t *testing.T, db *sql.DB) {
	t.Helper()
	gz, err := gzipBytes("salida " + protectedIn)
	if err != nil {
		t.Fatal(err)
	}
	pub, err := gzipBytes("salida pública")
	if err != nil {
		t.Fatal(err)
	}
	stmts := []struct {
		q    string
		args []any
	}{
		{`INSERT INTO sessions (id, project, summary, created_at, ended_at) VALUES ('s-hist', 'proj', ?, '2025-01-01 00:00:00', '2025-01-01 01:00:00')`, []any{protectedIn}},
		{`INSERT INTO sessions (id, project, summary, created_at) VALUES ('s-limpia', 'proj', 'resumen público', '2025-01-02 00:00:00')`, nil},
		{`INSERT INTO sessions (id, project, summary, created_at) VALUES ('s-otro', 'otro', ?, '2025-01-03 00:00:00')`, []any{protectedIn}},
		{`INSERT INTO memory_relations (id, project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at) VALUES (41, 'proj', 1, 2, 'related', 0.75, ?, '2024-05-05 05:05:05')`, []any{protectedIn}},
		{`INSERT INTO channel_activity (project, agent, scope, kind, last_error, last_error_at) VALUES ('proj', 'opencode', 'user', 'turn/reminder', ?, '2025-02-02 00:00:00')`, []any{protectedIn}},
		{`INSERT INTO compression_originals (ref, hash, project, content_gz, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at) VALUES ('privref', 'h1', 'proj', ?, 1, 'log', 'log', 'x', 'x', '2099-01-01')`, []any{gz}},
		{`INSERT INTO compression_originals (ref, hash, project, content_gz, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at) VALUES ('pubref', 'h2', 'proj', ?, 1, 'log', 'log', 'x', 'x', '2099-01-01')`, []any{pub}},
		{`INSERT INTO memories (id, project, type, title, content) VALUES (9, 'proj', 'decision', 'remanente', 'texto Z</private> fin')`, nil},
		{`INSERT INTO memories (id, project, type, title, content) VALUES (10, 'proj', 'decision', 'limpia', 'contenido público')`, nil},
	}
	for _, s := range stmts {
		if _, err := db.Exec(s.q, s.args...); err != nil {
			t.Fatalf("seed %q: %v", s.q, err)
		}
	}
}

func candidateIDs(cands []domain.SanitizeCandidate) []string {
	var out []string
	for _, c := range cands {
		out = append(out, string(c.Category)+":"+c.RecordID)
	}
	sort.Strings(out)
	return out
}

// TestPrivacyStore_CandidatosDesdeTextoCrudo cubre FR-006: los candidatos
// salen del texto original, acotados al proyecto y a la lista permitida.
func TestPrivacyStore_CandidatosDesdeTextoCrudo(t *testing.T) {
	db := openTestDB(t)
	seedHistorical037(t, db)
	store := NewPrivacyMaintenanceStore(db)

	cands, err := store.Candidates(context.Background(), "proj")
	if err != nil {
		t.Fatalf("Candidates: %v", err)
	}
	want := []string{
		"channel_error:opencode/user/turn%2Freminder",
		"compression_original:privref",
		"memory_review:9",
		"relation:41",
		"session:s-hist",
	}
	if got := candidateIDs(cands); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("candidatos = %v, quiero %v", got, want)
	}
	for _, c := range cands {
		if err := c.Validate("proj"); err != nil {
			t.Errorf("candidato inválido %+v: %v", c, err)
		}
	}
}

// TestPrivacyStore_AplicaSoloElCampoPermitido cubre FR-007: solo cambia el
// texto; identidad, fechas y confianza se conservan; un valor que cambió
// mientras tanto no se pisa; el original protegido se elimina.
func TestPrivacyStore_AplicaSoloElCampoPermitido(t *testing.T) {
	db := openTestDB(t)
	seedHistorical037(t, db)
	store := NewPrivacyMaintenanceStore(db)
	ctx := context.Background()

	changed := 0
	err := store.WithTransaction(ctx, "proj", func(tx ports.PrivacyTransaction) error {
		cands, err := tx.Candidates(ctx, "proj")
		if err != nil {
			return err
		}
		for _, c := range cands {
			if !c.Writable() {
				if _, err := tx.Apply(ctx, c, "x"); err == nil {
					t.Error("memory_review no debe ser escribible")
				}
				continue
			}
			ok, err := tx.Apply(ctx, c, domain.ProtectText(c.Text))
			if err != nil {
				return err
			}
			if ok {
				changed++
			}
		}
		stale := domain.SanitizeCandidate{Category: domain.SanitizeSession, RecordID: "s-limpia", Project: "proj", Field: "summary", Text: "otro valor"}
		if ok, err := tx.Apply(ctx, stale, "pisado"); err != nil || ok {
			t.Errorf("valor desactualizado aplicado: ok=%v err=%v", ok, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}
	if changed != 4 {
		t.Fatalf("filas cambiadas = %d, quiero 4", changed)
	}

	assertProtected(t, "summary", rawColumn(t, db, `SELECT summary FROM sessions WHERE id = 's-hist'`))
	assertProtected(t, "reasoning", rawColumn(t, db, `SELECT reasoning FROM memory_relations WHERE id = 41`))
	assertProtected(t, "last_error", rawColumn(t, db, `SELECT last_error FROM channel_activity WHERE kind = 'turn/reminder'`))
	if got := rawColumn(t, db, `SELECT created_at || '|' || ended_at FROM sessions WHERE id = 's-hist'`); got != "2025-01-01 00:00:00|2025-01-01 01:00:00" {
		t.Fatalf("fechas de sesión alteradas: %q", got)
	}
	if got := rawColumn(t, db, `SELECT confidence || '|' || created_at FROM memory_relations WHERE id = 41`); got != "0.75|2024-05-05 05:05:05" {
		t.Fatalf("metadatos de relación alterados: %q", got)
	}
	if got := rawColumn(t, db, `SELECT summary FROM sessions WHERE id = 's-limpia'`); got != "resumen público" {
		t.Fatalf("fila limpia pisada: %q", got)
	}
	if got := rawColumn(t, db, `SELECT summary FROM sessions WHERE id = 's-otro'`); got != protectedIn {
		t.Fatal("se modificó otro proyecto")
	}
	var n int
	_ = db.QueryRow(`SELECT COUNT(*) FROM compression_originals WHERE ref IN ('privref','pubref')`).Scan(&n)
	if n != 1 {
		t.Fatalf("originales restantes = %d, quiero solo el público", n)
	}
	if got := rawColumn(t, db, `SELECT content FROM memories WHERE id = 9`); got != "texto Z</private> fin" {
		t.Fatal("memory_review se modificó")
	}
	if cands, _ := store.Candidates(ctx, "proj"); len(cands) != 1 || cands[0].Category != domain.SanitizeMemoryReview {
		t.Fatalf("tras aplicar deben quedar solo revisiones: %v", candidateIDs(cands))
	}
}

// TestPrivacyStore_RollbackAnteError: un error del callback no confirma nada.
func TestPrivacyStore_RollbackAnteError(t *testing.T) {
	db := openTestDB(t)
	seedHistorical037(t, db)
	store := NewPrivacyMaintenanceStore(db)
	ctx := context.Background()
	boom := errors.New("falla a mitad")
	err := store.WithTransaction(ctx, "proj", func(tx ports.PrivacyTransaction) error {
		cands, _ := tx.Candidates(ctx, "proj")
		for _, c := range cands {
			if c.Writable() {
				if _, err := tx.Apply(ctx, c, domain.ProtectText(c.Text)); err != nil {
					return err
				}
			}
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("WithTransaction = %v, quiero el error del callback", err)
	}
	if got := rawColumn(t, db, `SELECT summary FROM sessions WHERE id = 's-hist'`); got != protectedIn {
		t.Fatal("el rollback no deshizo el cambio")
	}
}

// TestPrivacyStore_EscritorSimultaneoEsBusy cubre US2.4: con otra conexión
// reteniendo el bloqueo de escritura, el saneamiento falla con BUSY sin
// cambios ni corrupción.
func TestPrivacyStore_EscritorSimultaneoEsBusy(t *testing.T) {
	root := t.TempDir()
	db, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	seedHistorical037(t, db)
	other, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = other.Close() }()
	writer, err := other.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Exec(`UPDATE sessions SET summary = summary WHERE id = 's-limpia'`); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err = NewPrivacyMaintenanceStore(db).WithTransaction(ctx, "proj", func(ports.PrivacyTransaction) error { return nil })
	var se *domain.SanitizeError
	if !errors.As(err, &se) || se.Code != domain.SanitizeCodeBusy {
		t.Fatalf("quiero SANITIZE_BUSY, got %v", err)
	}
	_ = writer.Rollback()
	var ok string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&ok); err != nil || ok != "ok" {
		t.Fatalf("integridad: %q %v", ok, err)
	}
}

// TestPrivacyStore_CompactVerificaCheckpoint cubre FR-008: sin clientes
// abiertos el WAL queda vacío sin borrar archivos; con un lector abierto el
// checkpoint no completa y se informa BUSY.
func TestPrivacyStore_CompactVerificaCheckpoint(t *testing.T) {
	root := t.TempDir()
	db, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	seedHistorical037(t, db)
	store := NewPrivacyMaintenanceStore(db)
	ctx := context.Background()

	other, err := Init(root)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := other.Begin()
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := reader.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	err = store.Compact(ctx)
	var se *domain.SanitizeError
	if !errors.As(err, &se) || se.Code != domain.SanitizeCodeBusy {
		t.Fatalf("con lector abierto quiero SANITIZE_BUSY, got %v", err)
	}
	_ = reader.Rollback()
	_ = other.Close()

	if err := store.Compact(ctx); err != nil {
		t.Fatalf("Compact sin clientes: %v", err)
	}
	wal := DbPath(root) + "-wal"
	info, err := os.Stat(wal)
	if err != nil {
		t.Fatalf("el WAL no debe borrarse manualmente: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("WAL tras checkpoint TRUNCATE = %d bytes", info.Size())
	}
	if _, err := os.Stat(filepath.Dir(wal)); err != nil {
		t.Fatal(err)
	}
}
