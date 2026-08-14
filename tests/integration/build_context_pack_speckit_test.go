package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mem/adapters/secondary/compression"
	"mem/adapters/secondary/persistence"
	"mem/adapters/secondary/speckit"
	"mem/adapters/secondary/tokens"
	"mem/application/usecases"
)

func writeSpeckitFixture(t *testing.T, root string) {
	t.Helper()
	writeFileHelper(t, filepath.Join(root, ".specify", "feature.json"),
		`{"feature_directory": "specs/015-context-optimization"}`)
	writeFileHelper(t, filepath.Join(root, "specs", "015-context-optimization", "spec.md"), `
## Requirements

- **FR-001**: El sistema DEBE respetar el presupuesto de tokens del ContextPack.
`)
	writeFileHelper(t, filepath.Join(root, "specs", "099-otra-feature", "spec.md"), `
## Requirements

- **FR-001**: El sistema DEBE exportar reportes de facturación mensual.
`)
}

func writeFileHelper(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// TestBuildContextPack_SpecKit_ScopedToActiveFeature cubre spec.md Historia 4
// (ambos escenarios): con IncludeSpecKit activo solo entra contenido de la
// feature activa (.specify/feature.json), nunca de otra; con IncludeSpecKit
// apagado no entra nada de specs/.
func TestBuildContextPack_SpecKit_ScopedToActiveFeature(t *testing.T) {
	root := t.TempDir()
	writeSpeckitFixture(t, root)

	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	memRepo := persistence.NewMemoryRepository(db)

	reader := speckit.Reader{}

	packOn, err := usecases.BuildContextPack(
		memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, reader,
		usecases.ContextRequest{
			Task: "presupuesto de tokens del ContextPack", Project: "proj",
			MaxTokens: 2000, Root: root, IncludeSpecKit: true,
		},
	)
	if err != nil {
		t.Fatalf("BuildContextPack (speckit on): %v", err)
	}
	var sawRelevant bool
	for _, item := range packOn.Items {
		if strings.Contains(item.Content, "facturación") {
			t.Fatalf("se filtró contenido de otra feature: %+v", item)
		}
		if strings.Contains(item.Content, "presupuesto de tokens") {
			sawRelevant = true
		}
	}
	if !sawRelevant {
		t.Fatalf("no se incluyó el requisito de la feature activa; items: %+v", packOn.Items)
	}

	packOff, err := usecases.BuildContextPack(
		memRepo, compression.StructuralCompressor{}, tokens.ApproximateTokenCounter{}, reader,
		usecases.ContextRequest{
			Task: "presupuesto de tokens del ContextPack", Project: "proj",
			MaxTokens: 2000, Root: root, IncludeSpecKit: false,
		},
	)
	if err != nil {
		t.Fatalf("BuildContextPack (speckit off): %v", err)
	}
	for _, item := range packOff.Items {
		if strings.HasPrefix(item.ID, "speckit:") {
			t.Fatalf("IncludeSpecKit=false no debería incluir ningún item de Spec Kit: %+v", item)
		}
	}
}
