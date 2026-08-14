package speckit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestActiveFeature_MissingFile_ReturnsEmptyNoError(t *testing.T) {
	root := t.TempDir()
	r := Reader{}

	feature, err := r.ActiveFeature(root)
	if err != nil {
		t.Fatalf("ActiveFeature no debe fallar sin .specify/feature.json: %v", err)
	}
	if feature != "" {
		t.Fatalf("feature = %q, se esperaba vacío", feature)
	}
}

func TestActiveFeature_ReadsFeatureDirectory(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".specify", "feature.json"),
		`{"feature_directory": "specs/015-context-optimization"}`)
	r := Reader{}

	feature, err := r.ActiveFeature(root)
	if err != nil {
		t.Fatalf("ActiveFeature: %v", err)
	}
	if feature != "015-context-optimization" {
		t.Fatalf("feature = %q, se esperaba %q", feature, "015-context-optimization")
	}
}

func TestRead_ExtractsRelevantRequirementsScopedToFeature(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "specs", "015-context-optimization", "spec.md"), `
## Requirements

- **FR-001**: El sistema DEBE construir un ContextPack acotado a un presupuesto de tokens.
- **FR-002**: El sistema DEBE deduplicar memorias casi idénticas.
`)
	writeFile(t, filepath.Join(root, "specs", "099-otra-feature", "spec.md"), `
## Requirements

- **FR-001**: El sistema DEBE exportar reportes de facturación mensual.
`)

	r := Reader{}
	ctx, err := r.Read(root, "015-context-optimization", "presupuesto de tokens del ContextPack")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if ctx.Feature != "015-context-optimization" {
		t.Fatalf("Feature = %q, se esperaba 015-context-optimization", ctx.Feature)
	}
	foundRelevant := false
	for _, req := range ctx.Requirements {
		lower := strings.ToLower(req)
		if strings.Contains(lower, "presupuesto") && strings.Contains(lower, "tokens") {
			foundRelevant = true
		}
		if strings.Contains(lower, "facturación") {
			t.Fatalf("Read mezcló contenido de otra feature: %q", req)
		}
	}
	if !foundRelevant {
		t.Fatalf("no se encontró el requisito relevante entre: %+v", ctx.Requirements)
	}
}
