package persistence

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const MemDir = ".memory"
const DbName = "mem.db"

const Now = "datetime('now', '-5 hours')"

func FindRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, MemDir)); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no %s directory found in any parent (run 'mem init' first)", MemDir)
		}
		dir = parent
	}
}

func EnsureDir(root string) error {
	return os.MkdirAll(filepath.Join(root, MemDir), 0755)
}

func DbPath(root string) string {
	return filepath.Join(root, MemDir, DbName)
}

func Open(root string) (*sql.DB, error) {
	path := DbPath(root)
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func Init(root string) (*sql.DB, error) {
	if err := EnsureDir(root); err != nil {
		return nil, fmt.Errorf("create .memory dir: %w", err)
	}
	return Open(root)
}

func migrate(db *sql.DB) error {
	schema := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS memories (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project TEXT NOT NULL,
		session_id TEXT,
		type TEXT NOT NULL DEFAULT 'learning',
		title TEXT NOT NULL DEFAULT '',
		content TEXT NOT NULL,
		filepath TEXT,
		created_at TEXT NOT NULL DEFAULT (%s),
		updated_at TEXT NOT NULL DEFAULT (%s)
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		project TEXT NOT NULL,
		summary TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (%s),
		ended_at TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_memories_project ON memories(project);
	CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
	CREATE INDEX IF NOT EXISTS idx_memories_created ON memories(created_at DESC);
	CREATE TABLE IF NOT EXISTS memory_relations (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project TEXT NOT NULL,
		memory_id_a INTEGER NOT NULL,
		memory_id_b INTEGER NOT NULL,
		relation TEXT NOT NULL,
		confidence REAL NOT NULL DEFAULT 1.0,
		reasoning TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT (%s),
		FOREIGN KEY (memory_id_a) REFERENCES memories(id),
		FOREIGN KEY (memory_id_b) REFERENCES memories(id)
	);
	CREATE INDEX IF NOT EXISTS idx_relations_project ON memory_relations(project);
	CREATE INDEX IF NOT EXISTS idx_relations_mem_a ON memory_relations(memory_id_a);
	CREATE INDEX IF NOT EXISTS idx_relations_mem_b ON memory_relations(memory_id_b);
	`, Now, Now, Now, Now)
	_, err := db.Exec(schema)
	return err
}
