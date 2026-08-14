package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
	"mem/domain"
)

// TestBuildContextPack_RelevantWithinBudget_IrrelevantExcluded cubre la
// Historia 1 (spec.md, escenarios 1 y 3): con memorias relevantes e
// irrelevantes a la tarea, el paquete solo trae las relevantes, respeta el
// presupuesto de tokens, y cada item traza de vuelta a su memoria de origen.
func TestBuildContextPack_RelevantWithinBudget_IrrelevantExcluded(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	relevantID, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision,
		Title:   "Rotación refresh tokens",
		Content: "El servicio auth usa Redis para rotar refresh tokens sesión.",
	})
	if err != nil {
		t.Fatalf("insert relevante: %v", err)
	}
	// Título/contenido elegidos para NO compartir ningún término (ni siquiera
	// stopwords cortas como "de") con la tarea de abajo: tokenizeFTS no
	// filtra stopwords, así que cualquier palabra compartida (incluida "de")
	// haría matchear también a esta memoria.
	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Preference,
		Title:   "Preferencia estilo",
		Content: "Respuestas cortas sin resúmenes finales.",
	}); err != nil {
		t.Fatalf("insert irrelevante: %v", err)
	}

	pack, err := usecases.BuildContextPack(
		memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil,
		usecases.ContextRequest{
			Task:      "Rotación refresh tokens",
			Project:   "proj",
			MaxTokens: 500,
		},
	)
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}

	if pack.TokenCount > pack.Budget {
		t.Fatalf("TokenCount (%d) excede el Budget (%d)", pack.TokenCount, pack.Budget)
	}

	wantID := fmt.Sprintf("memory:%s", strconv.FormatInt(relevantID, 10))
	var found bool
	for _, item := range pack.Items {
		if strings.Contains(item.Content, "Preferencia estilo") || strings.Contains(item.Content, "resúmenes finales") {
			t.Fatalf("la memoria irrelevante no debería aparecer en el paquete: %+v", item)
		}
		if item.ID == wantID {
			found = true
		}
	}
	if !found {
		t.Fatalf("la memoria relevante (%s) no aparece en el paquete; items: %+v", wantID, pack.Items)
	}
}

// TestBuildContextPack_CriticalOverflow_ExplicitError cubre spec.md Historia
// 1, escenario 2: un presupuesto que no alcanza para lo crítico falla
// explícito en vez de devolver un paquete parcial.
func TestBuildContextPack_CriticalOverflow_ExplicitError(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	longContent := strings.Repeat("El servicio de auth usa Redis para rotar refresh tokens. ", 200)
	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Architecture,
		Title: "Arquitectura de refresh tokens", Content: longContent,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	_, err = usecases.BuildContextPack(
		memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil,
		usecases.ContextRequest{Task: "refresh tokens", Project: "proj", MaxTokens: 1},
	)
	if err == nil {
		t.Fatal("se esperaba un error de overflow crítico, no hubo error")
	}
}
