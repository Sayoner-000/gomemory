package persistence

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// writeLegacyDb crea <root>/.memory/mem.db con el esquema vigente y una
// memoria etiquetada con el nombre de proyecto antiguo (filepath.Base(root)).
func writeLegacyDb(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, MemDir)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := openDBFile(filepath.Join(dir, DbName))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO memories (project, type, title, content) VALUES (?, 'decision', 'legado', ?)`, filepath.Base(root), content); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func globalContents(t *testing.T, root string) []string {
	t.Helper()
	db, err := openDBFile(DbPath(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	rows, err := db.Query(`SELECT content FROM memories WHERE project = ? ORDER BY id`, ProjectKey(root))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		out = append(out, c)
	}
	return out
}

// TestMigrateLegacy_MueveNormalizaYRespetaForce cubre `mem migrate`: mueve la
// base legada al store global, normaliza la columna project a la clave
// nueva, rechaza sobrescribir un store existente sin --force y, con --force,
// descarta también el WAL anterior para no resucitar datos borrados.
func TestMigrateLegacy_MueveNormalizaYRespetaForce(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "proyecto-legado")
	writeLegacyDb(t, root, "contenido v1")

	if moved, err := MigrateLegacy(filepath.Join(t.TempDir(), "sin-legado"), false); err != nil || moved {
		t.Fatalf("sin legado no debe migrar: moved=%v err=%v", moved, err)
	}
	moved, err := MigrateLegacy(root, false)
	if err != nil || !moved {
		t.Fatalf("MigrateLegacy: moved=%v err=%v", moved, err)
	}
	if _, err := os.Stat(filepath.Join(root, MemDir, DbName)); !os.IsNotExist(err) {
		t.Fatalf("el legado debía moverse: %v", err)
	}
	if got := globalContents(t, root); len(got) != 1 || got[0] != "contenido v1" {
		t.Fatalf("filas migradas con la clave nueva: %v", got)
	}
	if info, err := os.Stat(DbPath(root)); err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("permisos del store migrado: %v %v", info, err)
	}

	writeLegacyDb(t, root, "contenido v2")
	if _, err := MigrateLegacy(root, false); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("sin --force debe negarse a sobrescribir: %v", err)
	}
	if err := os.WriteFile(DbPath(root)+"-wal", []byte("wal viejo"), 0o600); err != nil {
		t.Fatal(err)
	}
	moved, err = MigrateLegacy(root, true)
	if err != nil || !moved {
		t.Fatalf("MigrateLegacy --force: moved=%v err=%v", moved, err)
	}
	if got := globalContents(t, root); len(got) != 1 || got[0] != "contenido v2" {
		t.Fatalf("--force debe dejar solo el legado nuevo: %v", got)
	}
}

// TestEnsureDir_MigraLegadoPerezosamente: la primera apertura tras
// actualizar mueve el legado sin intervención; con store global presente no
// toca nada.
func TestEnsureDir_MigraLegadoPerezosamente(t *testing.T) {
	t.Setenv("GOMEMORY_DATA_HOME", t.TempDir())
	root := filepath.Join(t.TempDir(), "perezoso")
	writeLegacyDb(t, root, "dato perezoso")
	if err := EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	if got := globalContents(t, root); len(got) != 1 || got[0] != "dato perezoso" {
		t.Fatalf("migración perezosa: %v", got)
	}

	writeLegacyDb(t, root, "no debe pisar")
	if err := EnsureDir(root); err != nil {
		t.Fatalf("EnsureDir con store existente: %v", err)
	}
	if got := globalContents(t, root); len(got) != 1 || got[0] != "dato perezoso" {
		t.Fatalf("con store global presente no se sobrescribe: %v", got)
	}
}

// TestMoveFile_FuenteAusenteYCopia: una fuente ausente no es error (los
// -wal/-shm son opcionales) y un movimiento normal conserva el contenido.
func TestMoveFile_FuenteAusenteYCopia(t *testing.T) {
	dir := t.TempDir()
	if err := moveFile(filepath.Join(dir, "no-existe"), filepath.Join(dir, "dst")); err != nil {
		t.Fatalf("fuente ausente: %v", err)
	}
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("datos"), 0o600); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "sub", "dst")
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := moveFile(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(dst); string(b) != "datos" {
		t.Fatalf("contenido movido: %q", b)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatal("la fuente debía desaparecer")
	}
}
