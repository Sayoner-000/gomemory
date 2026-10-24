package persistence

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestMigrate_FailureMidwayLeavesNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("CREATE VIEW memories AS SELECT 1 AS id"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	opened, err := openDBFile(path)
	if err == nil {
		opened.Close()
		t.Fatal("expected migration over a view to fail")
	}
	db, err = sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count, version int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name IN ('instinct_stats', 'socratica_flows')").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("failed migration left %d schema objects behind", count)
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != 0 {
		t.Errorf("failed migration changed user_version to %d", version)
	}
}

func TestMigrate_ToleratesDuplicateColumn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	for i := 0; i < 2; i++ {
		db, err := openDBFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('memories') WHERE name='topic_key'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("open %d: missing topic_key", i+1)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMigrate_ToleratesUniqueIndexOverDuplicates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mem.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE memory_relations (
		id INTEGER PRIMARY KEY AUTOINCREMENT, project TEXT NOT NULL,
		memory_id_a INTEGER NOT NULL, memory_id_b INTEGER NOT NULL,
		relation TEXT NOT NULL, confidence REAL NOT NULL DEFAULT 1.0,
		reasoning TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT (datetime('now', '-5 hours')),
		FOREIGN KEY (memory_id_a) REFERENCES memories(id),
		FOREIGN KEY (memory_id_b) REFERENCES memories(id));
		INSERT INTO memory_relations(project,memory_id_a,memory_id_b,relation)
		VALUES ('p',1,2,'related'),('p',1,2,'related');`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = openDBFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM memory_relations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected 2 preserved rows, got %d", count)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE name='idx_relations_pair'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Error("unique index unexpectedly exists over duplicate pairs")
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name IN ('memories','sessions')").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("expected remaining schema, got %d tables", count)
	}
}
