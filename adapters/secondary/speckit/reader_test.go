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

func TestRead_EmptyTaskReturnsWholeTaskGraph(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "specs", "027-octopus", "tasks.md"), "- [ ] T001 Crear ruta\n- [ ] T002 Validar salida\n")
	writeFile(t, filepath.Join(root, ".specify", "memory", "constitution.md"), "- El sistema DEBE registrar toda decisión de arquitectura.\n")

	ctx, err := (Reader{}).Read(root, "027-octopus", "")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(ctx.TaskDependencies) != 2 {
		t.Fatalf("TaskDependencies = %v; se esperaban todas las tareas", ctx.TaskDependencies)
	}
	if len(ctx.Constraints) != 1 {
		t.Fatalf("Constraints = %v; con tarea vacía se esperaba el grafo completo, igual que TaskDependencies", ctx.Constraints)
	}
}

// TestRead_ShortWordsTaskReturnsConstraints reproduce el hallazgo B-001/A-010
// de la ACR sobre Octopus AAR: taskWords descarta palabras de menos de
// minWordLength runas, así que una tarea real como "fix the bug" produce un
// taskWords vacío. relevantLines ya tolera ese caso (bypass len==0);
// relevantConstraintLines debe tolerarlo también.
func TestRead_ShortWordsTaskReturnsConstraints(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "specs", "015-context-optimization", "spec.md"),
		"- **FR-001**: El sistema DEBE hacer algo.\n")
	writeFile(t, filepath.Join(root, ".specify", "memory", "constitution.md"),
		"- El sistema DEBE registrar toda decisión de arquitectura.\n")

	ctx, err := (Reader{}).Read(root, "015-context-optimization", "fix the bug")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(ctx.Requirements) != 1 {
		t.Fatalf("Requirements = %v; se esperaba el grafo completo (todas las palabras de la tarea son cortas)", ctx.Requirements)
	}
	if len(ctx.Constraints) != 1 {
		t.Fatalf("Constraints = %v; se esperaba el grafo completo, igual que Requirements", ctx.Constraints)
	}
}
