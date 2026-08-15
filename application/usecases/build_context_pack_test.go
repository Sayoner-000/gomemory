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

// hotspotProvider arma un fakeCodeProvider (definido en build_context_test.go,
// mismo paquete usecases_test) cuyo único símbolo marcado como hotspot vigente
// es filepath — feature 018, Historia 1.
func hotspotProvider(filepath string) *fakeCodeProvider {
	return &fakeCodeProvider{
		snap: domain.CodeProviderSnapshot{Provider: "fake-provider", Available: true},
		impactByFile: map[string]domain.CodeImpactAnnotation{
			filepath: {Hotspot: true, FanIn: 10},
		},
	}
}

// TestBuildContextPack_CodeGraphHotspotBoostsPriority cubre spec.md (feature
// 018) Historia 1, acceptance scenario 1: una memoria Preference (Optional por
// tipo) cuyo Filepath es un hotspot vigente sube a Relevant antes de repartir
// presupuesto.
func TestBuildContextPack_CodeGraphHotspotBoostsPriority(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Preference, Filepath: "hot.go",
		Title: "Nota de prueba", Content: "Nota de prueba sobre símbolo caliente.",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "símbolo caliente", Project: "proj", MaxTokens: 2000,
		IncludeCodeGraph: true,
		CodeProviders:    []ports.CodeGraphProvider{hotspotProvider("hot.go")},
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item, hay %d", len(pack.Items))
	}
	if pack.Items[0].Priority != domain.PriorityRelevant {
		t.Fatalf("Priority = %v, se esperaba PriorityRelevant (boost por hotspot)", pack.Items[0].Priority)
	}
}

// TestBuildContextPack_CodeGraphNeverTouchesCriticalPriority cubre spec.md
// Historia 1, acceptance scenario 2: un item ya Critical no cambia aunque su
// archivo sea un hotspot vigente.
func TestBuildContextPack_CodeGraphNeverTouchesCriticalPriority(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision, Filepath: "hot.go",
		Title: "Decisión de prueba", Content: "Decisión de prueba sobre símbolo caliente.",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "símbolo caliente", Project: "proj", MaxTokens: 2000,
		IncludeCodeGraph: true,
		CodeProviders:    []ports.CodeGraphProvider{hotspotProvider("hot.go")},
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item, hay %d", len(pack.Items))
	}
	if pack.Items[0].Priority != domain.PriorityCritical {
		t.Fatalf("Priority = %v, se esperaba PriorityCritical sin cambios", pack.Items[0].Priority)
	}
}

// TestBuildContextPack_CodeGraphDisabled_NoBoost cubre spec.md Historia 3
// (desactivación): con IncludeCodeGraph=false, el boost no se aplica aunque
// haya un proveedor con hotspot disponible.
func TestBuildContextPack_CodeGraphDisabled_NoBoost(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Preference, Filepath: "hot.go",
		Title: "Nota de prueba", Content: "Nota de prueba sobre símbolo caliente.",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "símbolo caliente", Project: "proj", MaxTokens: 2000,
		IncludeCodeGraph: false,
		CodeProviders:    []ports.CodeGraphProvider{hotspotProvider("hot.go")},
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item, hay %d", len(pack.Items))
	}
	if pack.Items[0].Priority != domain.PriorityOptional {
		t.Fatalf("Priority = %v, se esperaba PriorityOptional (IncludeCodeGraph=false, sin boost)", pack.Items[0].Priority)
	}
}

// TestBuildContextPack_NoCodeProviders_NoBoost cubre spec.md Historia 1,
// acceptance scenario 3: sin ningún proveedor configurado, cero impacto.
func TestBuildContextPack_NoCodeProviders_NoBoost(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Preference, Filepath: "hot.go",
		Title: "Nota de prueba", Content: "Nota de prueba sobre símbolo caliente.",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "símbolo caliente", Project: "proj", MaxTokens: 2000,
		IncludeCodeGraph: true,
		CodeProviders:    nil,
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item, hay %d", len(pack.Items))
	}
	if pack.Items[0].Priority != domain.PriorityOptional {
		t.Fatalf("Priority = %v, se esperaba PriorityOptional (sin proveedores, cero impacto)", pack.Items[0].Priority)
	}
}

// architectureProvider arma un fakeCodeProvider con un snapshot de
// arquitectura disponible — feature 018, Historia 2.
func architectureProvider() *fakeCodeProvider {
	return &fakeCodeProvider{
		snap: domain.CodeProviderSnapshot{
			Provider:  "fake-provider",
			Available: true,
			Architecture: &domain.CodeArchitecture{
				TotalNodes: 10,
				TotalEdges: 20,
			},
		},
	}
}

// TestBuildContextPack_CodeGraphArchitectureCandidate_WhenAvailable cubre
// spec.md Historia 2, acceptance scenario 1: con snapshot disponible y
// presupuesto amplio, aparece un ítem de arquitectura de código.
func TestBuildContextPack_CodeGraphArchitectureCandidate_WhenAvailable(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "cualquier tarea", Project: "proj", MaxTokens: 2000,
		IncludeCodeGraph: true,
		CodeProviders:    []ports.CodeGraphProvider{architectureProvider()},
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item, hay %d", len(pack.Items))
	}
	if pack.Items[0].ID != "codegraph:architecture" {
		t.Fatalf("ID = %q, se esperaba \"codegraph:architecture\"", pack.Items[0].ID)
	}
	if pack.Items[0].Priority != domain.PriorityOptional {
		t.Fatalf("Priority = %v, se esperaba PriorityOptional", pack.Items[0].Priority)
	}
}

// TestBuildContextPack_CodeGraphArchitectureAbsent_WhenUnavailable cubre
// spec.md Historia 2, acceptance scenario 2: sin snapshot disponible, cero
// mención — ningún ítem de arquitectura.
func TestBuildContextPack_CodeGraphArchitectureAbsent_WhenUnavailable(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "cualquier tarea", Project: "proj", MaxTokens: 2000,
		IncludeCodeGraph: true,
		CodeProviders: []ports.CodeGraphProvider{&fakeCodeProvider{
			snap: domain.CodeProviderSnapshot{Available: false},
		}},
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	if len(pack.Items) != 0 {
		t.Fatalf("se esperaba 0 items, hay %d", len(pack.Items))
	}
}

// TestBuildContextPack_CodeGraphArchitectureDiscardedWhenBudgetTight cubre
// spec.md Historia 2, acceptance scenario 3: snapshot disponible pero
// presupuesto que ya se agota en contenido crítico — el ítem de arquitectura
// queda descartado, contabilizado en ItemsDiscarded, sin afectar el resto.
func TestBuildContextPack_CodeGraphArchitectureDiscardedWhenBudgetTight(t *testing.T) {
	memRepo := newContextPackTestDeps(t)
	compressor := compression.StructuralCompressor{}
	counter := tokens.ApproximateTokenCounter{}

	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Decision,
		Title: "Decisión corta", Content: "Decisión corta de prueba.",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	pack, err := usecases.BuildContextPack(memRepo, compressor, counter, nil, usecases.ContextRequest{
		Task: "decisión corta", Project: "proj", MaxTokens: 30,
		IncludeCodeGraph: true,
		CodeProviders:    []ports.CodeGraphProvider{architectureProvider()},
	})
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}
	for _, item := range pack.Items {
		if item.ID == "codegraph:architecture" {
			t.Fatalf("el ítem de arquitectura no debería caber en un presupuesto de 30 tokens junto con lo crítico")
		}
	}
	if pack.Stats.ItemsDiscarded < 1 {
		t.Fatalf("ItemsDiscarded = %d, se esperaba al menos 1 (el ítem de arquitectura descartado)", pack.Stats.ItemsDiscarded)
	}
}
