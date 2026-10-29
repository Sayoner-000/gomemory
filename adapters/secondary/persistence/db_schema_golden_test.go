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

func TestMigrate_V3ToV4PreservesRowsAndCreatesHCDCatalog(t *testing.T) {
	db, err := openDBFile(filepath.Join(t.TempDir(), "mem.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`INSERT INTO memories(project, type, title, content, created_at, updated_at) VALUES('p','learning','keep','preserved','2026-10-27T00:00:00Z','2026-10-27T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA user_version = 3"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(db); err != nil {
		t.Fatalf("migrate v3: %v", err)
	}
	if got := userVersion(t, db); got != 4 {
		t.Fatalf("user_version = %d, want 4", got)
	}
	var content string
	if err := db.QueryRow("SELECT content FROM memories WHERE project = ? AND title = ?", "p", "keep").Scan(&content); err != nil || content != "preserved" {
		t.Fatalf("existing row content = %q, error %v", content, err)
	}
	for _, table := range []string{"hcd_units", "hcd_operations", "hcd_approval_prompts", "hcd_approvals", "hcd_clarifications", "hcd_decisions", "hcd_exceptions", "hcd_blocks", "hcd_executions", "hcd_evidence", "hcd_links", "hcd_normative_revisions", "hcd_focus", "hcd_heartbeats", "hcd_playbook_adoptions", "hcd_migrations"} {
		var count int
		if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&count); err != nil || count != 1 {
			t.Errorf("table %s missing after v3 migration (count=%d, err=%v)", table, count, err)
		}
	}
}
