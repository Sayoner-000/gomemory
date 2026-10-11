package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

// Marcadores sintéticos de la feature 037 (H-01/H-02). Ninguno es un secreto
// real: la credencial es la clave de ejemplo de la documentación de AWS.
const (
	privMarker  = "SECRETO_037"
	credMarker  = "AKIAIOSFODNN7EXAMPLE"
	publicPart  = "texto público útil"
	protectedIn = publicPart + " <private>" + privMarker + " <PRIVATE>anidado</private> fin</private> clave " + credMarker + " <private>" + privMarker + " sin cierre"
)

// assertProtected falla si got revela un marcador protegido o pierde el
// texto público.
func assertProtected(t *testing.T, where, got string) {
	t.Helper()
	for _, gone := range []string{privMarker, credMarker, "anidado"} {
		if strings.Contains(got, gone) {
			t.Fatalf("%s revela %q: %q", where, gone, got)
		}
	}
	if !strings.Contains(got, publicPart) {
		t.Fatalf("%s perdió el texto público: %q", where, got)
	}
}

// rawColumn lee una columna sin pasar por los lectores del repositorio, para
// comprobar qué se persistió de verdad.
func rawColumn(t *testing.T, db *sql.DB, query string, args ...any) string {
	t.Helper()
	var v sql.NullString
	if err := db.QueryRow(query, args...).Scan(&v); err != nil {
		t.Fatalf("leer %q: %v", query, err)
	}
	return v.String
}

// TestSessionSummary_ProtegeEscrituras cubre US1.1 (FR-001): EndSession y
// UpdateSessionSummary nunca persisten contenido protegido.
func TestSessionSummary_ProtegeEscrituras(t *testing.T) {
	db := openTestDB(t)

	open, err := StartSession(db, "proj")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := UpdateSessionSummary(db, open.ID, protectedIn); err != nil {
		t.Fatalf("UpdateSessionSummary: %v", err)
	}
	assertProtected(t, "summary tras UpdateSessionSummary",
		rawColumn(t, db, `SELECT summary FROM sessions WHERE id = ?`, open.ID))

	if err := EndSession(db, open.ID, protectedIn); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	assertProtected(t, "summary tras EndSession",
		rawColumn(t, db, `SELECT summary FROM sessions WHERE id = ?`, open.ID))
}

// TestSessionSummary_ProtegeLecturasHistoricas cubre US1.7 (FR-002): una fila
// anterior al cambio, guardada en crudo, se emite depurada sin reescribirse
// y sin alterar identidad ni fechas.
func TestSessionSummary_ProtegeLecturasHistoricas(t *testing.T) {
	db := openTestDB(t)

	if _, err := db.Exec(`INSERT INTO sessions (id, project, summary, created_at) VALUES ('hist-1', 'proj', ?, '2026-01-02 03:04:05')`, protectedIn); err != nil {
		t.Fatalf("fixture: %v", err)
	}

	active, err := ActiveSession(db, "proj")
	if err != nil || active == nil {
		t.Fatalf("ActiveSession: %v %v", active, err)
	}
	assertProtected(t, "ActiveSession", active.Summary)
	if active.ID != "hist-1" || active.CreatedAt != "2026-01-02 03:04:05" {
		t.Fatalf("identidad o fecha alteradas: %+v", active)
	}

	recent, err := RecentSessions(db, "proj", 10)
	if err != nil || len(recent) != 1 {
		t.Fatalf("RecentSessions: %v %v", recent, err)
	}
	assertProtected(t, "RecentSessions", recent[0].Summary)

	if raw := rawColumn(t, db, `SELECT summary FROM sessions WHERE id = 'hist-1'`); raw != protectedIn {
		t.Fatalf("la lectura no debe persistir cambios: %q", raw)
	}
}

// TestRelationReasoning_ProtegeEscriturasYLecturas cubre US1.2 (FR-001,
// FR-002): crear, actualizar, importar y leer relaciones.
func TestRelationReasoning_ProtegeEscriturasYLecturas(t *testing.T) {
	db := openTestDB(t)

	id, err := InsertRelation(db, &domain.Relation{Project: "proj", MemoryIDA: 1, MemoryIDB: 2, Relation: domain.Related, Confidence: 0.7, Reasoning: protectedIn})
	if err != nil {
		t.Fatalf("InsertRelation: %v", err)
	}
	assertProtected(t, "reasoning tras InsertRelation",
		rawColumn(t, db, `SELECT reasoning FROM memory_relations WHERE id = ?`, id))

	if err := UpdateRelation(db, id, domain.Related, 0.8, protectedIn); err != nil {
		t.Fatalf("UpdateRelation: %v", err)
	}
	assertProtected(t, "reasoning tras UpdateRelation",
		rawColumn(t, db, `SELECT reasoning FROM memory_relations WHERE id = ?`, id))

	imp, err := ImportRelation(db, &domain.Relation{Project: "proj", MemoryIDA: 3, MemoryIDB: 4, Relation: domain.Related, Confidence: 0.6, Reasoning: protectedIn, CreatedAt: "2025-05-06 07:08:09"})
	if err != nil {
		t.Fatalf("ImportRelation: %v", err)
	}
	assertProtected(t, "reasoning tras ImportRelation",
		rawColumn(t, db, `SELECT reasoning FROM memory_relations WHERE id = ?`, imp))
	if got := rawColumn(t, db, `SELECT created_at FROM memory_relations WHERE id = ?`, imp); got != "2025-05-06 07:08:09" {
		t.Fatalf("import alteró created_at: %q", got)
	}

	// Fila histórica cruda: todos los lectores la depuran.
	res, err := db.Exec(`INSERT INTO memory_relations (project, memory_id_a, memory_id_b, relation, confidence, reasoning, created_at) VALUES ('proj', 5, 6, ?, 0.9, ?, '2024-01-01 00:00:00')`, string(domain.Related), protectedIn)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	hist, _ := res.LastInsertId()

	r, err := GetRelation(db, "proj", hist)
	if err != nil || r == nil {
		t.Fatalf("GetRelation: %v %v", r, err)
	}
	assertProtected(t, "GetRelation", r.Reasoning)
	if r.Confidence != 0.9 || r.CreatedAt != "2024-01-01 00:00:00" || r.MemoryIDA != 5 || r.MemoryIDB != 6 {
		t.Fatalf("metadatos alterados: %+v", r)
	}
	pair, err := GetRelationByPair(db, "proj", 6, 5)
	if err != nil || pair == nil {
		t.Fatalf("GetRelationByPair: %v %v", pair, err)
	}
	assertProtected(t, "GetRelationByPair", pair.Reasoning)
	for name, list := range map[string]func() ([]domain.Relation, error){
		"ListRelations":    func() ([]domain.Relation, error) { return ListRelations(db, "proj", 50) },
		"ListAllRelations": func() ([]domain.Relation, error) { return ListAllRelations(db, "proj") },
	} {
		rels, err := list()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, r := range rels {
			assertProtected(t, name, r.Reasoning)
		}
	}
}

// TestChannelError_ProtegeDiagnostico cubre US1.3 (FR-005): el diagnóstico
// conserva una causa útil sin revelar contenido protegido, también en filas
// históricas.
func TestChannelError_ProtegeDiagnostico(t *testing.T) {
	db := openTestDB(t)

	if err := RecordChannelError(db, "proj", "opencode", "user", "turn_reminder", protectedIn); err != nil {
		t.Fatalf("RecordChannelError: %v", err)
	}
	assertProtected(t, "last_error persistido",
		rawColumn(t, db, `SELECT last_error FROM channel_activity WHERE project = 'proj' AND kind = 'turn_reminder'`))

	if _, err := db.Exec(`INSERT INTO channel_activity (project, agent, scope, kind, last_error, last_error_at) VALUES ('proj', 'claude', 'user', 'hist', ?, '2025-01-01 00:00:00')`, protectedIn); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	got, ok := LastChannelActivity(db, "proj", "claude", "user", "hist")
	if !ok {
		t.Fatal("LastChannelActivity sin datos")
	}
	assertProtected(t, "LastChannelActivity", got.LastError)
}

// TestOriginalStore_RechazaContenidoProtegido cubre US1.5 (FR-004): Put no
// guarda un original con región privada (completa o abierta) ni credencial;
// Get de un original histórico protegido falla sin devolver bytes; un
// original público se sigue recuperando exacto.
func TestOriginalStore_RechazaContenidoProtegido(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	store := NewOriginalStoreRepository(db, &fakeClock{now: time.Date(2026, 10, 11, 9, 0, 0, 0, time.UTC)}, 7, 0)

	for _, in := range []string{
		"salida <private>" + privMarker + "</private> pública",
		"salida <Private>" + privMarker + " sin cierre",
		"salida con " + credMarker,
	} {
		ref, err := store.Put(ctx, "proj", in, "log", "log")
		if !errors.Is(err, domain.ErrProtectedContent) || ref != "" {
			t.Fatalf("Put(%q) = %q, %v; quiero ErrProtectedContent", in, ref, err)
		}
	}
	if _, n, _ := store.Usage(ctx); n != 0 {
		t.Fatalf("Put protegido dejó %d filas", n)
	}

	// Original histórico protegido insertado por una versión anterior.
	hist, err := gzipBytes("hist <private>" + privMarker + "</private>")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO compression_originals (ref, hash, project, content_gz, raw_bytes, content_type, compressor, created_at, last_access_at, expires_at)
		VALUES ('histref00001', 'h', 'proj', ?, 10, 'log', 'log', '2026-10-01 00:00:00', '2026-10-01 00:00:00', '2099-01-01 00:00:00')`, hist); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	got, _, ok, err := store.Get(ctx, "histref00001")
	if !errors.Is(err, domain.ErrProtectedContent) || ok || got != "" {
		t.Fatalf("Get histórico protegido = %q ok=%v err=%v", got, ok, err)
	}
	if err != nil && strings.Contains(err.Error(), privMarker) {
		t.Fatalf("el error revela contenido: %v", err)
	}

	public := "línea pública 1\nlínea pública 2 con <privat> que no es etiqueta\n"
	ref, err := store.Put(ctx, "proj", public, "log", "log")
	if err != nil {
		t.Fatalf("Put público: %v", err)
	}
	if got, _, ok, err := store.Get(ctx, ref); err != nil || !ok || got != public {
		t.Fatalf("original público no exacto: %q ok=%v err=%v", got, ok, err)
	}
}
