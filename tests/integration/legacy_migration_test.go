package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func legacyMemoryTitle(i int) string {
	return fmt.Sprintf("memoria legada %d", i)
}

// seedLegacyDb crea un `.memory/mem.db` legado directamente en la ubicación
// donde vivía antes de esta feature (<root>/.memory/mem.db), simulando un
// proyecto instalado con `mem install` en una versión anterior de gomemory.
// Para sembrar el archivo se reutiliza persistence.Open/InsertMemory contra
// un GOMEMORY_DATA_HOME temporal (así se obtiene el esquema real migrado sin
// duplicar SQL), y luego se mueve el resultado a la ruta legada; el
// GOMEMORY_DATA_HOME real (fijado por TestMain) se restaura antes de volver,
// para que el resto del test dispare la migración de verdad al invocar bin.
func seedLegacyDb(t *testing.T, root string, n int) {
	t.Helper()

	origDataHome := os.Getenv("GOMEMORY_DATA_HOME")
	seedHome := t.TempDir()
	if err := os.Setenv("GOMEMORY_DATA_HOME", seedHome); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer os.Setenv("GOMEMORY_DATA_HOME", origDataHome)

	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("abrir mem.db temporal para sembrar legado: %v", err)
	}
	legacyProject := filepath.Base(filepath.Clean(root))
	for i := 0; i < n; i++ {
		mem := domain.Memory{
			Project: legacyProject,
			Type:    domain.Learning,
			Title:   legacyMemoryTitle(i),
			Content: "contenido de la memoria legada",
		}
		if _, err := persistence.InsertMemory(db, &mem); err != nil {
			db.Close()
			t.Fatalf("insertar memoria legada: %v", err)
		}
	}
	db.Close()

	seededPath, err := persistence.GlobalDbPath(persistence.ProjectKey(root))
	if err != nil {
		t.Fatalf("resolver ruta sembrada: %v", err)
	}
	memDir := filepath.Join(root, ".memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("crear .memory legado: %v", err)
	}
	if err := os.Rename(seededPath, filepath.Join(memDir, "mem.db")); err != nil {
		t.Fatalf("mover semilla a ruta legada: %v", err)
	}
}

// TestLegacyMigrationPreservesMemories cubre US2 (specs/005-global-mcp-store):
// un proyecto con `.memory/mem.db` legado (N memorias) conserva su historial
// íntegro al pasar al nuevo modelo — la migración corre de forma perezosa en
// el primer `mem save`/`mem list`.
func TestLegacyMigrationPreservesMemories(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	const n = 3
	seedLegacyDb(t, target, n)

	list := exec.Command(bin, "list", "-n", "50")
	list.Dir = target
	var out bytes.Buffer
	list.Stdout = &out
	list.Stderr = &out
	if err := list.Run(); err != nil {
		t.Fatalf("mem list tras migración perezosa: %v\n%s", err, out.String())
	}

	for i := 0; i < n; i++ {
		title := legacyMemoryTitle(i)
		if !bytes.Contains(out.Bytes(), []byte(title)) {
			t.Fatalf("esperaba encontrar la memoria migrada %q, output:\n%s", title, out.String())
		}
	}

	if _, err := os.Stat(filepath.Join(target, ".memory", "mem.db")); err == nil {
		t.Fatal("el mem.db legado debió moverse (no quedar duplicado) tras la migración")
	}
}

// TestMigrateCommandReportsAlreadyMigrated cubre el contrato de `mem migrate`
// (specs/005-global-mcp-store/contracts/cli-contracts.md): sin legado
// pendiente, reporta que no hay nada que migrar en vez de fallar.
func TestMigrateCommandReportsAlreadyMigrated(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	migrate := exec.Command(bin, "migrate")
	migrate.Dir = target
	var out bytes.Buffer
	migrate.Stdout = &out
	migrate.Stderr = &out
	if err := migrate.Run(); err != nil {
		t.Fatalf("mem migrate sin legado no debió fallar: %v\n%s", err, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("Nada que migrar")) {
		t.Fatalf("esperaba mensaje de 'nada que migrar', output:\n%s", out.String())
	}
}

// TestMigrateCommandForceOverwritesGlobal cubre el caso "ambos existen" del
// contrato de mem migrate: sin --force falla pidiendo confirmación, con
// --force sobrescribe el store global con el legado.
func TestMigrateCommandForceOverwritesGlobal(t *testing.T) {
	bin := buildMemBinary(t)
	target := t.TempDir()

	save := exec.Command(bin, "save", "-t", "ya en global", "-y", "decision", "contenido")
	save.Dir = target
	if out, err := save.CombinedOutput(); err != nil {
		t.Fatalf("mem save: %v\n%s", err, out)
	}

	seedLegacyDb(t, target, 1)

	migrateNoForce := exec.Command(bin, "migrate")
	migrateNoForce.Dir = target
	if out, err := migrateNoForce.CombinedOutput(); err == nil {
		t.Fatalf("mem migrate sin --force debió fallar con ambos existentes, output:\n%s", out)
	}

	migrateForce := exec.Command(bin, "migrate", "--force")
	migrateForce.Dir = target
	var out bytes.Buffer
	migrateForce.Stdout = &out
	migrateForce.Stderr = &out
	if err := migrateForce.Run(); err != nil {
		t.Fatalf("mem migrate --force: %v\n%s", err, out.String())
	}

	list := exec.Command(bin, "list", "-n", "50")
	list.Dir = target
	var listOut bytes.Buffer
	list.Stdout = &listOut
	list.Stderr = &listOut
	if err := list.Run(); err != nil {
		t.Fatalf("mem list: %v\n%s", err, listOut.String())
	}
	if bytes.Contains(listOut.Bytes(), []byte("ya en global")) {
		t.Fatalf("--force debió sobrescribir el store global con el legado, pero sigue la memoria vieja:\n%s", listOut.String())
	}
	if !bytes.Contains(listOut.Bytes(), []byte(legacyMemoryTitle(0))) {
		t.Fatalf("esperaba la memoria del legado tras --force, output:\n%s", listOut.String())
	}
}
