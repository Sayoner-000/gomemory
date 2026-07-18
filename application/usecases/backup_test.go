package usecases_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// TestCreateSnapshot_WritesValidBundleAndRestores cubre la Historia de Usuario 1
// (specs/009-mitigacion-riesgos): un snapshot generado por CreateSnapshot debe
// ser el mismo formato que ya sabe leer ImportBundle, y una restauración
// repetida no debe duplicar memorias (dedup existente de ImportBundle).
func TestCreateSnapshot_WritesValidBundleAndRestores(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "A", Content: "cuerpo A"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	backupDir := filepath.Join(t.TempDir(), "backups", "proj")
	path, err := usecases.CreateSnapshot(memRepo, relRepo, "proj", backupDir, 10)
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	var bundle domain.ExportBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		t.Fatalf("unmarshal snapshot: %v", err)
	}
	if len(bundle.Memories) != 1 || bundle.Memories[0].Title != "A" {
		t.Fatalf("bundle inesperado: %+v", bundle)
	}

	// Restaurar en un proyecto vacío usando el import ya existente.
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open snapshot: %v", err)
	}
	decoded, err := usecases.DecodeBundle(f)
	f.Close()
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	rep, err := usecases.ImportBundle(memRepo, relRepo, "restored", decoded)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if rep.MemoriesImported != 1 {
		t.Fatalf("esperaba 1 memoria restaurada, got %+v", rep)
	}

	// Restaurar el mismo snapshot dos veces no debe duplicar (dedup por hash).
	rep2, err := usecases.ImportBundle(memRepo, relRepo, "restored", decoded)
	if err != nil {
		t.Fatalf("reimport: %v", err)
	}
	if rep2.MemoriesImported != 0 || rep2.MemoriesSkipped != 1 {
		t.Fatalf("dedup falló en restauración repetida: %+v", rep2)
	}
}

// TestCreateSnapshot_PrunesOldestBeyondKeep cubre la retención acotada: solo
// deben sobrevivir los `keep` snapshots más recientes del proyecto.
func TestCreateSnapshot_PrunesOldestBeyondKeep(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)
	if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Content: "cuerpo"}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	backupDir := t.TempDir()
	const keep = 3
	var paths []string
	for i := 0; i < keep+2; i++ {
		p, err := usecases.CreateSnapshot(memRepo, relRepo, "proj", backupDir, keep)
		if err != nil {
			t.Fatalf("create snapshot %d: %v", i, err)
		}
		paths = append(paths, p)
	}

	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("read backup dir: %v", err)
	}
	if len(entries) != keep {
		t.Fatalf("esperaba %d snapshots retenidos, got %d", keep, len(entries))
	}
	if _, err := os.Stat(paths[len(paths)-1]); err != nil {
		t.Fatalf("el snapshot más reciente debía sobrevivir: %v", err)
	}
}

// TestCreateSnapshot_NeverBlocksOnBadDir simula un backupDir inutilizable (un
// archivo regular en su lugar) para confirmar que el error se propaga de forma
// simple y predecible — el caller (hookSessionEnd) es quien decide tragarlo.
func TestCreateSnapshot_NeverBlocksOnBadDir(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	blockedDir := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blockedDir, []byte("soy un archivo, no un directorio"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if _, err := usecases.CreateSnapshot(memRepo, relRepo, "proj", blockedDir, 10); err == nil {
		t.Fatal("esperaba error cuando backupDir no puede crearse")
	}
}
