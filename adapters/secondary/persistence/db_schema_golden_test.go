package persistence

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateSchemaGolden = flag.Bool("update-schema-golden", false, "rewrite testdata/schema.golden")

func schemaCatalog(t *testing.T, db *sql.DB) string {
	t.Helper()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "user_version=%d\n", version)
	rows, err := db.Query("SELECT type, name, tbl_name, COALESCE(sql, '') FROM sqlite_master ORDER BY type, name")
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var kind, name, table, statement string
		if err := rows.Scan(&kind, &name, &table, &statement); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&out, "%s|%s|%s|%s\n", kind, name, table, statement)
		if kind == "table" {
			tables = append(tables, name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	for _, table := range tables {
		cols, err := db.Query("SELECT cid, name, type, [notnull], COALESCE(dflt_value, ''), pk FROM pragma_table_info(?) ORDER BY cid", table)
		if err != nil {
			t.Fatal(err)
		}
		for cols.Next() {
			var cid, notnull, pk int
			var name, typ, def string
			if err := cols.Scan(&cid, &name, &typ, &notnull, &def, &pk); err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&out, "col|%s|%d|%s|%s|%d|%s|%d\n", table, cid, name, typ, notnull, def, pk)
		}
		if err := cols.Err(); err != nil {
			t.Fatal(err)
		}
		if err := cols.Close(); err != nil {
			t.Fatal(err)
		}
	}
	return out.String()
}

func TestSchemaGolden(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	got := schemaCatalog(t, db)
	path := filepath.Join("testdata", "schema.golden")
	if *updateSchemaGolden {
		if previous, err := os.ReadFile(path); err == nil {
			requireSchemaVersionBump(t, string(previous), got)
		} else if !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Fatalf("schema catalog differs (-want +got):\n-%s\n+%s", want, got)
	}
}

func requireSchemaVersionBump(t *testing.T, previous, current string) {
	t.Helper()
	oldHeader, oldCatalog, ok := strings.Cut(previous, "\n")
	if !ok {
		t.Fatal("invalid schema golden: missing version header")
	}
	_, newCatalog, ok := strings.Cut(current, "\n")
	if !ok {
		t.Fatal("invalid current schema: missing version header")
	}
	var oldVersion int
	if _, err := fmt.Sscanf(oldHeader, "user_version=%d", &oldVersion); err != nil {
		t.Fatal(err)
	}
	if oldCatalog != newCatalog && oldVersion == schemaVersion {
		t.Fatal("schema changed without bumping schemaVersion")
	}
}

func TestSchemaGolden_VersionBumpsWithCatalog(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	previous, err := os.ReadFile(filepath.Join("testdata", "schema.golden"))
	if err != nil {
		t.Fatal(err)
	}
	requireSchemaVersionBump(t, string(previous), schemaCatalog(t, db))
}

func TestMigrate_ThreeOpensSameCatalog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	var first string
	for i := 0; i < 3; i++ {
		db, err := openDBFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got := schemaCatalog(t, db)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = got
		} else if got != first {
			t.Fatalf("open %d changed the schema catalog", i+1)
		}
	}
}

func hcdTables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type = 'table' AND name LIKE 'hcd_%' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func seedMemoryAtVersion(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO memories(project, type, title, content, created_at, updated_at) VALUES('p','learning','keep','preserved','2026-10-27T00:00:00Z','2026-10-27T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", version)); err != nil {
		t.Fatal(err)
	}
}

func assertMigratedToV5(t *testing.T, db *sql.DB) {
	t.Helper()
	if err := migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if got := userVersion(t, db); got != 5 {
		t.Fatalf("user_version = %d, want 5", got)
	}
	var content string
	if err := db.QueryRow("SELECT content FROM memories WHERE project = ? AND title = ?", "p", "keep").Scan(&content); err != nil || content != "preserved" {
		t.Fatalf("existing row content = %q, error %v", content, err)
	}
	if got := strings.Join(hcdTables(t, db), ","); got != "hcd_approvals,hcd_evidence,hcd_units" {
		t.Fatalf("hcd tables = %s", got)
	}
	if _, err := db.Exec(`INSERT INTO hcd_units(project, id, number, title, slug, kind, lane, why, doc_path, created_at, updated_at) VALUES('p','HCD-001',1,'t','t','fix','fast','w','hcd/HCD-001-t.md','x','x')`); err != nil {
		t.Fatalf("v2 hcd_units not usable: %v", err)
	}
}

func TestMigrate_V3ToV5PreservesRowsAndCreatesHCDCatalog(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range hcdTables(t, db) {
		if _, err := db.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	seedMemoryAtVersion(t, db, 3)
	assertMigratedToV5(t, db)
}

// Una base v4 trae el catálogo HCD v1 que el binario de la rama 045 llegó a
// aplicar; v5 lo retira sin tocar las memorias (ADR 0003).
func TestMigrate_V4WithHCDV1CatalogDropsItAndKeepsMemories(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range hcdTables(t, db) {
		if _, err := db.Exec("DROP TABLE " + table); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range hcdV1Tables {
		if _, err := db.Exec("CREATE TABLE " + table + " (project TEXT NOT NULL, unit_id TEXT NOT NULL, legacy_payload TEXT NOT NULL)"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO hcd_units(project, unit_id, legacy_payload) VALUES('p','HCD-001','v1')`); err != nil {
		t.Fatal(err)
	}
	seedMemoryAtVersion(t, db, 4)
	assertMigratedToV5(t, db)
}

// Una base nueva o v3 nunca pasa por el DROP: solo una v4 tuvo el catálogo v1.
func TestDropHCDV1OnlyFromVersion4(t *testing.T) {
	var calls []string
	ex := recordingExecer{calls: &calls}
	for _, from := range []int{0, 3, 5} {
		if err := dropHCDV1(ex, from); err != nil {
			t.Fatal(err)
		}
	}
	if len(calls) != 0 {
		t.Fatalf("dropHCDV1 ran outside v4: %v", calls)
	}
	if err := dropHCDV1(ex, 4); err != nil {
		t.Fatal(err)
	}
	if len(calls) != len(hcdV1Tables) {
		t.Fatalf("calls = %d, want %d", len(calls), len(hcdV1Tables))
	}
}

type recordingExecer struct{ calls *[]string }

func (r recordingExecer) Exec(query string, _ ...any) (sql.Result, error) {
	*r.calls = append(*r.calls, query)
	return nil, nil
}
