package main

import (
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
	"mem/domain"
)

// TestBuildContextPack_NearDuplicates_CollapsedToOne cubre spec.md Historia 2
// (ambos escenarios): dos memorias casi idénticas sobre el mismo tema
// colapsan a una sola en el paquete, y la descartada se refleja en
// ContextStats.ItemsDuplicate.
func TestBuildContextPack_NearDuplicates_CollapsedToOne(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	// Mismo tema, misma redacción parafraseada — Jaccard sobre Título+Content
	// (application/usecases/detect_duplicates.go) debería agruparlas.
	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision,
		Title:   "Redis para sesiones",
		Content: "El servicio auth usa Redis para guardar sesiones de refresh token.",
	}); err != nil {
		t.Fatalf("insert memoria a: %v", err)
	}
	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision,
		Title:   "Redis sesiones nota dos",
		Content: "Las sesiones de refresh token se guardan en Redis en el servicio auth.",
	}); err != nil {
		t.Fatalf("insert memoria b: %v", err)
	}

	pack, err := usecases.BuildContextPack(
		memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil,
		usecases.ContextRequest{Task: "sesiones Redis", Project: "proj", MaxTokens: 2000},
	)
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}

	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item tras dedup, hubo %d: %+v", len(pack.Items), pack.Items)
	}
	if pack.Stats.ItemsDuplicate != 1 {
		t.Fatalf("Stats.ItemsDuplicate = %d, se esperaba 1", pack.Stats.ItemsDuplicate)
	}
}
