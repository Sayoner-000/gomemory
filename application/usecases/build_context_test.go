package usecases_test

import (
	"strings"
	"testing"

	"mem/adapters/secondary/persistence"
	"mem/application/usecases"
	"mem/domain"
)

func TestBuild_SurfacesUnresolvedConflicts(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	idA, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Redis para cache", Content: "..."})
	if err != nil {
		t.Fatalf("insert memory a: %v", err)
	}
	idB, err := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "usa Memcached para cache", Content: "..."})
	if err != nil {
		t.Fatalf("insert memory b: %v", err)
	}

	if _, _, err := usecases.RecordVerdict(relRepo, "proj", idA, idB, domain.ConflictsWith, 0.9, "se contradicen"); err != nil {
		t.Fatalf("record verdict: %v", err)
	}

	builder := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if !strings.Contains(out, "Conflictos sin resolver") {
		t.Fatalf("expected conflicts section in context, got:\n%s", out)
	}
	if !strings.Contains(out, "usa Redis para cache") || !strings.Contains(out, "usa Memcached para cache") {
		t.Fatalf("expected both conflicting titles in context, got:\n%s", out)
	}
}

func TestBuild_NoConflictsSectionWhenResolved(t *testing.T) {
	root := t.TempDir()
	db, err := persistence.Init(root)
	if err != nil {
		t.Fatalf("init db: %v", err)
	}
	defer db.Close()

	memRepo := persistence.NewMemoryRepository(db)
	sessRepo := persistence.NewSessionRepository(db)
	relRepo := persistence.NewRelationRepository(db)

	idA, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "A", Content: "..."})
	idB, _ := memRepo.Insert(&domain.Memory{Project: "proj", Type: domain.Decision, Title: "B", Content: "..."})

	if _, _, err := usecases.RecordVerdict(relRepo, "proj", idA, idB, domain.NotConflict, 1.0, "verifiqué, no hay conflicto real"); err != nil {
		t.Fatalf("record verdict: %v", err)
	}

	builder := usecases.New(memRepo, sessRepo, relRepo, root, "proj")
	out, err := builder.Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if strings.Contains(out, "Conflictos sin resolver") {
		t.Fatalf("did not expect conflicts section once resolved, got:\n%s", out)
	}
}
