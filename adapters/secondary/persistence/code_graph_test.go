package persistence

import (
	"testing"

	"mem/domain"
)

func TestReplaceFileNodesInsertsAndReturnsIDs(t *testing.T) {
	db := openTestDB(t)

	nodes := []domain.CodeNode{
		{Kind: domain.NodeFunction, Name: "Foo", Package: "pkg", Signature: "func Foo()", StartLine: 1, EndLine: 3, Exported: true},
		{Kind: domain.NodeFunction, Name: "bar", Package: "pkg", Signature: "func bar()", StartLine: 5, EndLine: 7},
	}

	inserted, err := ReplaceFileNodes(db, "proj", "pkg/file.go", "hash1", nodes)
	if err != nil {
		t.Fatalf("ReplaceFileNodes: %v", err)
	}
	if len(inserted) != 2 {
		t.Fatalf("esperaba 2 nodos insertados, got %d", len(inserted))
	}
	for _, n := range inserted {
		if n.ID == 0 {
			t.Error("esperaba un ID asignado")
		}
		if n.File != "pkg/file.go" {
			t.Errorf("esperaba File=pkg/file.go, got %s", n.File)
		}
	}

	hashes, err := FileHashesQuery(db, "proj")
	if err != nil {
		t.Fatalf("FileHashesQuery: %v", err)
	}
	if hashes["pkg/file.go"] != "hash1" {
		t.Errorf("esperaba hash1, got %s", hashes["pkg/file.go"])
	}
}

func TestReplaceFileNodesIsIdempotentOnReindex(t *testing.T) {
	db := openTestDB(t)

	first := []domain.CodeNode{{Kind: domain.NodeFunction, Name: "Foo", Package: "pkg"}}
	if _, err := ReplaceFileNodes(db, "proj", "pkg/file.go", "hash1", first); err != nil {
		t.Fatalf("first replace: %v", err)
	}

	second := []domain.CodeNode{
		{Kind: domain.NodeFunction, Name: "Foo", Package: "pkg"},
		{Kind: domain.NodeFunction, Name: "Baz", Package: "pkg"},
	}
	inserted, err := ReplaceFileNodes(db, "proj", "pkg/file.go", "hash2", second)
	if err != nil {
		t.Fatalf("second replace: %v", err)
	}
	if len(inserted) != 2 {
		t.Fatalf("esperaba 2 nodos tras el reindex, got %d", len(inserted))
	}

	all, err := NodesByName(db, "proj", "Foo")
	if err != nil {
		t.Fatalf("NodesByName: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("esperaba exactamente 1 nodo Foo tras reindexar (no duplicados), got %d", len(all))
	}

	status, err := CodeGraphStatus(db, "proj")
	if err != nil {
		t.Fatalf("CodeGraphStatus: %v", err)
	}
	if status.Nodes != 2 {
		t.Errorf("esperaba 2 nodos en total, got %d", status.Nodes)
	}
	if status.Files != 1 {
		t.Errorf("esperaba 1 archivo, got %d", status.Files)
	}
}

func TestDeleteCodeFileRemovesNodesAndEdges(t *testing.T) {
	db := openTestDB(t)

	nodesA, err := ReplaceFileNodes(db, "proj", "a.go", "h1", []domain.CodeNode{{Kind: domain.NodeFunction, Name: "A"}})
	if err != nil {
		t.Fatalf("replace a.go: %v", err)
	}
	nodesB, err := ReplaceFileNodes(db, "proj", "b.go", "h2", []domain.CodeNode{{Kind: domain.NodeFunction, Name: "B"}})
	if err != nil {
		t.Fatalf("replace b.go: %v", err)
	}

	edges := []domain.CodeEdge{{FromID: nodesA[0].ID, ToID: nodesB[0].ID, Kind: domain.EdgeCalls, Confidence: 1.0}}
	if err := InsertCodeEdges(db, "proj", "a.go", edges); err != nil {
		t.Fatalf("InsertCodeEdges: %v", err)
	}

	if err := DeleteCodeFile(db, "proj", "a.go"); err != nil {
		t.Fatalf("DeleteCodeFile: %v", err)
	}

	remaining, err := NodesByName(db, "proj", "A")
	if err != nil {
		t.Fatalf("NodesByName: %v", err)
	}
	if len(remaining) != 0 {
		t.Error("el nodo A debió eliminarse junto con el archivo")
	}

	status, err := CodeGraphStatus(db, "proj")
	if err != nil {
		t.Fatalf("CodeGraphStatus: %v", err)
	}
	if status.Edges != 0 {
		t.Errorf("las aristas originadas en a.go debieron eliminarse, got %d", status.Edges)
	}
	if status.Files != 1 {
		t.Errorf("esperaba que solo quede b.go, got %d archivos", status.Files)
	}
}

func TestSearchCodeNodes(t *testing.T) {
	db := openTestDB(t)

	nodes := []domain.CodeNode{
		{Kind: domain.NodeFunction, Name: "HandleRequest", Package: "server", Signature: "func HandleRequest(w http.ResponseWriter)"},
		{Kind: domain.NodeFunction, Name: "Unrelated", Package: "server"},
	}
	if _, err := ReplaceFileNodes(db, "proj", "server.go", "h1", nodes); err != nil {
		t.Fatalf("ReplaceFileNodes: %v", err)
	}

	results, err := SearchCodeNodes(db, "proj", "HandleRequest", 10)
	if err != nil {
		t.Fatalf("SearchCodeNodes: %v", err)
	}
	found := false
	for _, r := range results {
		if r.Name == "HandleRequest" {
			found = true
		}
	}
	if !found {
		t.Errorf("esperaba encontrar HandleRequest en la búsqueda, got %+v", results)
	}
}

func TestNeighborsBFS(t *testing.T) {
	db := openTestDB(t)

	a, _ := ReplaceFileNodes(db, "proj", "a.go", "h1", []domain.CodeNode{{Kind: domain.NodeFunction, Name: "A"}})
	b, _ := ReplaceFileNodes(db, "proj", "b.go", "h2", []domain.CodeNode{{Kind: domain.NodeFunction, Name: "B"}})
	c, _ := ReplaceFileNodes(db, "proj", "c.go", "h3", []domain.CodeNode{{Kind: domain.NodeFunction, Name: "C"}})

	if err := InsertCodeEdges(db, "proj", "a.go", []domain.CodeEdge{
		{FromID: a[0].ID, ToID: b[0].ID, Kind: domain.EdgeCalls, Confidence: 1.0},
	}); err != nil {
		t.Fatalf("insert edges a->b: %v", err)
	}
	if err := InsertCodeEdges(db, "proj", "b.go", []domain.CodeEdge{
		{FromID: b[0].ID, ToID: c[0].ID, Kind: domain.EdgeCalls, Confidence: 1.0},
	}); err != nil {
		t.Fatalf("insert edges b->c: %v", err)
	}

	// depth=1 desde A, dirección "out": solo debe alcanzar B.
	nodes, _, err := Neighbors(db, "proj", a[0].ID, domain.EdgeCalls, "out", 1)
	if err != nil {
		t.Fatalf("Neighbors depth 1: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Name != "B" {
		t.Errorf("esperaba solo [B] a profundidad 1, got %+v", nodes)
	}

	// depth=2 desde A, dirección "out": debe alcanzar B y C.
	nodes2, _, err := Neighbors(db, "proj", a[0].ID, domain.EdgeCalls, "out", 2)
	if err != nil {
		t.Fatalf("Neighbors depth 2: %v", err)
	}
	names := map[string]bool{}
	for _, n := range nodes2 {
		names[n.Name] = true
	}
	if !names["B"] || !names["C"] {
		t.Errorf("esperaba [B, C] a profundidad 2, got %+v", nodes2)
	}

	// dirección "in" desde C: debe alcanzar B (el llamador directo).
	nodesIn, _, err := Neighbors(db, "proj", c[0].ID, domain.EdgeCalls, "in", 1)
	if err != nil {
		t.Fatalf("Neighbors in: %v", err)
	}
	if len(nodesIn) != 1 || nodesIn[0].Name != "B" {
		t.Errorf("esperaba solo [B] como caller directo de C, got %+v", nodesIn)
	}
}
