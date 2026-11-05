package persistence

import (
	"slices"
	"testing"

	"mem/domain"
)

// HCD-028 VS-02: las fuentes locales responden por archivo.

func TestListMemoriesByFilepath(t *testing.T) {
	db := openTestDB(t)
	SetCodeImpactProvider(nil)
	insert := func(project, title, path string, typ domain.MemoryType) int64 {
		t.Helper()
		id, err := InsertMemory(db, &domain.Memory{Project: project, Type: typ, Title: title, Content: "contenido de " + title, Filepath: path})
		if err != nil {
			t.Fatalf("insert %s: %v", title, err)
		}
		return id
	}
	a := insert("proj", "decisión sobre reglas", "domain/rules.go", domain.Decision)
	b := insert("proj", "bug en reglas", "/repo/domain/rules.go", domain.Bugfix)
	insert("proj", "otro archivo", "domain/expand.go", domain.Decision)
	insert("otro", "otro proyecto", "domain/rules.go", domain.Decision)
	c := insert("proj", "checkpoint de reglas", "domain/rules.go", domain.Checkpoint)

	got, err := ListMemoriesByFilepath(db, "proj", []string{"domain/rules.go", "/repo/domain/rules.go"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, m := range got {
		ids = append(ids, m.ID)
	}
	if !slices.Equal(ids, []int64{c, b, a}) {
		t.Fatalf("esperaba las memorias del archivo, más recientes primero: %v (a=%d b=%d c=%d)", ids, a, b, c)
	}
	if got, _ := ListMemoriesByFilepath(db, "proj", []string{"domain/rules.go"}, 1); len(got) != 1 {
		t.Fatalf("el límite debe respetarse: %d", len(got))
	}
	if got, _ := ListMemoriesByFilepath(db, "proj", nil, 10); len(got) != 0 {
		t.Fatalf("sin rutas no hay memorias: %d", len(got))
	}
}

func TestTestsCovering(t *testing.T) {
	db := openTestDB(t)
	replace := func(path string, names ...string) []domain.CodeNode {
		t.Helper()
		var nodes []domain.CodeNode
		for _, n := range names {
			nodes = append(nodes, domain.CodeNode{Kind: domain.NodeFunction, Name: n})
		}
		out, err := ReplaceFileNodes(db, "proj", path, "h-"+path, nodes)
		if err != nil {
			t.Fatalf("replace %s: %v", path, err)
		}
		return out
	}
	target := replace("domain/rules.go", "EvaluateRules")
	tests := replace("domain/rules_test.go", "TestBloquea", "TestOtra", "helperArmar")
	caller := replace("adapters/x.go", "Llamador")
	edge := func(from, to domain.CodeNode) domain.CodeEdge {
		return domain.CodeEdge{FromID: from.ID, ToID: to.ID, Kind: domain.EdgeCalls, Confidence: 0.9}
	}
	if err := InsertCodeEdges(db, "proj", "domain/rules_test.go", []domain.CodeEdge{
		edge(tests[0], target[0]), // TestBloquea → EvaluateRules
		edge(tests[2], target[0]), // helperArmar → EvaluateRules: no es una prueba
		edge(tests[1], caller[0]), // TestOtra → Llamador: no cubre rules.go
	}); err != nil {
		t.Fatal(err)
	}
	if err := InsertCodeEdges(db, "proj", "adapters/x.go", []domain.CodeEdge{edge(caller[0], target[0])}); err != nil {
		t.Fatal(err)
	}

	got, err := TestsCovering(db, "proj", "domain/rules.go", 5)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []domain.EditContextTest{{Name: "TestBloquea", Dir: "./domain"}}) {
		t.Fatalf("solo TestBloquea cubre domain/rules.go: %+v", got)
	}
	if got, _ := TestsCovering(db, "proj", "no/existe.go", 5); len(got) != 0 {
		t.Fatalf("un archivo sin nodos no tiene pruebas: %+v", got)
	}
}
