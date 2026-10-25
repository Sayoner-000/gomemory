package persistence

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"mem/domain"
)

// socraticaColumn describes one column as pragma_table_info reports it.
type socraticaColumn struct {
	typ, def string
	notnull  bool
}

func tableColumns(t *testing.T, db *sql.DB, table string) map[string]socraticaColumn {
	t.Helper()
	rows, err := db.Query("SELECT name, type, [notnull], COALESCE(dflt_value, '') FROM pragma_table_info(?)", table)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols := map[string]socraticaColumn{}
	for rows.Next() {
		var name string
		var c socraticaColumn
		if err := rows.Scan(&name, &c.typ, &c.notnull, &c.def); err != nil {
			t.Fatal(err)
		}
		cols[name] = c
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return cols
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func requireSpecifyColumns(t *testing.T, db *sql.DB) {
	t.Helper()
	flows := tableColumns(t, db, "socratica_flows")
	if c, ok := flows["preparation_json"]; !ok || c.notnull || c.typ != "TEXT" {
		t.Errorf("socratica_flows.preparation_json = %+v (present %v), want nullable TEXT", c, ok)
	}
	if c, ok := flows["payload_policy"]; !ok || !c.notnull || c.def != "'persistent'" {
		t.Errorf("socratica_flows.payload_policy = %+v (present %v), want NOT NULL DEFAULT 'persistent'", c, ok)
	}
	if c, ok := tableColumns(t, db, "socratica_history")["preparation_json"]; !ok || c.notnull {
		t.Errorf("socratica_history.preparation_json = %+v (present %v), want nullable", c, ok)
	}
}

// catalogV2Fixture builds a database at catalog version 2: the current schema
// without the 042 columns, with one refine flow and its history row. Columns
// are dropped from a fresh catalog so the rest matches v2 exactly.
func catalogV2Fixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mem.db")
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		"ALTER TABLE socratica_flows DROP COLUMN preparation_json",
		"ALTER TABLE socratica_flows DROP COLUMN payload_policy",
		"ALTER TABLE socratica_history DROP COLUMN preparation_json",
		"PRAGMA user_version = 2",
		`INSERT INTO socratica_flows(id,schema_version,project,client,mode,original_prompt,phase,last_stable_phase,created_at,updated_at,proposed_prompt,scrutiny_json)
		 VALUES('legacy',1,'p','claude','refine','original legado','review','review','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z','propuesta legada','null')`,
		`INSERT INTO socratica_history(id,flow_id,project,client,mode,original_prompt,proposed_prompt,words_before,words_after,completed_at)
		 VALUES('legacy','legacy','p','claude','refine','original legado','propuesta legada',2,2,'2026-01-01T00:00:00Z')`,
	} {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSpecifyMigration_CatalogIsVersion3(t *testing.T) {
	if schemaVersion != 3 {
		t.Fatalf("schemaVersion = %d, want 3 for feature 042", schemaVersion)
	}
}

func TestSpecifyMigration_EmptyBase(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v := userVersion(t, db); v != 3 {
		t.Errorf("user_version = %d, want 3", v)
	}
	requireSpecifyColumns(t, db)
}

func TestSpecifyMigration_FromCatalogV2KeepsLegacyRows(t *testing.T) {
	path := catalogV2Fixture(t)
	db, err := openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v := userVersion(t, db); v != 3 {
		t.Errorf("user_version = %d, want 3", v)
	}
	requireSpecifyColumns(t, db)
	var prep sql.NullString
	var policy string
	if err := db.QueryRow("SELECT preparation_json, payload_policy FROM socratica_flows WHERE id='legacy'").Scan(&prep, &policy); err != nil {
		t.Fatal(err)
	}
	if prep.Valid || policy != "persistent" {
		t.Errorf("legacy row: preparation=%v policy=%q, want NULL and persistent", prep, policy)
	}
	store := NewSocraticaStore(db, time.Second)
	f, err := store.GetFlow(context.Background(), "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if f.Preparation != nil || f.EffectivePayloadPolicy() != domain.PayloadPersistent || f.ProposedPrompt != "propuesta legada" {
		t.Errorf("legacy flow = %+v, want no preparation, persistent, proposal kept", f)
	}
	history, err := store.ListHistory(context.Background(), "p", "claude", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Preparation != nil {
		t.Errorf("legacy history = %+v, want one row without preparation", history)
	}
}

// A failure after the 042 columns were added must roll them back with the
// rest of the migration and leave the v2 catalog and its rows untouched.
func TestSpecifyMigration_FailureRollsBackToV2(t *testing.T) {
	path := catalogV2Fixture(t)
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("DROP TABLE memories; CREATE VIEW memories AS SELECT 1 AS id"); err != nil {
		t.Fatal(err)
	}
	if err := raw.Close(); err != nil {
		t.Fatal(err)
	}
	if db, err := openDBFile(path); err == nil {
		db.Close()
		t.Fatal("expected the migration over a view to fail")
	}
	raw, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if v := userVersion(t, raw); v != 2 {
		t.Errorf("user_version = %d after rollback, want 2", v)
	}
	if _, ok := tableColumns(t, raw, "socratica_flows")["preparation_json"]; ok {
		t.Error("rollback kept socratica_flows.preparation_json")
	}
	var original string
	if err := raw.QueryRow("SELECT original_prompt FROM socratica_flows WHERE id='legacy'").Scan(&original); err != nil || original != "original legado" {
		t.Errorf("legacy row after rollback = %q, %v", original, err)
	}
}

func specifyTestFlow(id string) domain.RefinementFlow {
	f := socraticaTestFlow(id, "codex")
	f.Mode = domain.ModeSpecify
	f.Phase, f.LastStablePhase = domain.PhaseCaptured, domain.PhaseCaptured
	f.PayloadPolicy = domain.PayloadPersistent
	f.Preparation = domain.NewSpecifyPreparation()
	f.Preparation.Content.Intent = &domain.SpecifyIntent{Objective: "Exportar historial", ExpectedResult: "Archivo", Actors: []domain.SpecifyActor{{ID: "A1", Name: "Titular"}}}
	f.Preparation.Content.Scope = &domain.SpecifyScope{Included: []string{"Historial propio"}, Excluded: domain.DeclaredList{Declared: true, Items: []string{}},
		Pending: domain.PendingList{Declared: true, Items: []domain.SpecifyPending{{ID: "P1", Description: "Formato", Blocking: true, Resolution: domain.PendingOpen}}}}
	f.Preparation.Checks = []domain.SpecifyCheck{{Stage: domain.StageIntent, InputHash: "abc", Passed: true}}
	return f
}

func TestSpecifyStore_PreparationRoundTrip(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := specifyTestFlow("spec")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Preparation, f.Preparation) || got.PayloadPolicy != domain.PayloadPersistent {
		t.Fatalf("round-trip:\n got %#v\nwant %#v", got.Preparation, f.Preparation)
	}
	next := got
	next.Preparation = got.Preparation.Clone()
	next.Preparation.Content.Intent.Objective = "Exportar historial autorizado"
	next.Revision++
	next.UpdatedAt = got.UpdatedAt.Add(time.Second)
	if err = r.SaveTransition(ctx, got, next); err != nil {
		t.Fatal(err)
	}
	if got, err = r.GetFlow(ctx, f.ID); err != nil || got.Preparation.Content.Intent.Objective != "Exportar historial autorizado" || got.Revision != 1 {
		t.Fatalf("after transition: %+v, %v", got.Preparation, err)
	}
	// A stale revision is still refused by the existing CAS.
	if err = r.SaveTransition(ctx, f, next); !errors.Is(err, ErrSocraticaConcurrentUpdate) {
		t.Fatalf("stale SaveTransition = %v, want concurrent update", err)
	}
	h := domain.RefinementHistory{ID: f.ID, FlowID: f.ID, Project: "p", Client: "codex", Mode: domain.ModeSpecify, OriginalPrompt: "o", Preparation: got.Preparation, CompletedAt: time.Unix(200, 0).UTC()}
	if err = r.UpsertHistory(ctx, h, 5); err != nil {
		t.Fatal(err)
	}
	hs, err := r.ListHistory(ctx, "p", "codex", 5)
	if err != nil || len(hs) != 1 || !reflect.DeepEqual(hs[0].Preparation, got.Preparation) {
		t.Fatalf("history round-trip = %+v, %v", hs, err)
	}
}

func TestSpecifyStore_PayloadPolicyIsImmutable(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := specifyTestFlow("eph")
	f.PayloadPolicy = domain.PayloadEphemeral
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	next := f
	next.PayloadPolicy = domain.PayloadPersistent
	next.Revision++
	if err := r.SaveTransition(ctx, f, next); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, f.ID)
	if err != nil || got.PayloadPolicy != domain.PayloadEphemeral {
		t.Fatalf("policy = %q, %v; want ephemeral kept", got.PayloadPolicy, err)
	}
}

func TestSpecifyStore_NewerPreparationIsNotOverwritten(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := specifyTestFlow("future")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	future := `{"schema_version":99,"stage":"intent","content":{},"unknown":"kept"}`
	if _, err := r.db.Exec("UPDATE socratica_flows SET preparation_json=? WHERE id=?", future, f.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, f.ID)
	if err != nil || got.Preparation == nil || got.Preparation.SchemaVersion != 99 {
		t.Fatalf("read newer preparation = %+v, %v", got.Preparation, err)
	}
	next := got
	next.Preparation = domain.NewSpecifyPreparation()
	next.Revision++
	if err = r.SaveTransition(ctx, got, next); !errors.Is(err, domain.ErrUnsupportedPreparation) {
		t.Fatalf("SaveTransition over newer version = %v, want ErrUnsupportedPreparation", err)
	}
	var raw string
	if err = r.db.QueryRow("SELECT preparation_json FROM socratica_flows WHERE id=?", f.ID).Scan(&raw); err != nil || raw != future {
		t.Fatalf("stored preparation = %q, %v; want untouched", raw, err)
	}
}

// failingAlterExecer passes every statement to db except the ALTER that adds
// column, which fails with err (C-008 of ACR 042).
type failingAlterExecer struct {
	db     *sql.DB
	column string
	err    error
}

func (f failingAlterExecer) Exec(query string, args ...any) (sql.Result, error) {
	if strings.HasPrefix(query, "ALTER TABLE") && strings.Contains(query, " ADD COLUMN "+f.column+" ") {
		return nil, f.err
	}
	return f.db.Exec(query, args...)
}

// A failed ALTER must abort the migration so user_version is not raised over
// a catalog that lacks the columns scanSocratica reads.
func TestSpecifyMigration_FailedAlterAbortsMigration(t *testing.T) {
	raw, err := sql.Open("sqlite", catalogV2Fixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	ex := failingAlterExecer{db: raw, column: "payload_policy", err: errors.New("disk I/O error")}
	if err := migrateSchema(ex); err == nil || !strings.Contains(err.Error(), "socratica_flows.payload_policy") {
		t.Fatalf("migrateSchema = %v, want the failed ALTER reported", err)
	}
	ex.err = errors.New("duplicate column name: payload_policy")
	if err := migrateSchema(ex); err != nil {
		t.Fatalf("migrateSchema with an existing column = %v, want nil", err)
	}
}

// A lost CAS is reported with the contract category (C-004, ACR 042 r2).
func TestSpecifyStore_ConcurrentUpdateIsARevisionConflict(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := specifyTestFlow("cas")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	next := f
	next.Revision++
	if err := r.SaveTransition(ctx, f, next); err != nil {
		t.Fatal(err)
	}
	err := r.SaveTransition(ctx, f, next)
	if !errors.Is(err, ErrSocraticaConcurrentUpdate) || !errors.Is(err, domain.ErrRevisionConflict) {
		t.Fatalf("stale SaveTransition = %v, want a revision conflict", err)
	}
}

// Cancelling a flow with a newer preparation keeps it byte for byte, unknown
// fields included (C-005, ACR 042 r2).
func TestSpecifyStore_CancelKeepsANewerPreparationIntact(t *testing.T) {
	ctx := context.Background()
	r := socraticaTestRepo(t)
	f := specifyTestFlow("future-cancel")
	if err := r.CreateFlow(ctx, f); err != nil {
		t.Fatal(err)
	}
	future := `{"schema_version":99,"stage":"intent","content":{},"unknown":"kept"}`
	if _, err := r.db.Exec("UPDATE socratica_flows SET preparation_json=? WHERE id=?", future, f.ID); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFlow(ctx, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	next, _, err := domain.Reduce(got, domain.Event{Kind: "cancel"}, got.UpdatedAt.Add(time.Second), func() string { return "x" })
	if err != nil {
		t.Fatal(err)
	}
	if err = r.SaveTransition(ctx, got, next); err != nil {
		t.Fatalf("cancel over a newer preparation = %v", err)
	}
	var raw, phase string
	if err = r.db.QueryRow("SELECT preparation_json, phase FROM socratica_flows WHERE id=?", f.ID).Scan(&raw, &phase); err != nil || raw != future || phase != string(domain.PhaseCancelled) {
		t.Fatalf("after cancel: phase %q preparation %q, %v; want cancelled and untouched", phase, raw, err)
	}
}
