package main

import (
	"os"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/domain"
)

func memoryFixture(project string, memType domain.MemoryType) domain.Memory {
	return domain.Memory{
		Project: project,
		Type:    memType,
		Content: "contenido de prueba",
	}
}

func TestMaintenanceStats(t *testing.T) {
	root := t.TempDir()
	if err := persistence.EnsureDir(root); err != nil {
		t.Fatalf("ensure dir: %v", err)
	}

	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	memA1 := memoryFixture("proj-a", domain.Learning)
	memA2 := memoryFixture("proj-a", domain.Decision)
	memB1 := memoryFixture("proj-b", domain.Learning)
	if _, err := persistence.InsertMemory(db, &memA1); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := persistence.InsertMemory(db, &memA2); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := persistence.InsertMemory(db, &memB1); err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))

	stats, err := repo.Stats("proj-a")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}

	if stats.ProjectMemoryCount != 2 {
		t.Fatalf("expected 2 memorias en proj-a, got %d", stats.ProjectMemoryCount)
	}
	if stats.TotalMemoryCount != 3 {
		t.Fatalf("expected 3 memorias totales, got %d", stats.TotalMemoryCount)
	}

	info, err := os.Stat(persistence.DbPath(root))
	if err != nil {
		t.Fatalf("stat db file: %v", err)
	}
	if stats.FileSizeBytes != info.Size() {
		t.Fatalf("expected FileSizeBytes=%d, got %d", info.Size(), stats.FileSizeBytes)
	}
}

func TestMaintenancePurgeByProjectAndType(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	memA1 := memoryFixture("proj-a", domain.Learning)
	memA2 := memoryFixture("proj-a", domain.Decision)
	memB1 := memoryFixture("proj-b", domain.Learning)
	if _, err := persistence.InsertMemory(db, &memA1); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := persistence.InsertMemory(db, &memA2); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := persistence.InsertMemory(db, &memB1); err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))

	deleted, err := repo.Purge(ports.PurgeFilter{Project: "proj-a", Type: string(domain.Learning)})
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 memoria eliminada, got %d", deleted)
	}

	remaining, err := persistence.ListMemories(db, "proj-a", 50)
	if err != nil {
		t.Fatalf("list proj-a: %v", err)
	}
	if len(remaining) != 1 || remaining[0].Type != domain.Decision {
		t.Fatalf("expected solo la memoria 'decision' de proj-a, got %+v", remaining)
	}

	otherProject, err := persistence.ListMemories(db, "proj-b", 50)
	if err != nil {
		t.Fatalf("list proj-b: %v", err)
	}
	if len(otherProject) != 1 {
		t.Fatalf("proj-b no debe verse afectado, got %d memorias", len(otherProject))
	}
}

func TestMaintenancePurgeCleansOrphanRelations(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	memD := memoryFixture("proj-c", domain.Pattern)
	memE := memoryFixture("proj-c", domain.Pattern)
	idD, err := persistence.InsertMemory(db, &memD)
	if err != nil {
		t.Fatalf("insert D: %v", err)
	}
	idE, err := persistence.InsertMemory(db, &memE)
	if err != nil {
		t.Fatalf("insert E: %v", err)
	}

	if _, err := persistence.InsertRelation(db, &domain.Relation{
		Project:   "proj-c",
		MemoryIDA: idD,
		MemoryIDB: idE,
		Relation:  domain.Related,
	}); err != nil {
		t.Fatalf("insert relation: %v", err)
	}

	relsBefore, err := persistence.ListRelations(db, "proj-c", 10)
	if err != nil || len(relsBefore) != 1 {
		t.Fatalf("expected 1 relacion antes de purgar, got %d (err=%v)", len(relsBefore), err)
	}

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))
	if _, err := repo.Purge(ports.PurgeFilter{Project: "proj-c", Type: string(domain.Pattern)}); err != nil {
		t.Fatalf("purge: %v", err)
	}

	relsAfter, err := persistence.ListRelations(db, "proj-c", 10)
	if err != nil {
		t.Fatalf("list relations after purge: %v", err)
	}
	if len(relsAfter) != 0 {
		t.Fatalf("expected 0 relaciones huerfanas tras purgar, got %d", len(relsAfter))
	}
}

func TestMaintenancePurgeAllProjectsRequiresAllFlag(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	memA := memoryFixture("proj-a", domain.Learning)
	memB := memoryFixture("proj-b", domain.Learning)
	if _, err := persistence.InsertMemory(db, &memA); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := persistence.InsertMemory(db, &memB); err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))

	deleted, err := repo.Purge(ports.PurgeFilter{All: true})
	if err != nil {
		t.Fatalf("purge --all: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("expected 2 memorias eliminadas con --all, got %d", deleted)
	}
}

func TestMaintenancePurgeWithoutScopeFails(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))
	if _, err := repo.Purge(ports.PurgeFilter{}); err == nil {
		t.Fatal("expected error al purgar sin Project ni All=true (alcance ambiguo)")
	}
}

func TestMaintenanceCompactReclaimsDiskSpace(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))

	// Insertar y borrar muchas memorias deja paginas libres que VACUUM reclama.
	for i := 0; i < 500; i++ {
		mem := memoryFixture("proj-vacuum", domain.Learning)
		mem.Content = strings.Repeat("x", 500)
		if _, err := persistence.InsertMemory(db, &mem); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	if _, err := repo.Purge(ports.PurgeFilter{Project: "proj-vacuum"}); err != nil {
		t.Fatalf("purge: %v", err)
	}

	before, after, err := repo.Compact()
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if after > before {
		t.Fatalf("expected after<=before, got before=%d after=%d", before, after)
	}

	info, err := os.Stat(persistence.DbPath(root))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != after {
		t.Fatalf("expected el tamano reportado coincida con el archivo, got after=%d, real=%d", after, info.Size())
	}

	remaining, err := persistence.ListMemories(db, "proj-vacuum", 10)
	if err != nil {
		t.Fatalf("list after compact: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("compact no debe alterar filas, expected 0 memorias sobrevivientes, got %d", len(remaining))
	}
}

func TestMaintenanceCompactNoopOnAlreadyCompactDB(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))

	if _, _, err := repo.Compact(); err != nil {
		t.Fatalf("primer compact: %v", err)
	}
	// Segunda compactacion sobre una BD ya compacta no debe fallar.
	if _, _, err := repo.Compact(); err != nil {
		t.Fatalf("segundo compact sobre BD ya compacta: %v", err)
	}
}

func TestMaintenanceGCOnlyDeletesOlderThanThreshold(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	memOld := memoryFixture("proj-gc", domain.Learning)
	memRecent := memoryFixture("proj-gc", domain.Learning)
	idOld, err := persistence.InsertMemory(db, &memOld)
	if err != nil {
		t.Fatalf("insert old: %v", err)
	}
	if _, err := persistence.InsertMemory(db, &memRecent); err != nil {
		t.Fatalf("insert recent: %v", err)
	}
	if _, err := db.Exec(`UPDATE memories SET created_at = datetime('now', '-120 days') WHERE id = ?`, idOld); err != nil {
		t.Fatalf("backdate: %v", err)
	}

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))
	deleted, err := repo.Purge(ports.PurgeFilter{Project: "proj-gc", OlderThanDays: 90})
	if err != nil {
		t.Fatalf("gc (purge con OlderThanDays): %v", err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 memoria vieja eliminada, got %d", deleted)
	}

	remaining, err := persistence.ListMemories(db, "proj-gc", 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected 1 memoria reciente sobreviviente, got %d", len(remaining))
	}
}

func TestMaintenanceGCNoopWhenNothingExceedsThreshold(t *testing.T) {
	root := t.TempDir()
	_ = persistence.EnsureDir(root)
	db, err := persistence.Open(root)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	mem := memoryFixture("proj-gc2", domain.Learning)
	if _, err := persistence.InsertMemory(db, &mem); err != nil {
		t.Fatalf("insert: %v", err)
	}

	repo := persistence.NewMaintenanceRepository(db, persistence.DbPath(root))
	deleted, err := repo.Purge(ports.PurgeFilter{Project: "proj-gc2", OlderThanDays: 90})
	if err != nil {
		t.Fatalf("gc: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 memorias eliminadas (nada supera el umbral), got %d", deleted)
	}
}
