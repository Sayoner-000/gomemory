package usecases_test

import (
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

func TestRecordVerdict_InsertsNewRelation(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	relRepo := persistence.NewRelationRepository(db)

	rel, updated, err := usecases.RecordVerdict(relRepo, "proj", 1, 2, domain.ConflictsWith, 0.8, "se contradicen")
	if err != nil {
		t.Fatalf("record verdict: %v", err)
	}
	if updated {
		t.Fatal("expected updated=false for a brand new pair")
	}
	if rel.Relation != domain.ConflictsWith || rel.Reasoning != "se contradicen" {
		t.Fatalf("unexpected relation: %+v", rel)
	}
}

func TestRecordVerdict_UpdatesExistingRelationForPair(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	relRepo := persistence.NewRelationRepository(db)

	first, _, err := usecases.RecordVerdict(relRepo, "proj", 1, 2, domain.ConflictsWith, 0.5, "conflicto inicial")
	if err != nil {
		t.Fatalf("first verdict: %v", err)
	}

	second, updated, err := usecases.RecordVerdict(relRepo, "proj", 1, 2, domain.Supersedes, 0.95, "verifiqué el código: la memoria 2 refleja el estado actual")
	if err != nil {
		t.Fatalf("second verdict: %v", err)
	}
	if !updated {
		t.Fatal("expected updated=true for an existing pair")
	}
	if second.ID != first.ID {
		t.Fatalf("expected same relation id to be reused, got %d vs %d", first.ID, second.ID)
	}
	if second.Relation != domain.Supersedes {
		t.Fatalf("expected relation to be overwritten to supersedes, got %s", second.Relation)
	}

	rels, err := relRepo.List("proj", 10)
	if err != nil {
		t.Fatalf("list relations: %v", err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected exactly 1 relation row for the pair, got %d", len(rels))
	}
}

func TestRecordVerdict_RejectsSelfComparison(t *testing.T) {
	db, err := persistence.Init(t.TempDir())
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()
	relRepo := persistence.NewRelationRepository(db)

	if _, _, err := usecases.RecordVerdict(relRepo, "proj", 5, 5, domain.Related, 1.0, "x"); err == nil {
		t.Fatal("expected error comparing a memory with itself")
	}
}
