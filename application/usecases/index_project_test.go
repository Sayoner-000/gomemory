package usecases

import (
	"os"
	"path/filepath"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/domain"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func newTestIndexer(t *testing.T) (*Indexer, string) {
	t.Helper()
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	repo := persistence.NewCodeGraphRepository(db)
	return NewIndexer(repo, root, "test-project"), root
}

func TestIndexProjectExtractsFunctionsMethodsAndTypes(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "pkg/greet.go", `package pkg

type Greeter struct {
	Name string
}

func (g Greeter) Hello() string {
	return "hello " + g.Name
}

func NewGreeter(name string) Greeter {
	return Greeter{Name: name}
}
`)

	report, err := ix.IndexProject(false)
	if err != nil {
		t.Fatalf("IndexProject: %v", err)
	}
	if report.Parsed != 1 {
		t.Errorf("esperaba 1 archivo parseado, got %d", report.Parsed)
	}
	// file + type Greeter + method Hello + func NewGreeter = 4 nodos
	if report.Nodes != 4 {
		t.Errorf("esperaba 4 nodos, got %d", report.Nodes)
	}

	funcs, err := ix.Repo.NodesByName("test-project", "NewGreeter")
	if err != nil {
		t.Fatalf("NodesByName: %v", err)
	}
	if len(funcs) != 1 || funcs[0].Kind != domain.NodeFunction {
		t.Fatalf("esperaba un nodo función NewGreeter, got %+v", funcs)
	}

	methods, err := ix.Repo.NodesByName("test-project", "Greeter.Hello")
	if err != nil {
		t.Fatalf("NodesByName: %v", err)
	}
	if len(methods) != 1 || methods[0].Kind != domain.NodeMethod {
		t.Fatalf("esperaba un nodo método Greeter.Hello, got %+v", methods)
	}

	types, err := ix.Repo.NodesByName("test-project", "Greeter")
	if err != nil {
		t.Fatalf("NodesByName: %v", err)
	}
	if len(types) != 1 || types[0].Kind != domain.NodeType {
		t.Fatalf("esperaba un nodo tipo Greeter, got %+v", types)
	}
}

func TestIndexProjectResolvesSamePackageCalls(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "pkg/a.go", `package pkg

func A() int {
	return B()
}
`)
	writeFile(t, root, "pkg/b.go", `package pkg

func B() int {
	return 42
}
`)

	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	aNodes, err := ix.Repo.NodesByName("test-project", "A")
	if err != nil || len(aNodes) != 1 {
		t.Fatalf("NodesByName A: %v %+v", err, aNodes)
	}
	bNodes, err := ix.Repo.NodesByName("test-project", "B")
	if err != nil || len(bNodes) != 1 {
		t.Fatalf("NodesByName B: %v %+v", err, bNodes)
	}

	nodes, edges, err := ix.Repo.Neighbors("test-project", aNodes[0].ID, domain.EdgeCalls, "out", 1)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("esperaba 1 arista CALLS desde A, got %d", len(edges))
	}
	if len(nodes) != 1 || nodes[0].Name != "B" {
		t.Fatalf("esperaba que A llame a B, got %+v", nodes)
	}
}

func TestIndexProjectResolvesSelectorCallsAcrossPackages(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "sub/sub.go", `package sub

func Helper() int {
	return 7
}
`)
	writeFile(t, root, "main.go", `package main

import "test-project/sub"

func UseHelper() int {
	return sub.Helper()
}
`)

	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	useNodes, err := ix.Repo.NodesByName("test-project", "UseHelper")
	if err != nil || len(useNodes) != 1 {
		t.Fatalf("NodesByName UseHelper: %v %+v", err, useNodes)
	}

	_, edges, err := ix.Repo.Neighbors("test-project", useNodes[0].ID, domain.EdgeCalls, "out", 1)
	if err != nil {
		t.Fatalf("Neighbors: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("esperaba 1 arista CALLS resuelta vía selector (sub.Helper), got %d", len(edges))
	}
}

func TestIndexProjectImportsEdges(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "main.go", `package main

import "fmt"

func Main() {
	fmt.Println("hi")
}
`)

	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}

	pkgNodes, err := ix.Repo.NodesByName("test-project", "fmt")
	if err != nil || len(pkgNodes) != 1 {
		t.Fatalf("esperaba un nodo paquete fmt, got %v %+v", err, pkgNodes)
	}
	if pkgNodes[0].Kind != domain.NodePackage {
		t.Errorf("esperaba kind=package, got %s", pkgNodes[0].Kind)
	}
}

func TestIndexProjectSkipsUnchangedFilesByHash(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "a.go", `package main

func A() {}
`)

	first, err := ix.IndexProject(false)
	if err != nil {
		t.Fatalf("first index: %v", err)
	}
	if first.Parsed != 1 {
		t.Fatalf("esperaba 1 archivo parseado en la primera corrida, got %d", first.Parsed)
	}

	second, err := ix.IndexProject(false)
	if err != nil {
		t.Fatalf("second index: %v", err)
	}
	if second.Parsed != 0 {
		t.Errorf("esperaba 0 archivos parseados en la segunda corrida (sin cambios), got %d", second.Parsed)
	}
	if second.Skipped != 1 {
		t.Errorf("esperaba 1 archivo omitido por hash igual, got %d", second.Skipped)
	}
}

func TestIndexProjectDeletesRemovedFiles(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "a.go", `package main

func A() {}
`)
	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("first index: %v", err)
	}

	if err := os.Remove(filepath.Join(root, "a.go")); err != nil {
		t.Fatalf("remove a.go: %v", err)
	}

	report, err := ix.IndexProject(false)
	if err != nil {
		t.Fatalf("second index: %v", err)
	}
	if report.Deleted != 1 {
		t.Errorf("esperaba 1 archivo borrado del grafo, got %d", report.Deleted)
	}

	status, err := ix.Repo.Status("test-project")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Files != 0 {
		t.Errorf("esperaba 0 archivos tras borrar a.go, got %d", status.Files)
	}
}

func TestIndexFilesIncremental(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "a.go", `package main

func A() {}
`)
	writeFile(t, root, "b.go", `package main

func B() {}
`)

	report, err := ix.IndexFiles([]string{"a.go"})
	if err != nil {
		t.Fatalf("IndexFiles: %v", err)
	}
	if report.Parsed != 1 {
		t.Errorf("esperaba 1 archivo parseado, got %d", report.Parsed)
	}

	status, err := ix.Repo.Status("test-project")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Files != 1 {
		t.Errorf("IndexFiles no debió tocar b.go, esperaba 1 archivo indexado, got %d", status.Files)
	}
}
