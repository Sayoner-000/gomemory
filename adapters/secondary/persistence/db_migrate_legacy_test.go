package persistence

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func legacyDBFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mem.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE memories(id INTEGER PRIMARY KEY AUTOINCREMENT, project TEXT NOT NULL, session_id TEXT, type TEXT NOT NULL DEFAULT 'learning', title TEXT NOT NULL DEFAULT '', content TEXT NOT NULL, filepath TEXT, created_at TEXT NOT NULL DEFAULT '', updated_at TEXT NOT NULL DEFAULT '');
 CREATE TABLE sessions(id TEXT PRIMARY KEY, project TEXT NOT NULL, summary TEXT NOT NULL DEFAULT '', created_at TEXT NOT NULL DEFAULT '', ended_at TEXT);
 INSERT INTO memories(project,session_id,type,title,content,filepath,created_at,updated_at) VALUES ('legacy','s','learning','title','preserved content','src.go','2020-01-01','2020-01-02');
 INSERT INTO sessions(id,project,summary,created_at,ended_at) VALUES ('s','legacy','preserved summary','2020-01-01','2020-01-02');`)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestMigrate_LegacyDBKeepsData(t *testing.T) {
	db, err := openDBFile(legacyDBFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var id int
	var project, session, kind, title, content, file, created, updated string
	err = db.QueryRow(`SELECT id,project,session_id,type,title,content,filepath,created_at,updated_at FROM memories`).Scan(&id, &project, &session, &kind, &title, &content, &file, &created, &updated)
	if err != nil {
		t.Fatal(err)
	}
	if id != 1 || project != "legacy" || session != "s" || kind != "learning" || title != "title" || content != "preserved content" || file != "src.go" || created != "2020-01-01" || updated != "2020-01-02" {
		t.Fatalf("legacy memory changed: %d %q %q %q %q %q %q %q %q", id, project, session, kind, title, content, file, created, updated)
	}
	var sid, summary, ended string
	if err := db.QueryRow(`SELECT id,project,summary,created_at,ended_at FROM sessions`).Scan(&sid, &project, &summary, &created, &ended); err != nil {
		t.Fatal(err)
	}
	if sid != "s" || project != "legacy" || summary != "preserved summary" || created != "2020-01-01" || ended != "2020-01-02" {
		t.Fatalf("legacy session changed: %q %q %q %q %q", sid, project, summary, created, ended)
	}
	for table, column := range map[string]string{"memories": "topic_key", "sessions": "last_prompt"} {
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?", table, column).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Errorf("missing additive column %s.%s", table, column)
		}
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Errorf("schema version = %d, want %d", version, schemaVersion)
	}
}

func TestMigrate_LegacyDBGainsSessionsIndex(t *testing.T) {
	db, err := openDBFile(legacyDBFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var count, version int
	if err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_sessions_project_created'").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("legacy database did not gain the sessions index")
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != schemaVersion {
		t.Errorf("schema version = %d, want %d", version, schemaVersion)
	}
}
