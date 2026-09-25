package usecases_test

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

// T048 — con StableOrder (nivel max) la parte anterior al separador volátil es
// idéntica byte a byte entre dos construcciones aunque cambien la actividad
// automática y la sesión; sin StableOrder, el orden histórico no cambia.
func TestBuildContextStableOrder(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	for _, m := range []domain.Memory{
		{Type: domain.Decision, Title: "usa SQLite", Content: "decisión estable"},
		{Type: domain.Bugfix, Title: "arregla el hash", Content: "causa raíz: hash de la salida comprimida"},
		{Type: domain.Learning, Title: "aprendizaje", Content: "texto del aprendizaje"},
		{Type: domain.Checkpoint, Title: "Checkpoint", Content: "Editó: a.go"},
	} {
		m.Project = "proj"
		if _, err := memRepo.Insert(&m); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := sessRepo.Start("proj"); err != nil {
		t.Fatal(err)
	}

	build := func(stable bool) string {
		b := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
		b.StableOrder = stable
		out, err := b.Build()
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	prefix := func(doc string) string {
		i := strings.Index(doc, usecases.VolatileSeparator)
		if i < 0 {
			t.Fatalf("falta el separador volátil:\n%s", doc)
		}
		return doc[:i]
	}

	first := build(true)
	// Cambia lo volátil: actividad automática nueva.
	if _, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Checkpoint, Title: "Checkpoint", Content: "Editó: b.go"}); err != nil {
		t.Fatal(err)
	}
	second := build(true)
	if first == second {
		t.Fatal("control: la actividad nueva debería cambiar el documento")
	}
	if prefix(first) != prefix(second) {
		t.Errorf("el prefijo estable cambió:\n--- 1\n%s\n--- 2\n%s", prefix(first), prefix(second))
	}
	if strings.Contains(prefix(second), "Actividad Reciente") || strings.Contains(prefix(second), "Sesión Activa") {
		t.Error("las secciones volátiles deben ir detrás del separador")
	}

	// Sin StableOrder: sin separador y actividad antes que la sesión, como antes.
	legacy := build(false)
	if strings.Contains(legacy, usecases.VolatileSeparator) {
		t.Error("sin StableOrder no debe haber separador")
	}
	if strings.Index(legacy, "## Bugfixes") > strings.Index(legacy, "## Actividad Reciente") {
		t.Error("el orden histórico no debe cambiar")
	}
	// Mismo contenido, solo reordenado.
	if len(legacy)+len(usecases.VolatileSeparator)+2 != len(second) {
		t.Errorf("el reordenado no debe añadir ni quitar texto: %d vs %d", len(legacy), len(second))
	}
}
