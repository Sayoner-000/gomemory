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

// FindRoot resuelve la identidad del proyecto activo. Ya no exige que
// `.memory/` exista en el cwd o en algún padre: delega en FindProjectRoot,
// que usa el git root (o el cwd si no hay `.git`) como identidad. Se
// mantiene el nombre por compatibilidad con el resto del código, que la
// sigue usando para decir "dónde estoy parado", no "dónde ya se instaló".
func FindRoot() (string, error) {
	return FindProjectRoot()
}

// EnsureDir prepara el proyecto para usarse: crea su directorio en el store
// global (init perezoso — nunca falla por ausencia de instalación previa),
// migra un `.memory/mem.db` legado si lo encuentra, y mantiene `.memory/`
// dentro del árbol del proyecto para los archivos auxiliares que siguen
// viviendo ahí (marcador de sesión de hooks, `context.md` — ver
// specs/005-global-mcp-store/data-model.md, alcance de esta feature).
func EnsureDir(root string) error {
	key := ProjectKey(root)

	globalDir, err := GlobalProjectDir(key)
	if err != nil {
		return err
	}
	if err := migrateLegacyIfPresent(root, key); err != nil {
		return err
	}
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		return err
	}

	return os.MkdirAll(filepath.Join(root, MemDir), 0755)
}

// DbPath devuelve la ruta del mem.db de un proyecto en el store global. Solo
// puede fallar si no se puede resolver el directorio de datos del usuario
// (sin $HOME/$USERPROFILE: un entorno ya roto para casi cualquier
// herramienta), caso en el que se prefiere fallar rápido antes que fingir
// una ruta local que rompería el aislamiento del store global.
func DbPath(root string) string {
	path, err := GlobalDbPath(ProjectKey(root))
	if err != nil {
		panic(fmt.Sprintf("gomemory: no se pudo resolver el store global: %v", err))
	}
	return path
}

// Open abre (y migra el esquema de) el mem.db de un proyecto en el store
// global. Llama a EnsureDir primero: el directorio destino puede no existir
// todavía (init perezoso, FR-003) y sql.Open nunca crea directorios, solo
// archivos dentro de uno ya existente.
func Open(root string) (*sql.DB, error) {
	if err := EnsureDir(root); err != nil {
		return nil, fmt.Errorf("preparar directorio del proyecto: %w", err)
	}

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

// Init es un alias de Open: se conserva por compatibilidad con callers que
// documentan la intención de "primera inicialización", aunque desde el
// init perezoso ambos hacen exactamente lo mismo.
func Init(root string) (*sql.DB, error) {
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
	CREATE TABLE IF NOT EXISTS code_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project TEXT NOT NULL,
		path TEXT NOT NULL,
		hash TEXT NOT NULL,
		indexed_at TEXT NOT NULL DEFAULT (%s),
		UNIQUE(project, path)
	);
	CREATE INDEX IF NOT EXISTS idx_code_files_project ON code_files(project);
	CREATE TABLE IF NOT EXISTS code_nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project TEXT NOT NULL,
		file_id INTEGER NOT NULL DEFAULT 0,
		kind TEXT NOT NULL,
		name TEXT NOT NULL,
		package TEXT NOT NULL DEFAULT '',
		file TEXT NOT NULL DEFAULT '',
		receiver TEXT NOT NULL DEFAULT '',
		signature TEXT NOT NULL DEFAULT '',
		start_line INTEGER NOT NULL DEFAULT 0,
		end_line INTEGER NOT NULL DEFAULT 0,
		exported INTEGER NOT NULL DEFAULT 0,
		UNIQUE(project, kind, name, file_id)
	);
	CREATE INDEX IF NOT EXISTS idx_code_nodes_project_name ON code_nodes(project, name);
	CREATE INDEX IF NOT EXISTS idx_code_nodes_file ON code_nodes(file_id);
	CREATE TABLE IF NOT EXISTS code_edges (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		project TEXT NOT NULL,
		from_id INTEGER NOT NULL,
		to_id INTEGER NOT NULL,
		kind TEXT NOT NULL,
		confidence REAL NOT NULL DEFAULT 1.0,
		src_file_id INTEGER NOT NULL DEFAULT 0,
		UNIQUE(project, from_id, to_id, kind)
	);
	CREATE INDEX IF NOT EXISTS idx_code_edges_from ON code_edges(from_id);
	CREATE INDEX IF NOT EXISTS idx_code_edges_to ON code_edges(to_id);
	CREATE INDEX IF NOT EXISTS idx_code_edges_src_file ON code_edges(src_file_id);
	`, Now, Now, Now, Now, Now)
	if _, err := db.Exec(schema); err != nil {
		return err
	}

	// FTS5 es best-effort y separado del schema principal: si la build de
	// sqlite en uso no lo soporta, code_search simplemente no existe y
	// SearchNodes cae a LIKE — no debe romper la migración del resto de
	// gomemory (memorias, sesiones, relaciones).
	db.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS code_search USING fts5(
		name, signature, package, node_id UNINDEXED
	)`)

	return nil
}
