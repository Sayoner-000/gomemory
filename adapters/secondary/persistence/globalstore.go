package persistence

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// dataHomeEnvOverride permite fijar explícitamente el directorio de datos de
// gomemory, sin pasar por las convenciones de directorio del sistema
// operativo. Además de ser útil para usuarios avanzados, es lo que permite a
// los tests sandboxear el store global en un directorio temporal en vez de
// tocar el $HOME real de la máquina que corre la suite.
const dataHomeEnvOverride = "GOMEMORY_DATA_HOME"

// FindProjectRoot identifica la raíz de un proyecto subiendo desde el cwd en
// busca de un directorio `.git`. Si no encuentra ninguno (proyecto sin git
// inicializado), usa el cwd absoluto como identidad del proyecto. A
// diferencia de la vieja `FindRoot`, nunca falla por ausencia de instalación
// previa: solo puede fallar si `os.Getwd()` falla (entorno roto).
func FindProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := cwd
	for {
		if info, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil && info != nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd, nil
		}
		dir = parent
	}
}

var slugSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

const maxSlugLen = 40

// ProjectKey deriva una clave estable y segura para filesystem a partir de la
// ruta absoluta de un proyecto: un slug legible (para depuración humana, no
// para garantizar unicidad) más un hash corto de la ruta completa (lo que sí
// garantiza que dos proyectos con el mismo nombre de carpeta final, en rutas
// distintas, nunca colisionen). La misma ruta absoluta siempre produce la
// misma clave.
//
// Resuelve symlinks antes de hashear: en macOS os.Getwd() devuelve la ruta
// física (/private/var/... en vez de /var/...), así que dos llamadas sobre
// el mismo proyecto — una vía FindRoot (getwd) y otra con un --root crudo o
// un t.TempDir() sin resolver — producían claves distintas y partían la
// memoria de un mismo proyecto en dos stores (bug real detectado en
// TestCmdPurgeRequiresConfirmationThenDeletes/TestCmdGCEndToEnd). Si el path
// aún no existe o EvalSymlinks falla, se usa el path limpio tal cual.
func ProjectKey(root string) string {
	clean := filepath.Clean(root)
	if resolved, err := filepath.EvalSymlinks(clean); err == nil {
		clean = resolved
	}

	slug := slugSanitizer.ReplaceAllString(filepath.Base(clean), "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		slug = "project"
	}
	if len(slug) > maxSlugLen {
		slug = slug[:maxSlugLen]
	}

	sum := sha256.Sum256([]byte(clean))
	hash := hex.EncodeToString(sum[:8])

	return slug + "-" + hash
}

// DataHome resuelve el directorio base de datos de gomemory, respetando
// GOMEMORY_DATA_HOME si está definido, y si no, las convenciones del sistema
// operativo: $XDG_DATA_HOME (o ~/.local/share como fallback) en Linux/macOS,
// %LOCALAPPDATA% en Windows.
func DataHome() (string, error) {
	if v := os.Getenv(dataHomeEnvOverride); v != "" {
		return v, nil
	}

	if runtime.GOOS == "windows" {
		if v := os.Getenv("LOCALAPPDATA"); v != "" {
			return filepath.Join(v, "gomemory"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolver directorio de usuario: %w", err)
		}
		return filepath.Join(home, "AppData", "Local", "gomemory"), nil
	}

	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return filepath.Join(v, "gomemory"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolver directorio de usuario: %w", err)
	}
	return filepath.Join(home, ".local", "share", "gomemory"), nil
}

// GlobalProjectDir devuelve el directorio del store global reservado para el
// proyecto identificado por key.
func GlobalProjectDir(key string) (string, error) {
	home, err := DataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "projects", key), nil
}

// GlobalDbPath devuelve la ruta del mem.db en el store global para el
// proyecto identificado por key.
func GlobalDbPath(key string) (string, error) {
	dir, err := GlobalProjectDir(key)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, DbName), nil
}

// BackupDir devuelve el directorio de snapshots automáticos (specs/009-mitigacion-riesgos,
// Historia de Usuario 1) para el proyecto identificado por key, separado del
// directorio de datos activo (GlobalProjectDir) para que un snapshot nunca se
// confunda con el mem.db en uso.
func BackupDir(key string) (string, error) {
	home, err := DataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "backups", key), nil
}

// legacyProjectTables enumera las tablas con columna `project` que deben
// normalizarse al migrar un mem.db legado: son literales fijos (no entrada
// de usuario), nunca se concatenan con datos externos.
var legacyProjectTables = []string{
	"memories", "sessions", "memory_relations", "code_files", "code_nodes", "code_edges",
}

// migrateLegacyIfPresent es la versión perezosa usada por EnsureDir en cada
// apertura de proyecto: nunca sobrescribe (si el destino global ya tiene
// archivo, no toca nada) y no reporta nada al usuario — solo garantiza que
// el legado no se quede huérfano en el primer uso tras actualizar gomemory.
func migrateLegacyIfPresent(root, key string) error {
	if !hasLegacyDb(root) {
		return nil
	}
	globalPath, err := GlobalDbPath(key)
	if err != nil {
		return err
	}
	if _, err := os.Stat(globalPath); err == nil {
		return nil
	}
	return doMigrateLegacy(root, key)
}

// MigrateLegacy es la versión explícita usada por `mem migrate`: reporta si
// migró algo, y con force=true sobrescribe el store global aunque ya tenga
// datos propios (caso "ambos existen" documentado en
// specs/005-global-mcp-store/contracts/cli-contracts.md).
func MigrateLegacy(root string, force bool) (bool, error) {
	if !hasLegacyDb(root) {
		return false, nil
	}
	key := ProjectKey(root)
	globalPath, err := GlobalDbPath(key)
	if err != nil {
		return false, err
	}
	globalExists := false
	if _, err := os.Stat(globalPath); err == nil {
		globalExists = true
		if !force {
			return false, fmt.Errorf(
				"ya existe una base de datos en el store global (%s); usa --force para sobrescribirla con el legado de %s",
				globalPath, filepath.Join(root, MemDir, DbName))
		}
	}
	if globalExists {
		// force=true: descartar el store global existente por completo, WAL y
		// SHM incluidos — dejar restos de WAL de la BD vieja haría que SQLite
		// reproduzca frames de la BD descartada sobre el archivo recién
		// movido, resucitando datos que --force pidió explícitamente borrar.
		for _, p := range []string{globalPath, globalPath + "-wal", globalPath + "-shm"} {
			if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
				return false, fmt.Errorf("descartar store global previo: %w", err)
			}
		}
	}
	if err := doMigrateLegacy(root, key); err != nil {
		return false, err
	}
	return true, nil
}

func hasLegacyDb(root string) bool {
	_, err := os.Stat(filepath.Join(root, MemDir, DbName))
	return err == nil
}

// doMigrateLegacy mueve `<root>/.memory/mem.db` (+ -wal/-shm) al store
// global y normaliza la columna `project` de las filas migradas al nuevo
// valor de ProjectKey — sin esto, las filas legadas (etiquetadas con el
// viejo filepath.Base(root)) quedarían invisibles para cualquier lectura
// posterior, que ya filtra por la clave nueva.
func doMigrateLegacy(root, key string) error {
	legacyPath := filepath.Join(root, MemDir, DbName)
	globalPath, err := GlobalDbPath(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(globalPath), 0o700); err != nil {
		return fmt.Errorf("crear directorio del store global: %w", err)
	}

	oldProject := filepath.Base(filepath.Clean(root))

	if err := moveFile(legacyPath, globalPath); err != nil {
		return fmt.Errorf("mover mem.db legado: %w", err)
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = moveFile(legacyPath+suffix, globalPath+suffix)
	}
	os.Chmod(globalPath, 0o600) // hardening: el legado migrado hereda los permisos del mem.db nuevo

	db, err := sql.Open("sqlite", globalPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("abrir mem.db migrado: %w", err)
	}
	defer db.Close()

	for _, table := range legacyProjectTables {
		query := fmt.Sprintf("UPDATE %s SET project = ? WHERE project = ?", table)
		if _, err := db.Exec(query, key, oldProject); err != nil {
			return fmt.Errorf("normalizar project en %s: %w", table, err)
		}
	}

	return nil
}

// moveFile mueve src a dst, con fallback a copiar+borrar si están en
// filesystems distintos (os.Rename falla con "cross-device link"). No es
// error que src no exista (usado para -wal/-shm, que son opcionales).
func moveFile(src, dst string) error {
	if _, err := os.Stat(src); err != nil {
		return nil
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Remove(src)
}
