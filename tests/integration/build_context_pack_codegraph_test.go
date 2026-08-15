package main

import (
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/tokens"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

// fakeCodeGraphProvider implementa ports.CodeGraphProvider para pruebas de
// integración de la feature 018, sin tocar ningún binario externo.
type fakeCodeGraphProvider struct {
	snap         domain.CodeProviderSnapshot
	impactByFile map[string]domain.CodeImpactAnnotation
}

func (f *fakeCodeGraphProvider) Name() string                          { return f.snap.Provider }
func (f *fakeCodeGraphProvider) Snapshot() domain.CodeProviderSnapshot { return f.snap }
func (f *fakeCodeGraphProvider) MaybeRefresh()                         {}

func (f *fakeCodeGraphProvider) ImpactFor(filepath string) (domain.CodeImpactAnnotation, bool) {
	ann, ok := f.impactByFile[filepath]
	return ann, ok
}

var _ ports.CodeGraphProvider = (*fakeCodeGraphProvider)(nil)

// TestBuildContextPack_NoCodeGraph_ExcludesArchitectureAndBoost cubre
// spec.md (feature 018) Historia 3: con IncludeCodeGraph=false, un
// CodeGraphProvider con hotspot y snapshot de arquitectura disponible no
// aporta ningún ítem "codegraph:*" ni sube ninguna prioridad — mismo
// resultado que si no hubiera proveedor configurado en absoluto (FR-009).
func TestBuildContextPack_NoCodeGraph_ExcludesArchitectureAndBoost(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	if _, err := memRepo.Insert(&domain.Memory{
		Project: "proj", Type: domain.Preference, Filepath: "hot.go",
		Title: "Nota de prueba", Content: "Nota de prueba sobre símbolo caliente.",
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}

	provider := &fakeCodeGraphProvider{
		snap: domain.CodeProviderSnapshot{
			Provider:  "fake-provider",
			Available: true,
			Architecture: &domain.CodeArchitecture{
				TotalNodes: 10,
				TotalEdges: 20,
			},
		},
		impactByFile: map[string]domain.CodeImpactAnnotation{
			"hot.go": {Hotspot: true, FanIn: 10},
		},
	}

	pack, err := usecases.BuildContextPack(
		memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, nil,
		usecases.ContextRequest{
			Task: "símbolo caliente", Project: "proj", MaxTokens: 2000,
			IncludeCodeGraph: false,
			CodeProviders:    []ports.CodeGraphProvider{provider},
		},
	)
	if err != nil {
		t.Fatalf("BuildContextPack: %v", err)
	}

	if len(pack.Items) != 1 {
		t.Fatalf("se esperaba 1 item (solo la memoria), hay %d", len(pack.Items))
	}
	if pack.Items[0].ID == "codegraph:architecture" || pack.Items[0].Priority != domain.PriorityOptional {
		t.Fatalf("IncludeCodeGraph=false no debería agregar ítems codegraph:* ni subir prioridades: %+v", pack.Items[0])
	}
}
