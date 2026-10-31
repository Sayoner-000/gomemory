package usecases_test

import (
	"errors"
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/ports"
	"mem/application/usecases"
	"mem/domain"
)

func importRepos(t *testing.T) (ports.MemoryRepository, ports.RelationRepository) {
	t.Helper()
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return persistence.NewMemoryRepository(db), persistence.NewRelationRepository(db)
}

func exportMem(ref int64, title, content string) domain.ExportMemory {
	return domain.ExportMemory{RefID: ref, Type: "decision", Title: title, Content: content, CreatedAt: "2020-01-01 00:00:00", UpdatedAt: "2020-01-01 00:00:00"}
}

func TestImportBundle_RechazaAntesDeEscribir(t *testing.T) {
	memRepo, relRepo := importRepos(t)
	bundle := domain.ExportBundle{Memories: []domain.ExportMemory{
		exportMem(901, "nueva-A", "nueva A"),
		exportMem(902, "veneno", "<private>s</private>"),
		exportMem(903, "nueva-C", "nueva C"),
	}}
	_, err := usecases.ImportBundle(memRepo, relRepo, "p", bundle)
	if !errors.Is(err, domain.ErrValidation) || !strings.Contains(err.Error(), "902") {
		t.Fatalf("err = %v, quiero validación que cite la ref 902", err)
	}
	all, err := memRepo.ListAll("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Fatalf("se escribieron %d memorias de un bundle inválido", len(all))
	}
}

type pairFailRepo struct{ ports.RelationRepository }

func (pairFailRepo) GetByPair(string, int64, int64) (*domain.Relation, error) {
	return nil, errors.New("fallo de lectura")
}

func TestImportBundle_PropagaFalloDeGetByPair(t *testing.T) {
	memRepo, relRepo := importRepos(t)
	bundle := domain.ExportBundle{
		Memories:  []domain.ExportMemory{exportMem(1, "a", "cuerpo a"), exportMem(2, "b", "cuerpo b")},
		Relations: []domain.ExportRelation{{RefA: 1, RefB: 2, Relation: "related", Confidence: 1}},
	}
	rep, err := usecases.ImportBundle(memRepo, pairFailRepo{relRepo}, "p", bundle)
	if err == nil || !strings.Contains(err.Error(), "fallo de lectura") {
		t.Fatalf("err = %v, quiero el fallo de GetByPair", err)
	}
	if !strings.Contains(err.Error(), "2 memorias") {
		t.Fatalf("err = %v, debe informar lo ya importado", err)
	}
	if rep.RelationsImported != 0 {
		t.Fatalf("relaciones importadas = %d", rep.RelationsImported)
	}
	rels, err := relRepo.ListAll("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 0 {
		t.Fatalf("se insertó una relación tras fallar la lectura: %d", len(rels))
	}
}

func TestImportBundle_ReimportarConSecretoNoDuplica(t *testing.T) {
	memRepo, relRepo := importRepos(t)
	bundle := domain.ExportBundle{Memories: []domain.ExportMemory{
		exportMem(1, "con secreto", "visible <private>oculto</private> resto"),
	}}
	for i := 0; i < 2; i++ {
		if _, err := usecases.ImportBundle(memRepo, relRepo, "p", bundle); err != nil {
			t.Fatal(err)
		}
	}
	all, err := memRepo.ListAll("p")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("memorias = %d, quiero 1", len(all))
	}
}
