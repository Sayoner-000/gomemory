package usecases

import (
	"os"
	"path/filepath"
	"testing"

	"mem/domain"
)

// HCD-041: una llamada desde un archivo que no cambia hacia un símbolo que
// aparece después en otro archivo se resuelve en el reindexado incremental,
// sin `mem index -force`. Antes no se guardaba la llamada sin resolver y nadie
// volvía a intentarlo.

func callsFrom(t *testing.T, ix *Indexer, name string) []domain.CodeNode {
	t.Helper()
	from, err := ix.Repo.NodesByName("test-project", name)
	if err != nil || len(from) != 1 {
		t.Fatalf("NodesByName %s: %v %+v", name, err, from)
	}
	nodes, _, err := ix.Repo.Neighbors("test-project", from[0].ID, domain.EdgeCalls, "out", 1)
	if err != nil {
		t.Fatalf("Neighbors %s: %v", name, err)
	}
	return nodes
}

func TestIndexIncremental_ResuelveLlamadaHaciaSimboloNuevo(t *testing.T) {
	for _, via := range []string{"IndexProject", "IndexFiles"} {
		t.Run(via, func(t *testing.T) {
			ix, root := newTestIndexer(t)
			writeFile(t, root, "pkg/a.go", "package pkg\n\nfunc A() { F() }\n")
			if _, err := ix.IndexProject(false); err != nil {
				t.Fatalf("IndexProject: %v", err)
			}
			if got := callsFrom(t, ix, "A"); len(got) != 0 {
				t.Fatalf("F aún no existe: A no debía llamar a nada, got %+v", got)
			}

			// a.go no cambia: solo se indexa b.go, como hace el hook turn-end.
			writeFile(t, root, "pkg/b.go", "package pkg\n\nfunc F() {}\n")
			var err error
			if via == "IndexProject" {
				_, err = ix.IndexProject(false)
			} else {
				_, err = ix.IndexFiles([]string{"pkg/b.go"})
			}
			if err != nil {
				t.Fatalf("%s: %v", via, err)
			}
			if got := callsFrom(t, ix, "A"); len(got) != 1 || got[0].Name != "F" {
				t.Fatalf("esperaba A→F tras indexar b.go, got %+v", got)
			}
		})
	}
}

func TestIndexIncremental_PendienteRetiradaNoSeResuelve(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "pkg/a.go", "package pkg\n\nfunc A() int { F(); return len(\"x\") }\n")
	writeFile(t, root, "pkg/c.go", "package pkg\n\nfunc G() { H() }\n")
	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}
	// Los builtins nunca serán símbolos del proyecto: no se guardan.
	if got, err := ix.Repo.PendingCallsByNames("test-project", []string{"len"}); err != nil || len(got) != 0 {
		t.Fatalf("las llamadas a builtins no deben quedar pendientes: %v %+v", err, got)
	}

	// a.go deja de llamar a F y c.go desaparece: sus pendientes se van con ellos.
	writeFile(t, root, "pkg/a.go", "package pkg\n\nfunc A() int { return 1 }\n")
	if err := os.Remove(filepath.Join(root, "pkg/c.go")); err != nil {
		t.Fatal(err)
	}
	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}
	if got, err := ix.Repo.PendingCallsByNames("test-project", []string{"F", "H"}); err != nil || len(got) != 0 {
		t.Fatalf("no deben quedar pendientes de llamadas retiradas: %v %+v", err, got)
	}

	writeFile(t, root, "pkg/b.go", "package pkg\n\nfunc F() {}\n\nfunc H() {}\n")
	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}
	if got := callsFrom(t, ix, "A"); len(got) != 0 {
		t.Fatalf("A ya no llama a F: no debía aparecer la arista, got %+v", got)
	}
}

func TestIndexIncremental_ResuelveSelectorHaciaSimboloNuevo(t *testing.T) {
	ix, root := newTestIndexer(t)
	writeFile(t, root, "sub/sub.go", "package sub\n\nfunc Otro() int { return 1 }\n")
	writeFile(t, root, "main.go", "package main\n\nimport \"test-project/sub\"\n\nfunc UseHelper() int { return sub.Helper() }\n")
	if _, err := ix.IndexProject(false); err != nil {
		t.Fatalf("IndexProject: %v", err)
	}
	if got := callsFrom(t, ix, "UseHelper"); len(got) != 0 {
		t.Fatalf("sub.Helper aún no existe, got %+v", got)
	}

	writeFile(t, root, "sub/sub.go", "package sub\n\nfunc Otro() int { return 1 }\n\nfunc Helper() int { return 7 }\n")
	if _, err := ix.IndexFiles([]string{"sub/sub.go"}); err != nil {
		t.Fatalf("IndexFiles: %v", err)
	}
	if got := callsFrom(t, ix, "UseHelper"); len(got) != 1 || got[0].Name != "Helper" {
		t.Fatalf("esperaba UseHelper→Helper tras indexar sub.go, got %+v", got)
	}
}
