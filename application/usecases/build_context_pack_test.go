package usecases_test

import (
	"errors"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// newContextPackTestDeps abre una SQLite real en un directorio temporal —
// mismo criterio que build_context_test.go: se usa el adaptador real
// (MemoryRepository) en tests de caso de uso, no un doble, porque la
// búsqueda FTS5 es parte del comportamiento que se está probando
// (research.md §2).
func newContextPackTestDeps(t *testing.T) (memRepo ports.MemoryRepository) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return persistence.NewMemoryRepository(db)
}

func TestBuildContextPack_InvalidRequest(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	cases := []struct {
		name string
		req  usecases.ContextRequest
	}{
		{"task vacio", usecases.ContextRequest{Project: "proj", MaxTokens: 100}},
		{"project vacio", usecases.ContextRequest{Task: "algo", MaxTokens: 100}},
		{"max tokens cero", usecases.ContextRequest{Task: "algo", Project: "proj", MaxTokens: 0}},
		{"max tokens negativo", usecases.ContextRequest{Task: "algo", Project: "proj", MaxTokens: -1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, tc.req)
			if !errors.Is(err, domain.ErrInvalidContextRequest) {
				t.Fatalf("error = %v, se esperaba ErrInvalidContextRequest", err)
			}
		})
	}
}

func TestBuildContextPack_CriticalOverflow_NoPartialPack(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	// Decision/Architecture/Bugfix clasifican Critical (data-model.md) — un
	// contenido largo y un presupuesto absurdamente chico fuerza el overflow.
	longContent := ""
	for i := 0; i < 200; i++ {
		longContent += "El servicio de auth usa Redis para rotar refresh tokens. "
	}
	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision,
		Title: "Rotación de refresh tokens", Content: longContent,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "rotación de refresh tokens", Project: "proj", MaxTokens: 1,
	})
	if !errors.Is(err, domain.ErrCriticalContextOverflow) {
		t.Fatalf("error = %v, se esperaba ErrCriticalContextOverflow", err)
	}
	if len(pack.Items) != 0 {
		t.Fatalf("no debería devolverse un ContextPack parcial en overflow, tiene %d items", len(pack.Items))
	}
}

// TestBuildContextPack_StatsInvariants cubre spec.md US3 escenario 1 y
// data-model.md: los contadores de ContextStats deben ser consistentes entre
// sí, sin depender de cuántos items terminaron incluidos.
func TestBuildContextPack_StatsInvariants(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	for i, m := range []domain.Memory{
		{Project: "proj", Type: domain.Decision, Title: "Decisión rotación tokens", Content: "El servicio auth usa Redis para rotar tokens."},
		{Project: "proj", Type: domain.Pattern, Title: "Patrón rotación tokens", Content: "Rotar tokens cada 15 minutos es el patrón estándar."},
		{Project: "proj", Type: domain.Preference, Title: "Preferencia rotación tokens", Content: "Prefiero rotación de tokens automática sin intervención manual."},
	} {
		if _, err := memRepo.Insert(&m); err != nil {
			t.Fatalf("insert memoria %d: %v", i, err)
		}
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "rotación tokens", Project: "proj", MaxTokens: 1000,
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}

	s := pack.Stats
	sumIncluded := s.ItemsDuplicate + s.ItemsCritical + s.ItemsRelevant + s.ItemsOptional + s.ItemsDiscarded
	if s.ItemsRetrieved != sumIncluded {
		t.Errorf("ItemsRetrieved (%d) != suma de categorías (%d): %+v", s.ItemsRetrieved, sumIncluded, s)
	}
	if s.RawTokens-s.SavedTokens != s.FinalTokens {
		t.Errorf("RawTokens(%d) - SavedTokens(%d) != FinalTokens(%d)", s.RawTokens, s.SavedTokens, s.FinalTokens)
	}
}
