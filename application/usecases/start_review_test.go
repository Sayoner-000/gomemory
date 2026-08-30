package usecases

import (
	"testing"

	"mem/domain"
)

func TestStartReviewRejectsEmptyDigestWithoutPersisting(t *testing.T) {
	repo := newMemoryReviewRepository()
	_, err := StartReview(repo, StartReviewInput{Project: "proj", TargetType: domain.TargetDiff})
	if err == nil {
		t.Fatal("StartReview debe rechazar el digest vacío")
	}
	if len(repo.reviews) != 0 {
		t.Fatal("StartReview persistió una revisión inválida")
	}
}

func TestStartReviewFreezesTargetAndDegradesSameModel(t *testing.T) {
	repo := newMemoryReviewRepository()
	review, err := StartReview(repo, StartReviewInput{
		Project:    "proj",
		TargetType: domain.TargetDiff,
		Revision:   "working-tree",
		Digest:     "sha256:frozen",
		Scope:      []string{"domain/"},
		ReviewerA:  ReviewerIdentity{Provider: "openai", Model: "gpt-x"},
		ReviewerB:  ReviewerIdentity{Provider: "openai", Model: "gpt-x"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if review.Target.Digest() != "sha256:frozen" {
		t.Fatalf("target no congelado: %q", review.Target.Digest())
	}
	if review.IndependenceLevel != domain.IndependenceDegraded || review.IndependenceReason != "same-model" {
		t.Fatalf("independencia = %s/%s", review.IndependenceLevel, review.IndependenceReason)
	}
	stored, _ := repo.GetReview("proj", review.ID)
	if stored == nil || stored.Status != domain.ReviewAwaitingReviewers {
		t.Fatalf("revisión no persistida en awaiting_reviewers: %#v", stored)
	}
}

// TestStartReview_AplicaLaPoliticaDelProyecto cubre FR-017. Settings ya tenía
// ReviewMaxFixRounds y ReviewAutoFixSeverities desde la 027, pero nadie los leía:
// start_review.go reimplantaba maxRounds=2 y {CRITICAL,HIGH} a mano, así que
// configurar el proyecto no tenía ningún efecto observable.
func TestStartReview_AplicaLaPoliticaDelProyecto(t *testing.T) {
	repo := newMemoryReviewRepository()
	proyecto := domain.ReviewPolicy{
		FixAuthorized:     true,
		MaxFixRounds:      5,
		AutoFixSeverities: []domain.Severity{domain.SeverityCritical},
	}
	review, err := StartReview(repo, StartReviewInput{
		Project: "proj", TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:frozen", Policy: proyecto,
	})
	if err != nil {
		t.Fatal(err)
	}
	if review.MaxFixRounds != 5 {
		t.Errorf("MaxFixRounds = %d, se esperaba el 5 del proyecto", review.MaxFixRounds)
	}
	if len(review.AutoFixSeverities) != 1 || review.AutoFixSeverities[0] != domain.SeverityCritical {
		t.Errorf("AutoFixSeverities = %v, se esperaba la del proyecto", review.AutoFixSeverities)
	}
	if !review.FixAuthorized {
		t.Error("la política del proyecto autorizaba corregir")
	}

	// Los valores explícitos de la revisión ganan a la política del proyecto.
	explicita, err := StartReview(repo, StartReviewInput{
		Project: "proj", TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:frozen", Policy: proyecto, MaxFixRounds: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if explicita.MaxFixRounds != 1 {
		t.Errorf("el valor explícito de la revisión debe ganar: %d", explicita.MaxFixRounds)
	}

	// Sin política ni valores explícitos, el defecto sale del dominio.
	pordefecto, err := StartReview(repo, StartReviewInput{
		Project: "proj", TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:frozen",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pordefecto.MaxFixRounds != domain.DefaultMaxFixRounds {
		t.Errorf("MaxFixRounds = %d, se esperaba el defecto del dominio %d",
			pordefecto.MaxFixRounds, domain.DefaultMaxFixRounds)
	}
	if !pordefecto.FixAuthorized {
		t.Error("sin declarar nada, una revisión debe seguir autorizando corregir")
	}
}

// TestStartReview_SoloLecturaSeDeclaraExplicitamente cubre FR-018.
func TestStartReview_SoloLecturaSeDeclaraExplicitamente(t *testing.T) {
	repo := newMemoryReviewRepository()
	no := false
	review, err := StartReview(repo, StartReviewInput{
		Project: "proj", TargetType: domain.TargetDiff, Revision: "working-tree",
		Digest: "sha256:frozen", FixAuthorized: &no,
	})
	if err != nil {
		t.Fatal(err)
	}
	if review.FixAuthorized {
		t.Fatal("--read-only debe producir una revisión que no corrige")
	}
	if review.CurrentTargetDigest != "sha256:frozen" {
		t.Errorf("el target vigente debe arrancar igual que el original: %q", review.CurrentTargetDigest)
	}
}
